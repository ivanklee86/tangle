// This file contains the Tangle configuration struct.
package tangle

type TangleArgoCDConfig struct {
	Address         string `koanf:"address"`
	Insecure        bool   `koanf:"insecure"`
	AuthTokenEnvVar string `koanf:"authTokenEnvVar"`
}

type TangleConfig struct {
	Name    string                        `koanf:"name"`
	Domain  string                        `koanf:"domain"`
	Port    int                           `koanf:"port"`
	ArgoCDs map[string]TangleArgoCDConfig `koanf:"argocds"`

	// Internal configuration (for testing)
	DoNotInstrumentWorkers bool
}
