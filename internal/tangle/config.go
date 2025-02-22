// This file contains the Tangle configuration struct.
package tangle

type TangleArgoCDConfig struct {
	Address         string `koanf:"address"`
	Insecure        bool   `koanf:"insecure"`
	AuthTokenEnvVar string `koanf:"authTokenEnvVar"`
}

type TangleConfig struct {
	Name      string                        `koanf:"name"`
	Env       string                        `koanf:"env"`
	Domain    string                        `koanf:"domain"`
	Port      int                           `koanf:"port"`
	Timeout   int                           `koanf:"timeout"`
	ArgoCDs   map[string]TangleArgoCDConfig `koanf:"argocds"`
	SortOrder []string                      `koanf:"sortOrder"`

	// Workers
	ListWorkers        int `koanf:"listWorkers"`
	ManifestsWorkers   int `koanf:"manifestsWorkers"`
	HardRefreshWorkers int `koanf:"hardRefreshWorkers"`

	// Internal configuration (for testing)
	DoNotInstrument bool
}

var TangleConfigDefaults = TangleConfig{
	Env:     "dev",
	Timeout: 60,

	ListWorkers:        10,
	ManifestsWorkers:   5,
	HardRefreshWorkers: 5,
}
