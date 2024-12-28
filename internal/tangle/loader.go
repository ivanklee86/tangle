package tangle

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

const EnvVarPrefix string = "TANGLE_"

type LoadConfigOptions struct {
	Path string
}

func LoadConfig(config *koanf.Koanf, options LoadConfigOptions) (*TangleConfig, error) {
	// Look up config file
	configPath := ""
	if len(options.Path) == 0 {
		value, exists := os.LookupEnv(fmt.Sprintf("%sCONFIG_PATH", EnvVarPrefix))
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

	err = config.Load(env.Provider(EnvVarPrefix, ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, EnvVarPrefix)), "_", ".", -1)
	}), nil)
	if err != nil {
		return nil, err
	}

	// Unmarshall into config
	var tangleConfig TangleConfig
	err = config.Unmarshal("", &tangleConfig)
	if err != nil {
		return nil, err
	}

	return &tangleConfig, nil
}
