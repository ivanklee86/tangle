package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

// countingServer responds with a fixed status and body, and records how many
// requests it received. Counting requests is how the retry behavior below is
// asserted without any test actually sleeping.
func countingServer(t *testing.T, status int, body string) (*httptest.Server, *int32) {
	t.Helper()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server, &calls
}

// TestGetApplicationsSurfacesErrorBody covers the caller-facing half of #241:
// tangle-server now explains which label was wrong in the response body, and
// a client that throws that away leaves tangle-cli printing a bare status
// code for a mistake the user could have fixed.
func TestGetApplicationsSurfacesErrorBody(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		wantContains []string
	}{
		{
			name:         "400 with error body",
			status:       http.StatusBadRequest,
			body:         `{"error":"invalid label \"env\" in labels: expected key:value"}`,
			wantContains: []string{"400", `invalid label "env" in labels`},
		},
		{
			name:         "400 with non-JSON body",
			status:       http.StatusBadRequest,
			body:         "not json",
			wantContains: []string{"400"},
		},
		{
			name:         "500 with error body",
			status:       http.StatusInternalServerError,
			body:         `{"error":"simulated connection error"}`,
			wantContains: []string{"500", "simulated connection error"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, _ := countingServer(t, test.status, test.body)

			_, err := GetApplications(server.URL + "/api/applications")
			assert.Error(t, err)
			for _, want := range test.wantContains {
				assert.Contains(t, err.Error(), want)
			}

			var statusErr *StatusError
			assert.True(t, errors.As(err, &statusErr), "callers should be able to inspect the status")
			assert.Equal(t, test.status, statusErr.Code)
		})
	}
}

// TestGetApplicationWithRetriesSkips4xx pins the retry policy: a request the
// server has already rejected as malformed is not worth repeating, and with
// the default backoff a 400 would otherwise cost the caller over a minute
// before reporting a typo.
func TestGetApplicationWithRetriesSkips4xx(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantCalls int32
	}{
		{
			name:      "4xx is not retried",
			status:    http.StatusBadRequest,
			wantCalls: 1,
		},
		{
			name:      "404 is not retried",
			status:    http.StatusNotFound,
			wantCalls: 1,
		},
		{
			// Unchanged behavior: a 5xx may well be transient, so the
			// caller still gets every attempt they asked for.
			name:      "5xx is still retried",
			status:    http.StatusInternalServerError,
			wantCalls: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, calls := countingServer(t, test.status, `{"error":"boom"}`)

			// Zero backoff so the retried case doesn't make the suite wait.
			_, err := GetApplicationWithRetries(server.URL+"/api/applications", &ClientOptions{
				Retries: 2,
				Backoff: []int{0, 0},
			})

			assert.Error(t, err)
			assert.Equal(t, test.wantCalls, atomic.LoadInt32(calls))
		})
	}
}

// TestGetApplicationWithRetriesRecoversAfter5xx guards against the 4xx
// short-circuit accidentally breaking the case retries exist for.
func TestGetApplicationWithRetriesRecoversAfter5xx(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"transient"}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()

	applications, err := GetApplicationWithRetries(server.URL+"/api/applications", &ClientOptions{
		Retries: 2,
		Backoff: []int{0, 0},
	})

	assert.NoError(t, err)
	assert.NotNil(t, applications)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

// TestGetDiffsSurfacesErrorBody — the diffs endpoint has no 400 today, but it
// shares the status-handling path, so this pins it against drift.
func TestGetDiffsSurfacesErrorBody(t *testing.T) {
	server, _ := countingServer(t, http.StatusInternalServerError, `{"error":"manifest generation failed"}`)

	_, err := GetDiffs(server.URL+"/api/argocd/test/applications/test-1/diffs", "main", "feature")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manifest generation failed")
}
