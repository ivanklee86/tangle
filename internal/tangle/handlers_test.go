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
			name:       "invalid_tags",
			url:        "/applications?labels=foobar",
			test_count: 2,
			prod_count: 2,
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
