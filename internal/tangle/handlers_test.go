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
	"github.com/stretchr/testify/assert"
)

func TestHandlers(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}

	argocdConfig := make(map[string]TangleArgoCDConfig)
	argocdConfig["test"] = TangleArgoCDConfig{
		Address:         "localhost:8080",
		Insecure:        true,
		AuthTokenEnvVar: "ARGOCD_TOKEN",
	}
	argocdConfig["prod"] = TangleArgoCDConfig{
		Address:         "localhost:8080",
		Insecure:        true,
		AuthTokenEnvVar: "ARGOCD_PROD_TOKEN",
	}

	config := TangleConfig{
		Name:            "test-tangle",
		Domain:          "localhost",
		Port:            8081,
		ArgoCDs:         argocdConfig,
		DoNotInstrument: true,
	}

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
			prod_count: 1,
		},
		{
			name:       "tags_match_all",
			url:        "/applications?labels=foo:bar",
			test_count: 2,
			prod_count: 1,
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tangle := New(&config)

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
				if result.Name == "test" {
					assert.Equal(t, test.test_count, len(result.Applications))
				} else if result.Name == "prod" {
					assert.Equal(t, test.prod_count, len(result.Applications))
				}
			}
		})
	}
}

func TestHandlersError(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}

	argocdConfig := make(map[string]TangleArgoCDConfig)
	argocdConfig["test"] = TangleArgoCDConfig{
		Address:         "localhost:8080",
		Insecure:        true,
		AuthTokenEnvVar: "ARGOCD_TOKEN",
	}
	argocdConfig["prod"] = TangleArgoCDConfig{
		Address:         "https://localhost:8080",
		Insecure:        true,
		AuthTokenEnvVar: "ARGOCD_PROD_TOKEN",
	}

	config := TangleConfig{
		Name:            "test-tangle",
		Domain:          "localhost",
		Port:            8081,
		ArgoCDs:         argocdConfig,
		DoNotInstrument: true,
	}

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
			tangle := New(&config)

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
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}

	argocdConfig := make(map[string]TangleArgoCDConfig)
	argocdConfig["test"] = TangleArgoCDConfig{
		Address:         "localhost:8080",
		Insecure:        true,
		AuthTokenEnvVar: "ARGOCD_TOKEN",
	}
	argocdConfig["prod"] = TangleArgoCDConfig{
		Address:         "localhost:8080",
		Insecure:        true,
		AuthTokenEnvVar: "ARGOCD_PROD_TOKEN",
	}

	config := TangleConfig{
		Name:            "test-tangle",
		Domain:          "localhost",
		Port:            8081,
		ArgoCDs:         argocdConfig,
		DoNotInstrument: true,
	}

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
			tangle := New(&config)

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
