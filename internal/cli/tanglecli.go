package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/ivanklee86/tangle/internal/tangle"
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

func (t *TangleCLI) Configure() {
	t.Labels = labelStringsToMap(t.LabelsAsStrings)
}

func (t *TangleCLI) GenerateManifests() {
	labels := []string{}
	for k, v := range t.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}

	applicationsUrl := fmt.Sprintf("http://%s/api/applications?labels=%s", t.ServerAddr, strings.Join(labels, ","))

	t.Output(fmt.Sprintf("Calling %s", applicationsUrl))
	resp, err := http.Get(applicationsUrl)
	if err != nil {
		t.Error(fmt.Sprintf("Error calling %s: %s", applicationsUrl, err))
		return
	}
	body, err := io.ReadAll(resp.Body)
	t.Output(fmt.Sprintf("Response body: %s", body))
	if err != nil {
		t.Error(fmt.Sprintf("Error reading response body: %s", err))
		return

	}
	defer resp.Body.Close()

	var applications tangle.ApplicationsResponse
	err = json.Unmarshal(body, &applications)
	if err != nil {
		t.Error(fmt.Sprintf("Error unmarshalling response body: %s", err))
		return
	}

	t.Output(fmt.Sprintf("Found %d applications", len(applications.Results)))
}
