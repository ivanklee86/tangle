package tangle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/ivanklee86/tangle/internal/argocd"
	"github.com/ivanklee86/tangle/internal/argocd/argocdfakes"
)

// testArgoCDApplications and prodArgoCDApplications mirror the RBAC-scoped
// views a real ArgoCD server gave the "test"/"prod" ArgoCDs configured in
// integration/tangle.yaml before this file moved to argocdfakes — the
// "default" project (test-1/test-2) and "my-project" (test-3/test-4)
// subsets of argocdfakes.ExampleApplications().
func testArgoCDApplications() []argocdfakes.FakeApplication {
	return []argocdfakes.FakeApplication{
		{Name: "test-1", Project: "default", Namespace: "argocd", Labels: map[string]string{"env": "test", "foo": "bar", "bazz": "buzz"}, LiveRevision: "main"},
		{Name: "test-2", Project: "default", Namespace: "argocd", Labels: map[string]string{"env": "preprod", "foo": "bar", "bazz": "buzz"}, LiveRevision: "main"},
	}
}

func prodArgoCDApplications() []argocdfakes.FakeApplication {
	return []argocdfakes.FakeApplication{
		{Name: "test-3", Project: "my-project", Namespace: "argocd", Labels: map[string]string{"env": "prod", "foo": "bar"}, LiveRevision: "main"},
		{Name: "test-4", Project: "my-project", Namespace: "argocd", Labels: map[string]string{"env": "infra", "foo": "bar"}, LiveRevision: "main"},
	}
}

// newTestTangle builds a real *Tangle (router/logger/middleware intact) and
// then overwrites its ArgoCDs map with fakes — New()'s own wrapper
// construction never gets called before that swap, so nothing it does with
// the (unreachable) real ArgoCDClientOptions matters.
func newTestTangle() *Tangle {
	argocdConfig := make(map[string]TangleArgoCDConfig)
	argocdConfig["test"] = TangleArgoCDConfig{
		Address:         "localhost:8080",
		PlainText:       true,
		AuthTokenEnvVar: "ARGOCD_TOKEN",
	}
	argocdConfig["prod"] = TangleArgoCDConfig{
		Address:         "localhost:8080",
		PlainText:       true,
		AuthTokenEnvVar: "ARGOCD_PROD_TOKEN",
	}

	config := TangleConfig{
		Name:            "test-tangle",
		Domain:          "localhost",
		Port:            8081,
		ArgoCDs:         argocdConfig,
		DoNotInstrument: true,
	}

	tangle := New(&config, "testing")
	tangle.ArgoCDs = map[string]argocd.IArgoCDWrapper{
		"test": argocdfakes.NewFakeWrapper(testArgoCDApplications()),
		"prod": argocdfakes.NewFakeWrapper(prodArgoCDApplications()),
	}

	return tangle
}

// newIntegrationTangle builds a *Tangle whose ArgoCDs are real
// argocd.ArgoCDWrapper values over argocdfakes.FakeClient, so a request
// travels the production path: the handler's parsed label maps become one
// Kubernetes selector string, which FakeClient parses with
// k8s.io/apimachinery/pkg/labels exactly as a real ArgoCD server would.
//
// newTestTangle's FakeWrapper deliberately short-circuits that — it matches
// the two maps directly — which makes it useless for selector-construction
// bugs. #240 (an exclude-only query dropping its selector and returning every
// application) is invisible through FakeWrapper and caught here.
//
// The applications are split the same way newTestTangle splits them, and the
// same way the live cluster's RBAC does: "default" project to the test
// instance, "my-project" to prod.
func newIntegrationTangle(t *testing.T) (*Tangle, map[string]*argocdfakes.FakeClient) {
	t.Helper()

	tangle := newTestTangle()

	clients := map[string]*argocdfakes.FakeClient{
		"test": argocdfakes.NewFakeClient(argocdfakes.ExampleApplications()[:2]),
		"prod": argocdfakes.NewFakeClient(argocdfakes.ExampleApplications()[2:]),
	}

	for name, client := range clients {
		// DoNotInstrumentWorkers, or the second subtest panics re-registering
		// the same pool metrics on Prometheus's global registry.
		wrapper, err := argocd.New(client, name, &argocd.ArgoCDWrapperOptions{
			DoNotInstrumentWorkers: true,
		})
		assert.NoError(t, err)

		tangle.ArgoCDs[name] = wrapper
	}

	return tangle, clients
}

func TestHandlers(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		test_count int
		prod_count int
	}{
		{
			name:       "no_tags",
			url:        "/applications",
			test_count: 2,
			prod_count: 2,
		},
		{
			name:       "tags_match_all",
			url:        "/applications?labels=foo:bar",
			test_count: 2,
			prod_count: 2,
		},
		{
			name:       "tags_match_one",
			url:        "/applications?labels=env:test",
			test_count: 1,
			prod_count: 0,
		},
		{
			name:       "tags_match_none",
			url:        "/applications?labels=env:foobar",
			test_count: 0,
			prod_count: 0,
		},
		{
			name:       "multiple_tags",
			url:        "/applications?labels=env:test,bazz:buzz",
			test_count: 1,
			prod_count: 0,
		},
		{
			name:       "exclude",
			url:        "/applications?labels=foo:bar&excludeLabels=env:test",
			test_count: 1,
			prod_count: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tangle := newTestTangle()

			req, _ := http.NewRequest("GET", test.url, nil)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(tangle.applicationsHandler)
			handler.ServeHTTP(rr, req)

			assert.NotNil(t, rr.Body.String())
			assert.Equal(t, http.StatusOK, rr.Code)

			var result ApplicationsResponse
			err := json.NewDecoder(rr.Body).Decode(&result)
			assert.Nil(t, err)
			for _, result := range result.Results {
				switch result.Name {
				case "test":
					assert.Equal(t, test.test_count, len(result.Applications))
				case "prod":
					assert.Equal(t, test.prod_count, len(result.Applications))
				}
			}
		})
	}
}

func TestHandlersError(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedErr int
	}{
		{
			name:        "applications",
			url:         "/applications",
			expectedErr: http.StatusInternalServerError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tangle := newTestTangle()
			prodWrapper := argocdfakes.NewFakeWrapper(prodArgoCDApplications())
			prodWrapper.ErrOnListApplicationsByLabels = errors.New("simulated connection error")
			tangle.ArgoCDs["prod"] = prodWrapper

			req, _ := http.NewRequest("GET", test.url, nil)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(tangle.applicationsHandler)
			handler.ServeHTTP(rr, req)

			assert.NotNil(t, rr.Body.String())
			assert.Equal(t, test.expectedErr, rr.Code)

			var result ErrorResponse
			err := json.NewDecoder(rr.Body).Decode(&result)
			assert.Nil(t, err)

			assert.NotNil(t, result.Error)
		})
	}
}

func TestDiffs(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		requestBody map[string]interface{}
	}{
		{
			name: "happy_path",
			url:  "/api/argocd/test/applications/test-1/diffs",
			requestBody: map[string]interface{}{
				"currentRef": "main",
				"compareRef": "test_gitops",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tangle := newTestTangle()
			testWrapper := argocdfakes.NewFakeWrapper(testArgoCDApplications())
			testWrapper.ManifestsByApp["test-1"] = &argocd.GetManifestsResponse{
				LiveManifests:   []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example\n"},
				TargetManifests: []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example\ndata:\n  updated: \"true\"\n"},
			}
			tangle.ArgoCDs["test"] = testWrapper

			body, _ := json.Marshal(test.requestBody)
			req, _ := http.NewRequest("POST", test.url, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("argocd", "test")
			rctx.URLParams.Add("name", "test-1")
			ctx := context.Background()
			ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(tangle.applicationManifestsHandler)
			handler.ServeHTTP(rr, req)

			assert.NotNil(t, rr.Body.String())
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

// TestDiffsError guards against a regression where a GetManifests error fell
// through into the success path instead of returning, nil-dereferencing the
// (nil, on error) *argocd.GetManifestsResponse right after the error
// response had already been written — silently recovered by chi's
// Recoverer, so it never failed a test despite panicking on every real
// GetManifests error. See docs/agents/plans/go-major-dependency-migration.md's
// Outcome section for how this was found.
func TestDiffsError(t *testing.T) {
	t.Run("GetManifests error", func(t *testing.T) {
		tangle := newTestTangle()
		testWrapper := argocdfakes.NewFakeWrapper(testArgoCDApplications())
		testWrapper.ErrOnGetManifests["test-1"] = errors.New("simulated manifest generation error")
		tangle.ArgoCDs["test"] = testWrapper

		requestBody := map[string]interface{}{
			"currentRef": "main",
			"compareRef": "test_gitops",
		}
		body, _ := json.Marshal(requestBody)
		req, _ := http.NewRequest("POST", "/api/argocd/test/applications/test-1/diffs", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("argocd", "test")
		rctx.URLParams.Add("name", "test-1")
		ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tangle.applicationManifestsHandler)
		assert.NotPanics(t, func() { handler.ServeHTTP(rr, req) })

		assert.Equal(t, http.StatusOK, rr.Code)

		var result DiffsResponse
		err := json.NewDecoder(rr.Body).Decode(&result)
		assert.Nil(t, err)
		assert.Equal(t, "simulated manifest generation error", result.ManifestGenerationError)
		assert.Empty(t, result.LiveManifests)
		assert.Empty(t, result.TargetManifests)
		assert.Empty(t, result.Diffs)
	})
}

// TestHandlersIntegration runs the label matrix against real ArgoCDWrapper
// values over FakeClient, so the selector the handler's maps produce is
// actually parsed and applied. These rows are the end-to-end statement of
// #240: through FakeWrapper the exclude-only cases pass either way, because
// it never builds a selector at all.
func TestHandlersIntegration(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		test_count int
		prod_count int
	}{
		{
			name:       "no_tags",
			url:        "/api/applications",
			test_count: 2,
			prod_count: 2,
		},
		{
			name:       "tags_match_all",
			url:        "/api/applications?labels=foo:bar",
			test_count: 2,
			prod_count: 2,
		},
		{
			name:       "tags_match_one",
			url:        "/api/applications?labels=env:test",
			test_count: 1,
			prod_count: 0,
		},
		{
			name:       "multiple_tags",
			url:        "/api/applications?labels=env:test,bazz:buzz",
			test_count: 1,
			prod_count: 0,
		},
		{
			name:       "exclude_and_include",
			url:        "/api/applications?labels=foo:bar&excludeLabels=env:test",
			test_count: 1,
			prod_count: 2,
		},
		{
			// #240: before the fix this returned 2 and 2 — the selector was
			// never sent, so ArgoCD listed everything.
			name:       "exclude_only",
			url:        "/api/applications?excludeLabels=env:test",
			test_count: 1,
			prod_count: 2,
		},
		{
			// prod_count is 2, not 0: the prod fixtures carry no "bazz"
			// label at all, and a Kubernetes "!=" requirement matches a key
			// that is absent.
			name:       "exclude_only_multiple",
			url:        "/api/applications?excludeLabels=env:test,bazz:buzz",
			test_count: 0,
			prod_count: 2,
		},
		{
			// The same key in both maps with different values is redundant
			// but satisfiable, and must not be rejected.
			name:       "cross_map_different_values",
			url:        "/api/applications?labels=env:test&excludeLabels=env:prod",
			test_count: 1,
			prod_count: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tangle, _ := newIntegrationTangle(t)

			server := httptest.NewServer(tangle.Server.Handler)
			defer server.Close()

			resp, err := http.Get(server.URL + test.url)
			assert.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var result ApplicationsResponse
			assert.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

			counts := map[string]int{}
			for _, argoCDResult := range result.Results {
				counts[argoCDResult.Name] = len(argoCDResult.Applications)
			}

			assert.Equal(t, test.test_count, counts["test"], "test instance")
			assert.Equal(t, test.prod_count, counts["prod"], "prod instance")
		})
	}
}

// TestHandlersBadRequest covers #241: a label parameter that can't be turned
// into the selector the caller described is refused, rather than answered
// with a result set that doesn't match the request.
//
// These go through tangle.Server.Handler — the real chi router and its
// middleware — rather than calling applicationsHandler directly, because a
// new status code is exactly the kind of thing middleware could rewrite, and
// nothing else in this package covers the route as mounted.
func TestHandlersBadRequest(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantContains []string
	}{
		{
			name:         "malformed_include",
			url:          "/api/applications?labels=env",
			wantContains: []string{`"env"`, "labels", "key:value"},
		},
		{
			name:         "malformed_exclude",
			url:          "/api/applications?excludeLabels=env",
			wantContains: []string{`"env"`, "excludeLabels", "key:value"},
		},
		{
			name:         "too_many_separators",
			url:          "/api/applications?labels=env:test:extra",
			wantContains: []string{`"env:test:extra"`, "labels"},
		},
		{
			name:         "malformed_alongside_valid",
			url:          "/api/applications?labels=env,team:platform",
			wantContains: []string{`"env"`, "labels"},
		},
		{
			name:         "duplicate_include_key",
			url:          "/api/applications?labels=env:test,env:prod",
			wantContains: []string{`"env"`, "labels", "at most once"},
		},
		{
			name:         "duplicate_exclude_key",
			url:          "/api/applications?excludeLabels=env:test,env:prod",
			wantContains: []string{`"env"`, "excludeLabels", "at most once"},
		},
		{
			name:         "contradictory_pair",
			url:          "/api/applications?labels=env:test&excludeLabels=env:test",
			wantContains: []string{`"env"`, "labels", "excludeLabels", "no application can match"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tangle, clients := newIntegrationTangle(t)

			server := httptest.NewServer(tangle.Server.Handler)
			defer server.Close()

			resp, err := http.Get(server.URL + test.url)
			assert.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var result ErrorResponse
			assert.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
			for _, want := range test.wantContains {
				assert.Contains(t, result.Error, want)
			}

			// A rejected request must not reach ArgoCD. This is a separate
			// promise from the status code: answering a malformed query
			// anyway costs a fan-out to every configured instance for a
			// result the caller can't use.
			for name, client := range clients {
				assert.Zero(t, client.ListCallCount(), "%s instance was queried for a rejected request", name)
			}
		})
	}
}

// TestHandlersValidLabelsAreNotRejected guards the other direction: the
// validation added for #241 must not refuse queries that are merely
// redundant or unfiltered.
func TestHandlersValidLabelsAreNotRejected(t *testing.T) {
	urls := []string{
		"/api/applications",
		"/api/applications?labels=env:test&excludeLabels=env:prod",
		"/api/applications?labels=env:test,foo:bar",
		"/api/applications?excludeLabels=env:test,foo:bar",
	}

	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			tangle, _ := newIntegrationTangle(t)

			server := httptest.NewServer(tangle.Server.Handler)
			defer server.Close()

			resp, err := http.Get(server.URL + url)
			assert.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}
