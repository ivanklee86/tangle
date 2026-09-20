package client

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ivanklee86/tangle/internal/argocd"
	"github.com/ivanklee86/tangle/internal/argocd/argocdfakes"
	"github.com/ivanklee86/tangle/internal/tangle"
)

// newFakeTangleServer mirrors cmd/tangle-cli/main_test.go's helper of the
// same name — duplicated rather than shared because this file's package
// (client) and cmd/tangle-cli are different packages, and the helper is
// small enough that a shared test-support package isn't worth it. This
// package's tests previously hit a hardcoded "localhost:8081", relying on
// nothing actually listening there and only "passing" because of a stale Go
// test cache — see docs/agents/plans/go-major-dependency-migration.md's
// Outcome section for how that was found.
func newFakeTangleServer(t *testing.T) *httptest.Server {
	t.Helper()

	argocdConfig := make(map[string]tangle.TangleArgoCDConfig)
	argocdConfig["test"] = tangle.TangleArgoCDConfig{
		Address:         "localhost:8080",
		PlainText:       true,
		AuthTokenEnvVar: "ARGOCD_TOKEN",
	}
	argocdConfig["prod"] = tangle.TangleArgoCDConfig{
		Address:         "localhost:8080",
		PlainText:       true,
		AuthTokenEnvVar: "ARGOCD_PROD_TOKEN",
	}

	config := tangle.TangleConfig{
		Name:            "test-tangle",
		Domain:          "localhost",
		Port:            8081,
		ArgoCDs:         argocdConfig,
		DoNotInstrument: true,
	}

	realTangle := tangle.New(&config, "testing")

	testWrapper := argocdfakes.NewFakeWrapper([]argocdfakes.FakeApplication{
		{Name: "test-1", Project: "default", Namespace: "argocd", Labels: map[string]string{"env": "test", "foo": "bar", "bazz": "buzz"}, LiveRevision: "main"},
		{Name: "test-2", Project: "default", Namespace: "argocd", Labels: map[string]string{"env": "preprod", "foo": "bar", "bazz": "buzz"}, LiveRevision: "main"},
	})
	testWrapper.ManifestsByApp["test-1"] = &argocd.GetManifestsResponse{
		LiveManifests:   []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-1\n"},
		TargetManifests: []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-1\ndata:\n  updated: \"true\"\n"},
	}
	testWrapper.ManifestsByApp["test-2"] = &argocd.GetManifestsResponse{
		LiveManifests:   []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-2\n"},
		TargetManifests: []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-2\ndata:\n  updated: \"true\"\n"},
	}

	prodWrapper := argocdfakes.NewFakeWrapper([]argocdfakes.FakeApplication{
		{Name: "test-3", Project: "my-project", Namespace: "argocd", Labels: map[string]string{"env": "prod", "foo": "bar"}, LiveRevision: "main"},
		{Name: "test-4", Project: "my-project", Namespace: "argocd", Labels: map[string]string{"env": "infra", "foo": "bar"}, LiveRevision: "main"},
	})
	prodWrapper.ErrOnGetManifests["test-3"] = errors.New("rpc error: code = Unknown desc = failed to execute helm template command: broken values.yaml")
	prodWrapper.ManifestsByApp["test-4"] = &argocd.GetManifestsResponse{
		LiveManifests:   []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-4\n"},
		TargetManifests: []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-4\ndata:\n  updated: \"true\"\n"},
	}

	realTangle.ArgoCDs = map[string]argocd.IArgoCDWrapper{
		"test": testWrapper,
		"prod": prodWrapper,
	}

	server := httptest.NewServer(realTangle.Server.Handler)
	t.Cleanup(server.Close)

	return server
}

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
	server := newFakeTangleServer(t)
	fakeDomain := strings.TrimPrefix(server.URL, "http://")

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
			domain:      fakeDomain,
			insecure:    true,
			labels:      map[string]string{},
			lengthTest:  2,
			lengthProd:  2,
			expectError: false,
		},
		{
			name:     "get test applications",
			domain:   fakeDomain,
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
				assert.NoError(t, err)
				for _, result := range resp.Results {
					switch result.Name {
					case "test":
						assert.Len(t, result.Applications, test.lengthTest)
					case "prod":
						assert.Len(t, result.Applications, test.lengthProd)
					}
				}
			}
		})
	}
}

func TestGetApplicationsWithRetries(t *testing.T) {
	server := newFakeTangleServer(t)
	fakeDomain := strings.TrimPrefix(server.URL, "http://")

	tests := []struct {
		name        string
		domain      string
		insecure    bool
		options     *ClientOptions
		expectError bool
	}{
		{
			name:        "get all applications",
			domain:      fakeDomain,
			insecure:    true,
			options:     nil,
			expectError: false,
		},
		{
			name:     "with retries",
			domain:   fakeDomain,
			insecure: true,
			options: &ClientOptions{
				Retries: 3,
			},
			expectError: false,
		},
		{
			name:     "with custom period",
			domain:   fakeDomain,
			insecure: true,
			options: &ClientOptions{
				Retries: 3,
				Backoff: []int{1, 2, 3},
			},
			expectError: false,
		},
		{
			name:     "with invalid retries",
			domain:   fakeDomain,
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
					switch result.Name {
					case "test":
						assert.Len(t, result.Applications, 2)
					case "prod":
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
	server := newFakeTangleServer(t)
	fakeDomain := strings.TrimPrefix(server.URL, "http://")

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
			domain:      fakeDomain,
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
			domain:      fakeDomain,
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
	server := newFakeTangleServer(t)
	fakeDomain := strings.TrimPrefix(server.URL, "http://")

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
			domain:      fakeDomain,
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
			domain:      fakeDomain,
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
			domain:      fakeDomain,
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
			domain:      fakeDomain,
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
