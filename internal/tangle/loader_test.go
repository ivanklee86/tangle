package tangle

import (
	"testing"

	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	t.Run("Loader", func(t *testing.T) {
		config := koanf.New(".")
		options := LoadConfigOptions{
			Path: "../../integration/tangle.yaml",
		}

		loadedConfig, err := LoadConfig(config, options)
		assert.Nil(t, err)
		assert.NotNil(t, loadedConfig)
		assert.Equal(t, "tangle", config.String("name"))
		assert.Equal(t, "tangle", loadedConfig.Name)
		assert.Len(t, loadedConfig.ArgoCDs, 2)
	})

	t.Run("Environment variable overrides file config", func(t *testing.T) {
		t.Setenv("TANGLE_NAME", "from-env")

		config := koanf.New(".")
		options := LoadConfigOptions{
			Path: "../../integration/tangle.yaml",
		}

		loadedConfig, err := LoadConfig(config, options)
		assert.Nil(t, err)
		assert.Equal(t, "from-env", loadedConfig.Name)
	})
}

// kubernetesServiceLinkEnv is what kubelet injects into every pod sharing a namespace with a
// Service named "tangle", unless the pod sets `enableServiceLinks: false`.
func kubernetesServiceLinkEnv(t *testing.T) {
	t.Helper()

	t.Setenv("TANGLE_SERVICE_HOST", "10.43.1.2")
	t.Setenv("TANGLE_SERVICE_PORT", "8080")
	t.Setenv("TANGLE_SERVICE_PORT_HTTP", "8080")
	t.Setenv("TANGLE_PORT", "tcp://10.43.1.2:8080")
	t.Setenv("TANGLE_PORT_8080_TCP", "tcp://10.43.1.2:8080")
	t.Setenv("TANGLE_PORT_8080_TCP_PROTO", "tcp")
	t.Setenv("TANGLE_PORT_8080_TCP_PORT", "8080")
	t.Setenv("TANGLE_PORT_8080_TCP_ADDR", "10.43.1.2")
}

func loadTestConfig(t *testing.T) (*koanf.Koanf, *TangleConfig, error) {
	t.Helper()

	config := koanf.New(".")
	loadedConfig, err := LoadConfig(config, LoadConfigOptions{Path: "../../integration/tangle.yaml"})

	return config, loadedConfig, err
}

func TestEnvironmentVariables(t *testing.T) {
	t.Run("Kubernetes service link variables don't break config loading", func(t *testing.T) {
		kubernetesServiceLinkEnv(t)

		config, loadedConfig, err := loadTestConfig(t)

		assert.Nil(t, err)
		// TANGLE_PORT_8080_TCP_ADDR and friends used to unflatten into a map at "port",
		// which failed to decode into an int and crash-looped the server at startup.
		assert.Equal(t, 8081, loadedConfig.Port)
		assert.Nil(t, config.Get("port.8080"))
		assert.Nil(t, config.Get("service"))
	})

	t.Run("Ignored environment variables are reported", func(t *testing.T) {
		kubernetesServiceLinkEnv(t)

		_, loadedConfig, err := loadTestConfig(t)

		assert.Nil(t, err)
		assert.Equal(t, []string{
			"TANGLE_PORT",
			"TANGLE_PORT_8080_TCP",
			"TANGLE_PORT_8080_TCP_ADDR",
			"TANGLE_PORT_8080_TCP_PORT",
			"TANGLE_PORT_8080_TCP_PROTO",
			"TANGLE_SERVICE_HOST",
			"TANGLE_SERVICE_PORT",
			"TANGLE_SERVICE_PORT_HTTP",
		}, loadedConfig.IgnoredEnvVars)
	})

	t.Run("A real port override is still honored", func(t *testing.T) {
		t.Setenv("TANGLE_PORT", "9090")

		_, loadedConfig, err := loadTestConfig(t)

		assert.Nil(t, err)
		assert.Equal(t, 9090, loadedConfig.Port)
		assert.Empty(t, loadedConfig.IgnoredEnvVars)
	})

	t.Run("An override replaces the file's value rather than landing beside it", func(t *testing.T) {
		t.Setenv("TANGLE_LISTWORKERS", "99")

		config, loadedConfig, err := loadTestConfig(t)

		assert.Nil(t, err)
		assert.Equal(t, 99, loadedConfig.ListWorkers)
		// The env key is canonicalized to the tag's spelling, so there's one key, not two.
		assert.Equal(t, "99", config.String("listWorkers"))
		assert.Equal(t, "", config.String("listworkers"))
	})

	t.Run("Overrides reach into the argocds map", func(t *testing.T) {
		t.Setenv("TANGLE_ARGOCDS_TEST_ADDRESS", "argocd.example.com:443")
		t.Setenv("TANGLE_ARGOCDS_TEST_PLAINTEXT", "false")

		_, loadedConfig, err := loadTestConfig(t)

		assert.Nil(t, err)
		assert.Len(t, loadedConfig.ArgoCDs, 2)
		assert.Equal(t, "argocd.example.com:443", loadedConfig.ArgoCDs["test"].Address)
		assert.False(t, loadedConfig.ArgoCDs["test"].PlainText)
		assert.Equal(t, "localhost:8080", loadedConfig.ArgoCDs["prod"].Address)
	})

	t.Run("Variables that name no config key are ignored", func(t *testing.T) {
		t.Setenv("TANGLE_NOT_A_SETTING", "value")
		// Naming a struct or map on its own sets nothing, and must not clobber it.
		t.Setenv("TANGLE_ARGOCDS", "value")

		config, loadedConfig, err := loadTestConfig(t)

		assert.Nil(t, err)
		assert.Len(t, loadedConfig.ArgoCDs, 2)
		assert.Nil(t, config.Get("not"))
		assert.Equal(t, []string{"TANGLE_ARGOCDS", "TANGLE_NOT_A_SETTING"}, loadedConfig.IgnoredEnvVars)
	})
}

func TestConfigPathEnvVar(t *testing.T) {
	t.Run("TANGLE_CONFIG_PATH is consumed, not reported as ignored", func(t *testing.T) {
		t.Setenv(ConfigPathEnvVar, "../../integration/tangle.yaml")

		config := koanf.New(".")
		loadedConfig, err := LoadConfig(config, LoadConfigOptions{})

		assert.Nil(t, err)
		assert.Equal(t, "tangle", loadedConfig.Name)
		assert.Empty(t, loadedConfig.IgnoredEnvVars)
	})
}

func TestConfigFileKeys(t *testing.T) {
	// Every key documented in docs/configuration.md has to match its koanf tag, or setting it
	// does nothing at all. `manifestWorkers` was documented — and set in this fixture — for a
	// tag spelled `manifestsWorkers`, so GetManifests parallelism silently stayed at its default.
	t.Run("Tuning keys in the config file reach the struct", func(t *testing.T) {
		_, loadedConfig, err := loadTestConfig(t)

		assert.Nil(t, err)
		assert.Equal(t, 8081, loadedConfig.Port)
		assert.Equal(t, 20, loadedConfig.ListWorkers)
		assert.Equal(t, 10, loadedConfig.ManifestsWorkers)
		assert.Equal(t, 10, loadedConfig.HardRefreshWorkers)
		// Not in the fixture, so it falls back to the struct default.
		assert.Equal(t, TangleConfigDefaults.Timeout, loadedConfig.Timeout)
	})
}
