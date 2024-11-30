// This file contains the Tangle configuration struct.
package tangle

type TangleConfig struct {
	Name                   string `koanf:"name"`
	Domain                 string `koanf:"domain"`
	Port                   int    `koanf:"port"`
	DoNotInstrumentWorkers bool
}
