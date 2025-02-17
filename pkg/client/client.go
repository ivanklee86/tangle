package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ivanklee86/tangle/internal/tangle"
)

const (
	APPLICATIONS_PATH = "api/applications"
	DIFFS_PATH        = "api/diffs"
)

// GenerateApplicationsUrl generates the URL for the applications endpoint.
func GenerateApplicationsUrl(domain string, insecure bool, labels map[string]string) string {
	protocol := "http"
	if !insecure {
		protocol = "https"
	}

	url := fmt.Sprintf("%s://%s/%s", protocol, domain, APPLICATIONS_PATH)

	labelsAsStrings := []string{}
	if len(labels) > 0 {
		for k, v := range labels {
			labelsAsStrings = append(labelsAsStrings, fmt.Sprintf("%s:%s", k, v))
		}

		url += fmt.Sprintf("?labels=%s", strings.Join(labelsAsStrings, ","))
	}

	return url
}

// GenerateDiffUrl generates the URL for the diffs endpoint.
func GenerateDiffUrl(domain string, insecure bool, labels map[string]string, gitRef string) string {
	protocol := "http"
	if !insecure {
		protocol = "https"
	}

	url := fmt.Sprintf("%s://%s/%s", protocol, domain, DIFFS_PATH)

	labelsAsStrings := []string{}
	if len(labels) > 0 {
		for k, v := range labels {
			labelsAsStrings = append(labelsAsStrings, fmt.Sprintf("%s:%s", k, v))
		}

		url += fmt.Sprintf("?labels=%s", strings.Join(labelsAsStrings, ","))
	}

	if len(labels) > 0 {
		url += "&"
	} else {
		url += "?"
	}

	url += fmt.Sprintf("gitRef=%s", gitRef)

	return url
}

// GetApplications retrieves the applications from the given URL.
func GetApplications(url string) (*tangle.ApplicationsResponse, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	applications := &tangle.ApplicationsResponse{}
	err = json.NewDecoder(resp.Body).Decode(applications)
	if err != nil {
		return nil, err
	}

	return applications, nil
}
