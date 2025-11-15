package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alitto/pond/v2"
	"github.com/ivanklee86/tangle/internal/tangle"
	"github.com/ivanklee86/tangle/pkg/client"
	"github.com/jedib0t/go-pretty/v6/table"
)

type Config struct {
	ServerAddr             string
	Insecure               bool
	LabelsAsStrings        []string
	ExcludeLabelsAsStrings []string
	Labels                 map[string]string
	ExcludeLabels          map[string]string
	Folder                 string
	TargetRef              string
	FailOnErrors           bool
	Retries                int
}

type TangleCLI struct {
	*Config

	// Allow swapping out stdout/stderr for testing.
	Out io.Writer
	Err io.Writer
}

type ApplicationDiffDetail struct {
	ArgoCD      string
	Application string
	LiveRef     string
	Response    tangle.DiffsResponse
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

// Prints results in human readable format
func (t *TangleCLI) renderApplicationsTable(applications []*ApplicationDiffDetail) {
	applicationsTable := table.NewWriter()
	applicationsTable.SetOutputMirror(t.Out)
	applicationsTable.SetStyle(table.StyleColoredBright)
	applicationsTable.AppendHeader(table.Row{"ArgoCD", "Application", "Error"})
	for _, application := range applications {
		manifestGenError := "False ✅"
		if application.Response.ManifestGenerationError != "" {
			manifestGenError = "True 🔥"
		}

		applicationsTable.AppendRow(table.Row{application.ArgoCD, application.Application, manifestGenError})
	}
	applicationsTable.Render()
}

func (t *TangleCLI) WriteFiles(applicationDiffsDetail *ApplicationDiffDetail) error {
	diffFile, err := os.Create(fmt.Sprintf("%s/diff-%s-%s.yaml", t.Folder, applicationDiffsDetail.ArgoCD, applicationDiffsDetail.Application))
	if err != nil {
		return err
	}
	manifestsFile, err := os.Create(fmt.Sprintf("%s/manifests-%s-%s.yaml", t.Folder, applicationDiffsDetail.ArgoCD, applicationDiffsDetail.Application))
	if err != nil {
		return err
	}

	defer diffFile.Close()
	defer manifestsFile.Close()

	_, err = diffFile.WriteString(applicationDiffsDetail.Response.Diffs)
	if err != nil {
		return err
	}
	_, err = manifestsFile.WriteString(applicationDiffsDetail.Response.TargetManifests)
	if err != nil {
		return err
	}

	if applicationDiffsDetail.Response.ManifestGenerationError != "" {
		errorFile, err := os.Create(fmt.Sprintf("%s/error-%s-%s.txt", t.Folder, applicationDiffsDetail.ArgoCD, applicationDiffsDetail.Application))
		if err != nil {
			return err
		}
		defer errorFile.Close()
		_, err = errorFile.WriteString(applicationDiffsDetail.Response.ManifestGenerationError)
		if err != nil {
			return err
		}
	}

	return nil
}

// New returns a new instance of TangleCLI.
func New() *TangleCLI {
	config := Config{}
	config.Labels = labelStringsToMap(config.LabelsAsStrings)
	config.ExcludeLabels = labelStringsToMap(config.ExcludeLabelsAsStrings)

	if config.Folder == "" {
		dir, err := os.Getwd()
		if err != nil {
			panic(err)
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
	config.ExcludeLabels = labelStringsToMap(config.ExcludeLabelsAsStrings)

	if config.Folder == "" {
		dir, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		config.Folder = dir
	}

	return &TangleCLI{
		Config: &config,
		Out:    os.Stdout,
		Err:    os.Stdin,
	}
}

func (t *TangleCLI) Configure() {
	t.Labels = labelStringsToMap(t.LabelsAsStrings)
	t.ExcludeLabels = labelStringsToMap(t.ExcludeLabelsAsStrings)
}

func (t *TangleCLI) GenerateManifests() {
	applicationsUrl := client.GenerateApplicationsUrlWithOptions(t.ServerAddr, t.Insecure, &client.ApplicationsUrlOptions{
		Labels:        t.Labels,
		ExcludeLabels: t.ExcludeLabels,
	})
	t.OutputHeading(fmt.Sprintf("📱 Calling %s", applicationsUrl))

	var applications *tangle.ApplicationsResponse
	var err error
	if t.Config.Retries > 0 {
		options := client.ClientOptions{
			Retries: t.Config.Retries,
		}
		applications, err = client.GetApplicationWithRetries(applicationsUrl, &options)
		if err != nil {
			t.Error(fmt.Sprintf("Error getting applications: %s", err))
		}
	} else {
		applications, err = client.GetApplications(applicationsUrl)
		if err != nil {
			t.Error(fmt.Sprintf("Error getting applications: %s", err))
		}
	}

	// Parse out applications for all ArgoCDs.
	applicationDiffsDetails := []ApplicationDiffDetail{}
	for _, argocd := range applications.Results {
		for _, application := range argocd.Applications {
			applicationDiffsDetails = append(applicationDiffsDetails, ApplicationDiffDetail{
				ArgoCD:      argocd.Name,
				Application: application.Name,
				LiveRef:     application.LiveRef,
			})
		}
	}

	t.Output(fmt.Sprintf("Applications found: %d", len(applicationDiffsDetails)))

	// Get diffs for each application
	t.OutputHeading("🔍 Getting manifests and diffs!")
	pool := pond.NewResultPool[*ApplicationDiffDetail](len(applicationDiffsDetails))
	group := pool.NewGroup()
	for _, diff := range applicationDiffsDetails {
		group.SubmitErr(func() (*ApplicationDiffDetail, error) {
			diffUrl := client.GenerateDiffUrl(t.ServerAddr, t.Insecure, diff.ArgoCD, diff.Application)

			var response *tangle.DiffsResponse
			var err error
			if t.Config.Retries > 0 {
				options := client.ClientOptions{
					Retries: t.Config.Retries,
				}
				response, err = client.GetDiffsWithRetries(diffUrl, diff.LiveRef, t.TargetRef, &options)
				if err != nil {
					return nil, err
				}
			} else {
				response, err = client.GetDiffs(diffUrl, diff.LiveRef, t.TargetRef)
				if err != nil {
					return nil, err
				}
			}

			diff.Response = *response

			return &diff, nil
		})
	}
	results, err := group.Wait()
	if err != nil {
		t.Error(fmt.Sprintf("Error getting diffs: %s", err))
	}

	// Write files
	for _, result := range results {
		err := t.WriteFiles(result)
		if err != nil {
			t.Error(fmt.Sprintf("Error writing files: %s", err))
		}
	}

	// Report results
	t.renderApplicationsTable(results)

	// Fail if errors found
	failures := false
	for _, result := range results {
		if result.Response.ManifestGenerationError != "" {
			failures = true
		}
	}

	if t.FailOnErrors && failures {
		t.Error("Failures found in manifest generation!")
		os.Exit(1)
	}
}
