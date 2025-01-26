package argocd

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewArgoCDClient(t *testing.T) {
	setup(t)

	tests := []struct {
		name    string
		options *ArgoCDClientOptions
		wantErr bool
	}{
		{
			name: "creates client with valid options",
			options: &ArgoCDClientOptions{
				Address:         "localhost:8080",
				Insecure:        true,
				AuthTokenEnvVar: "ARGOCD_TOKEN",
			},
			wantErr: false,
		},
		{
			name: "creates client with invalid options",
			options: &ArgoCDClientOptions{
				Address:         "https://localhost:8080",
				Insecure:        true,
				AuthTokenEnvVar: "ARGOCD_TOKEN1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewArgoCDClient(tt.options)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

func TestArgoCDClient_List(t *testing.T) {
	setup(t)

	labelQuery := "env=test"

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
				Insecure:        true,
				AuthTokenEnvVar: "ARGOCD_TOKEN",
			},
			query: &application.ApplicationQuery{
				Selector: &labelQuery,
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

func TestArgoCDClient_GetApplicationManifests(t *testing.T) {
	setup(t)

	applicationName := "test-1"
	// revision := "main"

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
				Insecure:        true,
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
