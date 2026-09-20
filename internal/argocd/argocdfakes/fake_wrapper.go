package argocdfakes

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	"github.com/ivanklee86/tangle/internal/argocd"
)

// FakeApplication is the fixture shape FakeWrapper filters against directly.
// Unlike argocd.ListApplicationsResult (ArgoCDWrapper's own output shape),
// it retains Labels, since FakeWrapper does its own label-matching in place
// of a real ArgoCD server's selector evaluation — the maps
// IArgoCDWrapper.ListApplicationsByLabels receives never make it into
// ListApplicationsResult.
type FakeApplication struct {
	Name         string
	Project      string
	Namespace    string
	Labels       map[string]string
	Health       v1alpha1.AppHealthStatus
	SyncStatus   v1alpha1.SyncStatus
	LiveRevision string
}

// FakeWrapper is an in-memory argocd.IArgoCDWrapper backed by a fixed list
// of applications. It's for internal/tangle's handler tests, which operate
// one layer above ArgoCDWrapper's own pooling logic — they need canned
// per-ArgoCD results, not the pond worker-pool orchestration exercised.
type FakeWrapper struct {
	Applications []FakeApplication

	// ManifestsByApp maps an application name to the GetManifestsResponse
	// GetManifests returns for it. A name with no entry (and no
	// ErrOnGetManifests override) is treated as "not found".
	ManifestsByApp map[string]*argocd.GetManifestsResponse

	// Url/Scheme back GetUrl/GetScheme.
	Url    string
	Scheme string

	// ErrOnListApplicationsByLabels, if set, is returned by every
	// ListApplicationsByLabels call.
	ErrOnListApplicationsByLabels error

	// ErrOnGetManifests, keyed by application name, is returned by
	// GetManifests for that name.
	ErrOnGetManifests map[string]error
}

// NewFakeWrapper returns a FakeWrapper seeded with apps and ready to use.
func NewFakeWrapper(apps []FakeApplication) *FakeWrapper {
	return &FakeWrapper{
		Applications:      apps,
		ManifestsByApp:    map[string]*argocd.GetManifestsResponse{},
		Url:               "localhost:8080",
		Scheme:            "http",
		ErrOnGetManifests: map[string]error{},
	}
}

func (f *FakeWrapper) ListApplicationsByLabels(_ context.Context, includeLabels map[string]string, excludeLabels map[string]string) ([]argocd.ListApplicationsResult, error) {
	if f.ErrOnListApplicationsByLabels != nil {
		return nil, f.ErrOnListApplicationsByLabels
	}

	results := []argocd.ListApplicationsResult{}
	for _, app := range f.Applications {
		if !matchesLabels(app.Labels, includeLabels, excludeLabels) {
			continue
		}

		results = append(results, argocd.ListApplicationsResult{
			Name:         app.Name,
			Project:      app.Project,
			Namespace:    app.Namespace,
			Health:       app.Health,
			SyncStatus:   app.SyncStatus,
			LiveRevision: app.LiveRevision,
		})
	}

	return results, nil
}

func (f *FakeWrapper) GetManifests(_ context.Context, applicationName string, _ string, _ string) (*argocd.GetManifestsResponse, error) {
	if err, ok := f.ErrOnGetManifests[applicationName]; ok {
		return nil, err
	}

	manifests, ok := f.ManifestsByApp[applicationName]
	if !ok {
		return nil, fmt.Errorf("argocdfakes: no manifests fixture for application %q", applicationName)
	}

	return manifests, nil
}

func (f *FakeWrapper) GetUrl() string {
	return f.Url
}

func (f *FakeWrapper) GetScheme() string {
	return f.Scheme
}

// matchesLabels reports whether candidate satisfies every includeLabels
// equality and every excludeLabels inequality — the same semantics
// ArgoCDWrapper.ListApplicationsByLabels asks a real ArgoCD server for by
// combining both maps into a single "key=value,key!=value" selector string.
func matchesLabels(candidate map[string]string, includeLabels map[string]string, excludeLabels map[string]string) bool {
	for key, value := range includeLabels {
		if candidate[key] != value {
			return false
		}
	}

	for key, value := range excludeLabels {
		if candidate[key] == value {
			return false
		}
	}

	return true
}

var _ argocd.IArgoCDWrapper = (*FakeWrapper)(nil)
