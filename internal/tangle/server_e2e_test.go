//go:build e2e

package tangle

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
)

// newE2ETangle builds a real *Tangle from integration/tangle.yaml against
// the live ArgoCD `task services:cicd` brings up — the same config-loading
// path loader_test.go exercises, and the same "test"/"prod" ArgoCDs this
// package's mock-backed handler tests (handlers_test.go) used to build by
// hand before those moved to argocdfakes.
func newE2ETangle(t *testing.T) *Tangle {
	t.Helper()

	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatal(err)
	}

	config := koanf.New(".")
	loadedConfig, err := LoadConfig(config, LoadConfigOptions{Path: "../../integration/tangle.yaml"})
	if err != nil {
		t.Fatal(err)
	}

	// integration/tangle.yaml doesn't set doNotInstrument (real deployments
	// want the metrics) — but New() registers each ArgoCD's worker-pool
	// metrics on Prometheus's global registry, and constructing a fresh
	// Tangle per subtest (this file's own tests do exactly that) would
	// panic on the second registration of the same metric names. Disable
	// instrumentation for these short-lived test instances, same as
	// handlers_test.go's hand-rolled fake-backed config already does.
	loadedConfig.DoNotInstrument = true

	return New(loadedConfig, "testing")
}

// TestE2E_ServerApplications exercises applicationsHandler against a live
// ArgoCD seeded from integration/kubernetes/example/ — all four
// applications, split "default"/"my-project" across the "test"/"prod"
// ArgoCDs configured in integration/tangle.yaml, with the same
// label/exclude-label matrix TestHandlers asserts against fakes.
//
// Every top-level test in this file is prefixed TestE2E_ (matching
// internal/argocd/client_e2e_test.go's TestE2E_*) so `-run '^TestE2E_'`
// (tasks/go.yaml's test:e2e/test:e2e:ci) can select just these.
func TestE2E_ServerApplications(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		test_count int
		prod_count int
	}{
		{name: "no_tags", url: "/applications", test_count: 2, prod_count: 2},
		{name: "tags_match_all", url: "/applications?labels=foo:bar", test_count: 2, prod_count: 2},
		{name: "tags_match_one", url: "/applications?labels=env:test", test_count: 1, prod_count: 0},
		{name: "tags_match_none", url: "/applications?labels=env:foobar", test_count: 0, prod_count: 0},
		{name: "invalid_tags", url: "/applications?labels=foobar", test_count: 2, prod_count: 2},
		{name: "multiple_tags", url: "/applications?labels=env:test,bazz:buzz", test_count: 1, prod_count: 0},
		{name: "exclude", url: "/applications?labels=foo:bar&excludeLabels=env:test", test_count: 1, prod_count: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tangle := newE2ETangle(t)

			req, _ := http.NewRequest("GET", test.url, nil)
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(tangle.applicationsHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var result ApplicationsResponse
			err := json.NewDecoder(rr.Body).Decode(&result)
			assert.NoError(t, err)
			for _, argoCDResult := range result.Results {
				switch argoCDResult.Name {
				case "test":
					assert.Equal(t, test.test_count, len(argoCDResult.Applications))
				case "prod":
					assert.Equal(t, test.prod_count, len(argoCDResult.Applications))
				}
			}
		})
	}
}

// TestServerE2EDiffs generates a real diff for one application per ArgoCD
// against the real cluster's "main"/"test_gitops" branches. test-4 (not
// test-3) is deliberately the "prod" case here: test-3's target-branch helm
// chart has a genuinely broken values.yaml (confirmed live — its diff
// response comes back 200 with only manifestGenerationError set, via a
// server-side error path in applicationManifestsHandler that has no `return`
// after the error branch and works only because chi's Recoverer middleware
// absorbs the resulting nil-pointer panic after the error JSON already
// flushed — see the CLI tests, which exercise that exact path through the
// full router). A new test shouldn't freshly depend on that accident when a
// clean example (test-4) makes the same point about real diff generation.
func TestE2E_ServerDiffs(t *testing.T) {
	tests := []struct {
		name   string
		argocd string
		app    string
	}{
		{name: "test_argocd", argocd: "test", app: "test-1"},
		{name: "prod_argocd", argocd: "prod", app: "test-4"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tangle := newE2ETangle(t)

			requestBody, _ := json.Marshal(DiffsRequest{LiveRef: "main", TargetRef: "test_gitops"})
			req, _ := http.NewRequest("POST", "/api/argocd/"+test.argocd+"/applications/"+test.app+"/diffs", bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("argocd", test.argocd)
			rctx.URLParams.Add("name", test.app)
			ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(tangle.applicationManifestsHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var result DiffsResponse
			err := json.NewDecoder(rr.Body).Decode(&result)
			assert.NoError(t, err)
			assert.Empty(t, result.ManifestGenerationError)
			assert.NotEmpty(t, result.TargetManifests)
		})
	}
}
