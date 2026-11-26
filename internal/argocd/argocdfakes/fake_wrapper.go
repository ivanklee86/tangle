package argocdfakes

import (
	"context"
	"fmt"
	"sync"

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
	Sources      []argocd.Source
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

	// requests records every GetManifests request, in arrival order. Guarded
	// by requestsMu: the CLI sends its diff requests concurrently.
	requestsMu sync.Mutex
	requests   []argocd.ManifestsRequest
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
			Sources:      app.Sources,
		})
	}

	return results, nil
}

// GetManifests returns the canned response for the application. It doesn't
// plan revisions: a fixture with no Plan reports an OK single-source diff. Tests
// of revision planning use a real argocd.ArgoCDWrapper over FakeClient.
func (f *FakeWrapper) GetManifests(_ context.Context, request argocd.ManifestsRequest) (*argocd.GetManifestsResponse, error) {
	f.requestsMu.Lock()
	f.requests = append(f.requests, request)
	f.requestsMu.Unlock()

	if err, ok := f.ErrOnGetManifests[request.ApplicationName]; ok {
		return nil, err
	}

	manifests, ok := f.ManifestsByApp[request.ApplicationName]
	if !ok {
		return nil, fmt.Errorf("argocdfakes: no manifests fixture for application %q", request.ApplicationName)
	}

	if manifests.Plan.Status == "" {
		withPlan := *manifests
		withPlan.Plan.Status = argocd.DiffStatusOK
		return &withPlan, nil
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
//
// It reaches those semantics by a different route, though: it matches the two
// maps directly and never builds a selector. So FakeWrapper cannot observe a
// bug in how that selector is constructed — #240, where an exclude-only query
// was sent with no selector at all, passes through here either way. A test
// that needs to cover selector construction wants a real argocd.ArgoCDWrapper
// over FakeClient, which does parse the string (see
// internal/tangle/handlers_test.go's newIntegrationTangle).
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

// Close satisfies argocd.IArgoCDWrapper. There's no connection behind a fake,
// so there's nothing to release.
func (f *FakeWrapper) Close() error {
	return nil
}

// Requests returns a copy of the GetManifests requests seen so far.
func (f *FakeWrapper) Requests() []argocd.ManifestsRequest {
	f.requestsMu.Lock()
	defer f.requestsMu.Unlock()

	return append([]argocd.ManifestsRequest(nil), f.requests...)
}
