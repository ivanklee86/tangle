package cli

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/ivanklee86/tangle/pkg/client"
	"github.com/jedib0t/go-pretty/v6/table"
)

type Config struct {
	ServerAddr      string
	Insecure        bool
	LabelsAsStrings []string
	Labels          map[string]string
	Folder          string
	TargetRef       string
}

type TangleCLI struct {
	*Config

	// Allow swapping out stdout/stderr for testing.
	Out io.Writer
	Err io.Writer
}

type ApplicationDiffDetails struct {
	ArgoCD      string
	Application string
	LiveRef     string
	TargetRef   string
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

func (t *TangleCLI) renderApplicationsTable(applications []ApplicationDiffDetails) {
	applicationsTable := table.NewWriter()
	applicationsTable.SetOutputMirror(t.Out)
	applicationsTable.SetStyle(table.StyleColoredBright)
	applicationsTable.AppendHeader(table.Row{"ArgoCD", "Application"})
	for _, application := range applications {
		applicationsTable.AppendRow(table.Row{application.ArgoCD, application.Application})
	}
	applicationsTable.Render()
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
	applicationsUrl := client.GenerateApplicationsUrl(t.ServerAddr, t.Insecure, t.Labels)
	t.OutputHeading(fmt.Sprintf("📱 Calling %s", applicationsUrl))

	applications, err := client.GetApplications(applicationsUrl)
	if err != nil {
		t.Error(fmt.Sprintf("Error getting applications: %s", err))
	}

	// Parse out applications for all ArgoCDs.
	applicationDiffsDetails := []ApplicationDiffDetails{}
	for _, argocd := range applications.Results {
		for _, application := range argocd.Applications {
			applicationDiffsDetails = append(applicationDiffsDetails, ApplicationDiffDetails{
				ArgoCD:      argocd.Name,
				Application: application.Name,
				LiveRef:     application.LiveRef,
				TargetRef:   t.TargetRef,
			})
		}
	}

	t.Output(fmt.Sprintf("Applications found: %d", len(applicationDiffsDetails)))
	t.renderApplicationsTable(applicationDiffsDetails)
}
