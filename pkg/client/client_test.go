package client

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateApplicationsUrl(t *testing.T) {
	tests := []struct {
		name       string
		domain     string
		insecure   bool
		labels     map[string]string
		expected   string
		multiLabel bool
	}{
		{
			name:       "local server",
			domain:     "test.domain",
			insecure:   true,
			labels:     map[string]string{},
			expected:   "http://test.domain/api/applications",
			multiLabel: false,
		},
		{
			name:       "https",
			domain:     "test.domain",
			insecure:   false,
			labels:     map[string]string{},
			expected:   "https://test.domain/api/applications",
			multiLabel: false,
		},
		{
			name:     "one label",
			domain:   "test.domain",
			insecure: false,
			labels: map[string]string{
				"label1": "value1",
			},
			expected:   "https://test.domain/api/applications?labels=label1:value1",
			multiLabel: false,
		},
		{
			name:     "multiple labels",
			domain:   "test.domain",
			insecure: false,
			labels: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
			expected:   "",
			multiLabel: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := GenerateApplicationsUrl(test.domain, test.insecure, test.labels)
			if !test.multiLabel {
				assert.Equal(t, test.expected, actual)
			} else {
				assert.True(t,
					strings.Contains(actual, "?labels=label1:value1,label2:value2") || strings.Contains(actual, "?labels=label2:value2,label1:value1"),
				)
			}

		})
	}
}

func TestGenerateApplicationsUrlWithOptions(t *testing.T) {
	tests := []struct {
		name       string
		domain     string
		insecure   bool
		options    *ApplicationsUrlOptions
		expected   string
		multiLabel bool
	}{
		{
			name:     "local server",
			domain:   "test.domain",
			insecure: true,
			options: &ApplicationsUrlOptions{
				Labels: map[string]string{},
			},
			expected:   "http://test.domain/api/applications",
			multiLabel: false,
		},
		{
			name:     "https",
			domain:   "test.domain",
			insecure: false,
			options: &ApplicationsUrlOptions{
				Labels: map[string]string{},
			},
			expected:   "https://test.domain/api/applications",
			multiLabel: false,
		},
		{
			name:     "one label",
			domain:   "test.domain",
			insecure: false,
			options: &ApplicationsUrlOptions{
				Labels: map[string]string{
					"label1": "value1",
				},
			},
			expected:   "https://test.domain/api/applications?labels=label1:value1",
			multiLabel: false,
		},
		{
			name:     "one exclude label",
			domain:   "test.domain",
			insecure: false,
			options: &ApplicationsUrlOptions{
				ExcludeLabels: map[string]string{
					"label1": "value1",
				},
			},
			expected:   "https://test.domain/api/applications?excludeLabels=label1:value1",
			multiLabel: false,
		},
		{
			name:     "one include and one exclude label",
			domain:   "test.domain",
			insecure: false,
			options: &ApplicationsUrlOptions{
				Labels: map[string]string{
					"label1": "value1",
				},
				ExcludeLabels: map[string]string{
					"label2": "value2",
				},
			},
			expected:   "https://test.domain/api/applications?labels=label1:value1&excludeLabels=label2:value2",
			multiLabel: false,
		},
		{
			name:     "multiple labels",
			domain:   "test.domain",
			insecure: false,
			options: &ApplicationsUrlOptions{
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
			},
			expected:   "",
			multiLabel: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := GenerateApplicationsUrlWithOptions(test.domain, test.insecure, test.options)
			if !test.multiLabel {
				assert.Equal(t, test.expected, actual)
			} else {
				assert.True(t,
					strings.Contains(actual, "?labels=label1:value1,label2:value2") || strings.Contains(actual, "?labels=label2:value2,label1:value1"),
				)
			}

		})
	}
}

func TestGenerateDiffUrl(t *testing.T) {
	tests := []struct {
		name        string
		domain      string
		insecure    bool
		argocd      string
		application string
		expected    string
	}{
		{
			name:        "local server",
			domain:      "test.domain",
			insecure:    true,
			argocd:      "test",
			application: "test-1",
			expected:    "http://test.domain/api/argocd/test/applications/test-1/diffs",
		},
		{
			name:        "actual domain",
			domain:      "test.domain",
			insecure:    false,
			argocd:      "test",
			application: "test-1",
			expected:    "https://test.domain/api/argocd/test/applications/test-1/diffs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := GenerateDiffUrl(test.domain, test.insecure, test.argocd, test.application)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestGetApplications(t *testing.T) {
	tests := []struct {
		name        string
		domain      string
		insecure    bool
		labels      map[string]string
		lengthTest  int
		lengthProd  int
		expectError bool
	}{
		{
			name:        "get all applications",
			domain:      "localhost:8081",
			insecure:    true,
			labels:      map[string]string{},
			lengthTest:  2,
			lengthProd:  2,
			expectError: false,
		},
		{
			name:     "get test applications",
			domain:   "localhost:8081",
			insecure: true,
			labels: map[string]string{
				"env": "test",
			},
			lengthTest:  1,
			lengthProd:  0,
			expectError: false,
		},
		{
			name:        "error",
			domain:      "1localhost:8081",
			insecure:    true,
			labels:      map[string]string{},
			lengthTest:  0,
			lengthProd:  0,
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := GetApplications(GenerateApplicationsUrl(test.domain, test.insecure, test.labels))
			if test.expectError {
				assert.Error(t, err)
			} else {
				for _, result := range resp.Results {
					if result.Name == "test" {
						assert.Len(t, result.Applications, test.lengthTest)
					} else if result.Name == "prod" {
						assert.Len(t, result.Applications, test.lengthProd)
					}
				}
			}
		})
	}
}

func TestGetApplicationsWithRetries(t *testing.T) {
	tests := []struct {
		name        string
		domain      string
		insecure    bool
		options     *ClientOptions
		expectError bool
	}{
		{
			name:        "get all applications",
			domain:      "localhost:8081",
			insecure:    true,
			options:     nil,
			expectError: false,
		},
		{
			name:     "with retries",
			domain:   "localhost:8081",
			insecure: true,
			options: &ClientOptions{
				Retries: 3,
			},
			expectError: false,
		},
		{
			name:     "with custom period",
			domain:   "localhost:8081",
			insecure: true,
			options: &ClientOptions{
				Retries: 3,
				Backoff: []int{1, 2, 3},
			},
			expectError: false,
		},
		{
			name:     "with invalid retries",
			domain:   "localhost:8081",
			insecure: true,
			options: &ClientOptions{
				Retries: 6,
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			emptyLabels := map[string]string{}
			resp, err := GetApplicationWithRetries(GenerateApplicationsUrl(test.domain, test.insecure, emptyLabels), test.options)

			if !test.expectError {
				assert.Nil(t, err)
				for _, result := range resp.Results {
					if result.Name == "test" {
						assert.Len(t, result.Applications, 2)
					} else if result.Name == "prod" {
						assert.Len(t, result.Applications, 2)
					}
				}
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestGetDiffs(t *testing.T) {
	tests := []struct {
		name        string
		domain      string
		insecure    bool
		argocd      string
		application string
		liveRef     string
		targetRef   string
		argocdError bool
		expectError bool
	}{
		{
			name:        "get diff",
			domain:      "localhost:8081",
			insecure:    true,
			argocd:      "test",
			application: "test-1",
			liveRef:     "main",
			targetRef:   "test_gitops",
			argocdError: false,
			expectError: false,
		},
		{
			name:        "get diff with outofsync app",
			domain:      "localhost:8081",
			insecure:    true,
			argocd:      "test",
			application: "test-2",
			liveRef:     "main",
			targetRef:   "test_gitops",
			argocdError: false,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := GetDiffs(GenerateDiffUrl(test.domain, test.insecure, test.argocd, test.application), test.liveRef, test.targetRef)
			if test.expectError {
				assert.Error(t, err)
			} else if test.argocdError {
				assert.NotNil(t, actual.ManifestGenerationError)
			} else {
				assert.NotNil(t, actual.LiveManifests)
				assert.NotNil(t, actual.Diffs)
			}
		})
	}
}

func TestGetDiffsWithRetries(t *testing.T) {
	tests := []struct {
		name        string
		domain      string
		insecure    bool
		argocd      string
		application string
		liveRef     string
		targetRef   string
		options     *ClientOptions
		argocdError bool
		expectError bool
	}{
		{
			name:        "no options",
			domain:      "localhost:8081",
			insecure:    true,
			argocd:      "test",
			application: "test-1",
			liveRef:     "main",
			targetRef:   "test_gitops",
			options:     nil,
			argocdError: false,
			expectError: false,
		},
		{
			name:        "retries",
			domain:      "localhost:8081",
			insecure:    true,
			argocd:      "test",
			application: "test-1",
			liveRef:     "main",
			targetRef:   "test_gitops",
			options:     &ClientOptions{Retries: 3},
			argocdError: false,
			expectError: false,
		},
		{
			name:        "retries and custom retries",
			domain:      "localhost:8081",
			insecure:    true,
			argocd:      "test",
			application: "test-1",
			liveRef:     "main",
			targetRef:   "test_gitops",
			options:     &ClientOptions{Retries: 3, Backoff: []int{1, 2, 3}},
			argocdError: false,
			expectError: false,
		},
		{
			name:        "invalid_config",
			domain:      "localhost:8081",
			insecure:    true,
			argocd:      "test",
			application: "test-1",
			liveRef:     "main",
			targetRef:   "test_gitops",
			options:     &ClientOptions{Retries: 6},
			argocdError: false,
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := GetDiffsWithRetries(GenerateDiffUrl(test.domain, test.insecure, test.argocd, test.application), test.liveRef, test.targetRef, test.options)
			if test.expectError {
				assert.Error(t, err)
			} else if test.argocdError {
				assert.NotNil(t, actual.ManifestGenerationError)
			} else {
				assert.NotNil(t, actual.LiveManifests)
				assert.NotNil(t, actual.Diffs)
			}
		})
	}
}

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name        string
		options     *ClientOptions
		expectError bool
	}{
		{
			name: "just retries",
			options: &ClientOptions{
				Retries: 3,
			},
			expectError: false,
		},
		{
			name: "all options",
			options: &ClientOptions{
				Retries: 3,
				Backoff: []int{1, 2, 3},
			},
			expectError: false,
		},
		{
			name: "retry greater than default backoff periods",
			options: &ClientOptions{
				Retries: 6,
			},
			expectError: true,
		},
		{
			name: "retry greater than custom backoff",
			options: &ClientOptions{
				Retries: 4,
				Backoff: []int{1, 2, 3},
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		if !test.expectError {
			assert.Nil(t, validateClientOptions(*test.options))
		} else {
			assert.Error(t, validateClientOptions(*test.options))
		}
	}
}
