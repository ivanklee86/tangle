package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ivanklee86/tangle/internal/tangle"
)

const (
	APPLICATIONS_PATH = "api/applications"
)

var DEFAULT_BACKOFF = []int{1, 5, 10, 20, 30}

type ClientOptions struct {
	Retries int
	Backoff []int
}

type ApplicationsUrlOptions struct {
	Domain        string
	Insecure      bool
	Labels        map[string]string
	ExcludeLabels map[string]string
}

// Validate option
func validateClientOptions(options ClientOptions) error {
	var backoffLen = len(DEFAULT_BACKOFF)
	if len(options.Backoff) > 0 {
		backoffLen = len(options.Backoff)
	}

	if options.Retries > backoffLen {
		return fmt.Errorf("retries cannot be greater than # of backoff periods (%d)", backoffLen)
	}

	return nil
}

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

func GenerateApplicationsUrlWithOptions(domain string, insecure bool, options *ApplicationsUrlOptions) string {
	protocol := "http"
	if !insecure {
		protocol = "https"
	}

	url := fmt.Sprintf("%s://%s/%s", protocol, domain, APPLICATIONS_PATH)

	labelsAsStrings := []string{}
	if options != nil && len(options.Labels) > 0 {
		for k, v := range options.Labels {
			labelsAsStrings = append(labelsAsStrings, fmt.Sprintf("%s:%s", k, v))
		}

		url += fmt.Sprintf("?labels=%s", strings.Join(labelsAsStrings, ","))
	}

	excludeLabelsAsStrings := []string{}
	if options != nil && len(options.ExcludeLabels) > 0 {
		for k, v := range options.ExcludeLabels {
			excludeLabelsAsStrings = append(excludeLabelsAsStrings, fmt.Sprintf("%s:%s", k, v))
		}

		if strings.Contains(url, "?") {
			url += "&"
		} else {
			url += "?"
		}
		url += fmt.Sprintf("excludeLabels=%s", strings.Join(excludeLabelsAsStrings, ","))
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

func GetApplicationWithRetries(url string, options *ClientOptions) (*tangle.ApplicationsResponse, error) {
	var retries = 0
	var backoff = DEFAULT_BACKOFF
	if options != nil {
		err := validateClientOptions(*options)
		if err != nil {
			return nil, err
		}
		retries = options.Retries
		if len(options.Backoff) > 0 {
			backoff = options.Backoff
		}
	}

	for i := 0; i <= retries; i++ {
		applications, err := GetApplications(url)
		if err == nil {
			return applications, nil
		} else if err != nil && i == retries {
			return nil, err
		} else if err != nil {
			time.Sleep(time.Duration(backoff[i]) * time.Second)
		}
	}

	return nil, nil
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

func GetDiffsWithRetries(url string, liveRef string, targetRef string, options *ClientOptions) (*tangle.DiffsResponse, error) {
	var retries = 0
	var backoff = DEFAULT_BACKOFF
	if options != nil {
		err := validateClientOptions(*options)
		if err != nil {
			return nil, err
		}
		retries = options.Retries
		if len(options.Backoff) > 0 {
			backoff = options.Backoff
		}
	}

	for i := 0; i <= retries; i++ {
		diffs, err := GetDiffs(url, liveRef, targetRef)
		if err == nil {
			return diffs, nil
		} else if err != nil && i == retries {
			return nil, err
		} else if err != nil {
			time.Sleep(time.Duration(backoff[i]) * time.Second)
		}
	}

	return nil, nil
}
