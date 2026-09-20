//go:build e2e

package argocd

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

// setup loads the live ArgoCD auth tokens `task argocd:token` mints into
// .env. Every test in this file needs a real ArgoCD reachable at
// localhost:8080 (see `task services:cicd`) — ArgoCDClient is the thing that
// talks real gRPC-web to ArgoCD, so there's nothing left to fake underneath
// it without just testing our own assumptions about ArgoCD's behavior.
func setup(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}
}

// TestE2E_NewArgoCDClient_ValidOptions is split out from the unit-tested
// TestNewArgoCDClient (client_test.go, no build tag — always compiled, so a
// same-named function here would collide when building with -tags=e2e).
// Unlike the "invalid options" case, which only exercises this package's own
// missing-token-env-var validation, a valid ARGOCD_TOKEN still needs a real
// ArgoCD server to dial successfully — NewClientOrDie/NewApplicationClientOrDie
// do an eager handshake — so this case belongs at the e2e layer.
//
// Every top-level test in this file is prefixed TestE2E_ (matching
// internal/tangle/server_e2e_test.go's TestE2E_Server*) so `-run '^TestE2E_'`
// (tasks/go.yaml's test:e2e/test:e2e:ci) can select just these, instead of
// also re-running every untagged unit/integration test that -tags=e2e still
// compiles alongside them.
func TestE2E_NewArgoCDClient_ValidOptions(t *testing.T) {
	setup(t)

	got, err := NewArgoCDClient(&ArgoCDClientOptions{
		Address:         "localhost:8080",
		PlainText:       true,
		AuthTokenEnvVar: "ARGOCD_TOKEN",
	})
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

func TestE2E_ArgoCDClient_List(t *testing.T) {
	setup(t)

	labelQueryInclude := "foo=bar"
	labelQueryExclude := "foo=bar,env!=test"

	tests := []struct {
		name          string
		options       *ArgoCDClientOptions
		query         *application.ApplicationQuery
		resultsLength int
		wantErr       bool
	}{
		{
			name: "lists applications successfully",
			options: &ArgoCDClientOptions{
				Address:         "localhost:8080",
				PlainText:       true,
				AuthTokenEnvVar: "ARGOCD_TOKEN",
			},
			query: &application.ApplicationQuery{
				Selector: &labelQueryInclude,
			},
			resultsLength: 2,
			wantErr:       false,
		},
		{
			name: "lists applications with exclude labels",
			options: &ArgoCDClientOptions{
				Address:         "localhost:8080",
				PlainText:       true,
				AuthTokenEnvVar: "ARGOCD_TOKEN",
			},
			query: &application.ApplicationQuery{
				Selector: &labelQueryExclude,
			},
			resultsLength: 1,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewArgoCDClient(tt.options)
			assert.NoError(t, err)

			got, err := client.List(context.Background(), tt.query)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Len(t, got.Items, tt.resultsLength)
		})
	}
}

func TestE2E_ArgoCDClient_GetApplicationManifests(t *testing.T) {
	setup(t)

	applicationName := "test-1"

	tests := []struct {
		name    string
		options *ArgoCDClientOptions
		query   *application.ApplicationManifestQuery
		wantErr bool
	}{
		{
			name: "gets application manifests successfully",
			options: &ArgoCDClientOptions{
				Address:         "localhost:8080",
				PlainText:       true,
				AuthTokenEnvVar: "ARGOCD_TOKEN",
			},
			query: &application.ApplicationManifestQuery{
				Name: &applicationName,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewArgoCDClient(tt.options)
			assert.NoError(t, err)

			got, err := client.GetApplicationManifests(context.Background(), tt.query)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

func TestE2E_ArgoCDClient_Get(t *testing.T) {
	setup(t)
	applicationName := "test-1"
	refresh := "hard"

	tests := []struct {
		name    string
		options *ArgoCDClientOptions
		query   *application.ApplicationQuery
		wantErr bool
	}{
		{
			name: "gets application successfully",
			options: &ArgoCDClientOptions{
				Address:         "localhost:8080",
				PlainText:       true,
				AuthTokenEnvVar: "ARGOCD_TOKEN",
			},
			query: &application.ApplicationQuery{
				Name:    &applicationName,
				Refresh: &refresh,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewArgoCDClient(tt.options)
			assert.NoError(t, err)

			got, err := client.Get(context.Background(), tt.query)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}
