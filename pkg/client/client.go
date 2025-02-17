package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ivanklee86/tangle/internal/tangle"
)

const (
	APPLICATIONS_PATH = "api/applications"
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
func GenerateDiffUrl(domain string, insecure bool, argocd string, application string) string {
	protocol := "http"
	if !insecure {
		protocol = "https"
	}
	url := fmt.Sprintf("%s://%s/api/argocd/%s/applications/%s/diffs", protocol, domain, argocd, application)

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

// GetDiffs retrieves the diffs for an ArgoCD Application.
func GetDiffs(url string, liveRef string, targetRef string) (*tangle.DiffsResponse, error) {
	// Build request
	requestBody := tangle.DiffsRequest{
		LiveRef:   liveRef,
		TargetRef: targetRef,
	}
	requestJson, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	diffs := &tangle.DiffsResponse{}
	err = json.NewDecoder(resp.Body).Decode(diffs)
	if err != nil {
		return nil, err
	}

	return diffs, nil
}
