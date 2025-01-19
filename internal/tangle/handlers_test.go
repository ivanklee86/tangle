package tangle

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
