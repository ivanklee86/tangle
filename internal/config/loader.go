package config

import (
	"os"

	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

func Load() (*koanf.Koanf, err) {
	value, exists := os.LookupEnv("TANGLE_CONFIG_PATH")
	if !exists {
		return nil, error("configuration path not set")
	}

	f := file.Provider(value)

}
