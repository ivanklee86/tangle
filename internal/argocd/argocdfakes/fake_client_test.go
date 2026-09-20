package argocdfakes

import (
	"context"
	"errors"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	repoServerApiClient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/stretchr/testify/assert"
)

// Test cases for FakeClient.List:
//   - no selector returns every application
//   - an equality selector ("key=value") returns only matching applications
//   - a combined equality+inequality selector ("key=value,key!=value") is
//     parsed and applied the same way a real ArgoCD server would
//   - an invalid selector string errors
//   - ErrOnList, when set, short-circuits with that error
func TestFakeClient_List(t *testing.T) {
	tests := []struct {
		name        string
		errOnList   error
		selector    *string
		wantNames   []string
		wantErr     bool
		wantErrText string
	}{
		{
			name:      "no selector returns every application",
			selector:  nil,
			wantNames: []string{"test-1", "test-2", "test-3", "test-4"},
		},
		{
			name:      "equality selector filters",
			selector:  strPtr("env=test"),
			wantNames: []string{"test-1"},
		},
		{
			name:      "combined equality and inequality selector filters",
			selector:  strPtr("foo=bar,env!=test"),
			wantNames: []string{"test-2", "test-3", "test-4"},
		},
		{
			name:        "invalid selector errors",
			selector:    strPtr("!!!not a selector!!!"),
			wantErr:     true,
			wantErrText: "invalid selector",
		},
		{
			name:      "ErrOnList short-circuits",
			errOnList: errors.New("boom"),
			selector:  nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewFakeClient(ExampleApplications())
			client.ErrOnList = tt.errOnList

			query := &application.ApplicationQuery{Selector: tt.selector}
			got, err := client.List(context.Background(), query)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrText != "" {
					assert.Contains(t, err.Error(), tt.wantErrText)
				}
				return
			}

			assert.NoError(t, err)
			assert.Len(t, got.Items, len(tt.wantNames))

			gotNames := []string{}
			for _, app := range got.Items {
				gotNames = append(gotNames, app.Name)
			}
			assert.ElementsMatch(t, tt.wantNames, gotNames)
		})
	}
}

// Test cases for FakeClient.Get:
//   - a known application name returns that application
//   - an unknown application name errors
//   - ErrOnGet, keyed by name, short-circuits with that error
func TestFakeClient_Get(t *testing.T) {
	tests := []struct {
		name     string
		appName  string
		errOnGet map[string]error
		wantErr  bool
	}{
		{
			name:    "known application is returned",
			appName: "test-1",
		},
		{
			name:    "unknown application errors",
			appName: "does-not-exist",
			wantErr: true,
		},
		{
			name:     "ErrOnGet short-circuits for that name",
			appName:  "test-1",
			errOnGet: map[string]error{"test-1": errors.New("boom")},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewFakeClient(ExampleApplications())
			if tt.errOnGet != nil {
				client.ErrOnGet = tt.errOnGet
			}

			got, err := client.Get(context.Background(), &application.ApplicationQuery{Name: &tt.appName})

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.appName, got.Name)
		})
	}
}

func TestFakeClient_Get_RequiresName(t *testing.T) {
	client := NewFakeClient(ExampleApplications())

	_, err := client.Get(context.Background(), &application.ApplicationQuery{})
	assert.Error(t, err)
}

// Test cases for FakeClient.GetApplicationManifests:
//   - a name with a manifests fixture returns it
//   - a name with no manifests fixture errors ("not found")
//   - ErrOnGetManifests, keyed by name, short-circuits with that error
func TestFakeClient_GetApplicationManifests(t *testing.T) {
	tests := []struct {
		name              string
		appName           string
		errOnGetManifests map[string]error
		wantErr           bool
	}{
		{
			name:    "known application returns its manifests",
			appName: "test-1",
		},
		{
			name:    "application with no manifests fixture errors",
			appName: "test-2",
			wantErr: true,
		},
		{
			name:              "ErrOnGetManifests short-circuits for that name",
			appName:           "test-1",
			errOnGetManifests: map[string]error{"test-1": errors.New("boom")},
			wantErr:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewFakeClient(ExampleApplications())
			client.ManifestsByApp["test-1"] = &repoServerApiClient.ManifestResponse{Manifests: []string{"apiVersion: v1"}}
			if tt.errOnGetManifests != nil {
				client.ErrOnGetManifests = tt.errOnGetManifests
			}

			got, err := client.GetApplicationManifests(context.Background(), &application.ApplicationManifestQuery{Name: &tt.appName})

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, []string{"apiVersion: v1"}, got.Manifests)
		})
	}
}

func TestFakeClient_GetUrlAndScheme(t *testing.T) {
	client := NewFakeClient(ExampleApplications())

	assert.NotEmpty(t, client.GetUrl())
	assert.NotEmpty(t, client.GetScheme())

	client.Url = "argocd.example.com"
	client.Scheme = "https"
	assert.Equal(t, "argocd.example.com", client.GetUrl())
	assert.Equal(t, "https", client.GetScheme())
}

func strPtr(s string) *string {
	return &s
}
