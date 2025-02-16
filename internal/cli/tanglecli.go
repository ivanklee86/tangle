package cli

import (
	"io"
	"log"
	"os"
	"strings"
)

type Config struct {
	ServerAddr      string
	Insecure        bool
	LabelsAsStrings []string
	Labels          map[string]string
	Folder          string
}

type TangleCLI struct {
	*Config

	// Allow swapping out stdout/stderr for testing.
	Out io.Writer
	Err io.Writer
}

func countCharacterOccurrences(s string, c rune) int {
	count := 0
	for _, char := range s {
		if char == c {
			count++
		}
	}
	return count
}

func labelStringsToMap(labelsAsStrings []string) map[string]string {
	labels := make(map[string]string)

	for _, labelString := range labelsAsStrings {
		if countCharacterOccurrences(labelString, '=') == 1 {
			kv := strings.Split(labelString, "=")

			if len(kv) == 2 {
				labels[kv[0]] = kv[1]
			}
		}
	}

	return labels
}

// New returns a new instance of TangleCLI.
func New() *TangleCLI {
	config := Config{}
	config.Labels = labelStringsToMap(config.LabelsAsStrings)

	if config.Folder == "" {
		dir, err := os.Getwd()
		if err != nil {
			log.Fatal(err) // Handle the error appropriately
		}
		config.Folder = dir
	}

	return &TangleCLI{
		Config: &config,
		Out:    os.Stdout,
		Err:    os.Stdin,
	}
}

func NewWithConfig(config Config) *TangleCLI {
	config.Labels = labelStringsToMap(config.LabelsAsStrings)

	return &TangleCLI{
		Config: &config,
		Out:    os.Stdout,
		Err:    os.Stdin,
	}
}

// func (t *TangleCLI) writeManifests() {

// }
