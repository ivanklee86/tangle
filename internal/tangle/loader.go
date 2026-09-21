package tangle

import (
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

const EnvVarPrefix string = "TANGLE_"

// ConfigPathEnvVar names the YAML config file. LoadConfig reads it directly, so it is deliberately
// not a key in TangleConfig.
const ConfigPathEnvVar string = EnvVarPrefix + "CONFIG_PATH"

type LoadConfigOptions struct {
	Path string
}

func LoadConfig(config *koanf.Koanf, options LoadConfigOptions) (*TangleConfig, error) {
	// Look up config file
	configPath := ""
	if len(options.Path) == 0 {
		value, exists := os.LookupEnv(ConfigPathEnvVar)
		if !exists {
			return nil, errors.New("configuration path not set")
		}
		configPath = value
	} else {
		configPath = options.Path
	}

	// Sensible defaults
	err := config.Load(structs.Provider(TangleConfigDefaults, "koanf"), nil)
	if err != nil {
		return nil, err
	}

	// Load configuration from environment then apply env overrides
	file := file.Provider(configPath)
	err = config.Load(file, yaml.Parser())
	if err != nil {
		return nil, err
	}

	// Kubernetes injects <SERVICE>_SERVICE_HOST, <SERVICE>_PORT and
	// <SERVICE>_PORT_<port>_<proto>_ADDR (and friends) into every pod sharing a namespace with a
	// Service, so a Service named "tangle" collides head-on with this prefix:
	// TANGLE_PORT_8080_TCP_ADDR would otherwise unflatten into a map at "port", where an int is
	// expected. Ignore any TANGLE_ variable that doesn't name a real config key, plus any whose
	// value has the injected Docker-link shape — TANGLE_PORT does name a real key.
	ignoredEnvVars := []string{}
	err = config.Load(env.Provider(".", env.Opt{
		Prefix: EnvVarPrefix,
		TransformFunc: func(k, v string) (string, any) {
			key := strings.ReplaceAll(strings.ToLower(
				strings.TrimPrefix(k, EnvVarPrefix)), "_", ".")

			if k == ConfigPathEnvVar {
				return "", nil
			}

			canonical, exists := resolveConfigKey(key)
			if !exists || kubernetesServiceLinkValue.MatchString(v) {
				ignoredEnvVars = append(ignoredEnvVars, k)
				return "", nil
			}

			return canonical, v
		},
	}), nil)
	if err != nil {
		return nil, err
	}
	slices.Sort(ignoredEnvVars)

	// Unmarshall into config
	var tangleConfig TangleConfig
	err = config.Unmarshal("", &tangleConfig)
	if err != nil {
		return nil, err
	}

	tangleConfig.IgnoredEnvVars = ignoredEnvVars

	return &tangleConfig, nil
}
