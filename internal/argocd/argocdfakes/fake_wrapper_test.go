package argocdfakes

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ivanklee86/tangle/internal/argocd"
)

var fakeManifestsResponse = argocd.GetManifestsResponse{
	LiveManifests:   []string{"apiVersion: v1\nkind: ConfigMap"},
	TargetManifests: []string{"apiVersion: v1\nkind: ConfigMap\ndata:\n  updated: \"true\""},
}

func exampleWrapperApplications() []FakeApplication {
	return []FakeApplication{
		{Name: "test-1", Project: "default", Namespace: "argocd", Labels: map[string]string{"env": "test", "foo": "bar", "bazz": "buzz"}},
		{Name: "test-2", Project: "default", Namespace: "argocd", Labels: map[string]string{"env": "preprod", "foo": "bar", "bazz": "buzz"}},
		{Name: "test-3", Project: "my-project", Namespace: "argocd", Labels: map[string]string{"env": "prod", "foo": "bar"}},
		{Name: "test-4", Project: "my-project", Namespace: "argocd", Labels: map[string]string{"env": "infra", "foo": "bar"}},
	}
}

// Test cases for FakeWrapper.ListApplicationsByLabels:
//   - no include/exclude labels returns every fixture application
//   - include labels are AND-ed across multiple keys
//   - include and exclude labels combine (matches include, doesn't match any exclude)
//   - ErrOnListApplicationsByLabels, when set, short-circuits with that error
func TestFakeWrapper_ListApplicationsByLabels(t *testing.T) {
	tests := []struct {
		name          string
		errOnList     error
		includeLabels map[string]string
		excludeLabels map[string]string
		wantNames     []string
	}{
		{
			name:      "no labels returns every application",
			wantNames: []string{"test-1", "test-2", "test-3", "test-4"},
		},
		{
			name:          "include labels AND across keys",
			includeLabels: map[string]string{"env": "test", "bazz": "buzz"},
			wantNames:     []string{"test-1"},
		},
		{
			name:          "include and exclude labels combine",
			includeLabels: map[string]string{"foo": "bar"},
			excludeLabels: map[string]string{"env": "test"},
			wantNames:     []string{"test-2", "test-3", "test-4"},
		},
		{
			name:          "no match returns an empty (not nil) result",
			includeLabels: map[string]string{"env": "does-not-exist"},
			wantNames:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper := NewFakeWrapper(exampleWrapperApplications())

			got, err := wrapper.ListApplicationsByLabels(context.Background(), tt.includeLabels, tt.excludeLabels)
			assert.NoError(t, err)
			assert.NotNil(t, got)

			gotNames := []string{}
			for _, result := range got {
				gotNames = append(gotNames, result.Name)
			}
			assert.ElementsMatch(t, tt.wantNames, gotNames)
		})
	}
}

func TestFakeWrapper_ListApplicationsByLabels_ErrOnList(t *testing.T) {
	wrapper := NewFakeWrapper(exampleWrapperApplications())
	wrapper.ErrOnListApplicationsByLabels = errors.New("boom")

	_, err := wrapper.ListApplicationsByLabels(context.Background(), nil, nil)
	assert.Error(t, err)
}

// Test cases for FakeWrapper.GetManifests:
//   - a name with a manifests fixture returns it
//   - a name with no manifests fixture errors ("not found")
//   - ErrOnGetManifests, keyed by name, short-circuits with that error
func TestFakeWrapper_GetManifests(t *testing.T) {
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
			wrapper := NewFakeWrapper(exampleWrapperApplications())
			wrapper.ManifestsByApp["test-1"] = &fakeManifestsResponse
			if tt.errOnGetManifests != nil {
				wrapper.ErrOnGetManifests = tt.errOnGetManifests
			}

			got, err := wrapper.GetManifests(context.Background(), tt.appName, "main", "test_gitops")

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, &fakeManifestsResponse, got)
		})
	}
}

func TestFakeWrapper_GetUrlAndScheme(t *testing.T) {
	wrapper := NewFakeWrapper(exampleWrapperApplications())

	assert.NotEmpty(t, wrapper.GetUrl())
	assert.NotEmpty(t, wrapper.GetScheme())

	wrapper.Url = "argocd.example.com"
	wrapper.Scheme = "https"
	assert.Equal(t, "argocd.example.com", wrapper.GetUrl())
	assert.Equal(t, "https", wrapper.GetScheme())
}
