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
	})
}
