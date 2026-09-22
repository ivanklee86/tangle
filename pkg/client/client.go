package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// StatusError is a non-200 response from tangle-server. Message carries the
// server's own ErrorResponse body when it sent one — for a 400 that's the
// label the caller got wrong, which is the whole point of the status.
type StatusError struct {
	Code    int
	Message string
}

func (e *StatusError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("unexpected status code: %d", e.Code)
	}

	return fmt.Sprintf("unexpected status code: %d: %s", e.Code, e.Message)
}

// Retryable reports whether repeating the request could plausibly succeed. A
// 4xx says the request itself is wrong — retrying a malformed label filter
// just spends the backoff periods to get the same answer.
func (e *StatusError) Retryable() bool {
	return e.Code < 400 || e.Code >= 500
}

// statusError builds a StatusError from a non-200 response, pulling the
// server's ErrorResponse body out when it's there and falling back to the
// bare status when it isn't.
func statusError(code int, body []byte) *StatusError {
	errorResponse := tangle.ErrorResponse{}
	if err := json.Unmarshal(body, &errorResponse); err != nil {
		return &StatusError{Code: code}
	}

	return &StatusError{Code: code, Message: errorResponse.Error}
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
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp.StatusCode, body)
	}

	applications := &tangle.ApplicationsResponse{}
	err = json.Unmarshal(body, applications)
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
		}

		// A 4xx means the request is wrong, not that the server was
		// momentarily unhappy — retrying a malformed label filter spends
		// every backoff period to arrive at the same answer, and hides the
		// server's explanation behind the delay.
		var statusErr *StatusError
		if errors.As(err, &statusErr) && !statusErr.Retryable() {
			return nil, err
		}

		if i == retries {
			return nil, err
		}

		time.Sleep(time.Duration(backoff[i]) * time.Second)
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
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp.StatusCode, body)
	}

	// Parse response
	diffs := &tangle.DiffsResponse{}
	err = json.Unmarshal(body, diffs)
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
