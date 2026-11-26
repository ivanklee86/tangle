// Package argocdfakes provides in-memory implementations of
// argocd.IArgoCDClient and argocd.IArgoCDWrapper, for tests that need to
// exercise code above the real gRPC-web wire protocol (pooling,
// label-filtering, error-propagation) without a live ArgoCD instance.
package argocdfakes

import (
	"context"
	"fmt"
	"sync"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoServerApiClient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/ivanklee86/tangle/internal/argocd"
)

// FakeClient is an in-memory argocd.IArgoCDClient backed by a fixed list of
// applications, filtered the same way a real ArgoCD server interprets a
// label selector (via k8s.io/apimachinery/pkg/labels). It's for
// internal/argocd's own tests — ArgoCDWrapper's pooling/error-propagation
// logic — which need a client one layer below to exercise, not a live
// ArgoCD's wire protocol.
type FakeClient struct {
	Applications []v1alpha1.Application

	// ManifestsByApp maps an application name to the manifests
	// GetApplicationManifests returns for it. A name with no entry (and no
	// ErrOnGetManifests override) is treated as "not found", mirroring real
	// ArgoCD's behavior for an unknown application.
	ManifestsByApp map[string]*repoServerApiClient.ManifestResponse

	// Url/Scheme back GetUrl/GetScheme.
	Url    string
	Scheme string

	// ErrOnList, if set, is returned by every List call.
	ErrOnList error

	// ListQueries records every ApplicationQuery List was called with, in
	// order, so tests can assert on the selector string a caller built
	// rather than inferring it from which applications came back. Guarded by
	// listMu: ArgoCDWrapper issues List from a pond worker pool.
	listMu      sync.Mutex
	ListQueries []*application.ApplicationQuery

	// ErrOnGet, keyed by application name, is returned by Get for that name.
	ErrOnGet map[string]error

	// ErrOnGetManifests, keyed by application name, is returned by
	// GetApplicationManifests for that name.
	ErrOnGetManifests map[string]error

	// ManifestsFunc, if set, answers every GetApplicationManifests call that
	// ErrOnGetManifests doesn't, in place of ManifestsByApp. Use it when the
	// live and target sides must differ, by looking at the query's revisions.
	ManifestsFunc func(*application.ApplicationManifestQuery) (*repoServerApiClient.ManifestResponse, error)

	// GetQueries and ManifestQueries record every Get and
	// GetApplicationManifests call, in order, so tests can assert on exactly
	// what reached ArgoCD. Guarded by callMu: both run on worker pools.
	callMu          sync.Mutex
	GetQueries      []*application.ApplicationQuery
	ManifestQueries []*application.ApplicationManifestQuery
}

// RecordedGets returns a copy of the Get queries seen so far.
func (f *FakeClient) RecordedGets() []*application.ApplicationQuery {
	f.callMu.Lock()
	defer f.callMu.Unlock()

	return append([]*application.ApplicationQuery(nil), f.GetQueries...)
}

// RecordedManifestQueries returns a copy of the manifest queries seen so far.
func (f *FakeClient) RecordedManifestQueries() []*application.ApplicationManifestQuery {
	f.callMu.Lock()
	defer f.callMu.Unlock()

	return append([]*application.ApplicationManifestQuery(nil), f.ManifestQueries...)
}

// NewFakeClient returns a FakeClient seeded with apps and ready to use.
func NewFakeClient(apps []v1alpha1.Application) *FakeClient {
	return &FakeClient{
		Applications:      apps,
		ManifestsByApp:    map[string]*repoServerApiClient.ManifestResponse{},
		Url:               "localhost:8080",
		Scheme:            "http",
		ErrOnGet:          map[string]error{},
		ErrOnGetManifests: map[string]error{},
	}
}

func (f *FakeClient) List(_ context.Context, in *application.ApplicationQuery) (*v1alpha1.ApplicationList, error) {
	f.listMu.Lock()
	f.ListQueries = append(f.ListQueries, in)
	f.listMu.Unlock()

	if f.ErrOnList != nil {
		return nil, f.ErrOnList
	}

	selector := labels.Everything()
	if in != nil && in.Selector != nil && *in.Selector != "" {
		parsed, err := labels.Parse(*in.Selector)
		if err != nil {
			return nil, fmt.Errorf("argocdfakes: invalid selector %q: %w", *in.Selector, err)
		}
		selector = parsed
	}

	items := []v1alpha1.Application{}
	for _, app := range f.Applications {
		if selector.Matches(labels.Set(app.Labels)) {
			items = append(items, app)
		}
	}

	return &v1alpha1.ApplicationList{Items: items}, nil
}

// LastListSelector returns the selector string from the most recent List
// call and whether one was set at all. A query sent with no selector — what
// an unfiltered "list everything" request looks like — reports false, which
// is the distinction ArgoCDWrapper gets wrong when it drops an exclude-only
// selector.
func (f *FakeClient) LastListSelector() (string, bool) {
	f.listMu.Lock()
	defer f.listMu.Unlock()

	if len(f.ListQueries) == 0 {
		return "", false
	}

	query := f.ListQueries[len(f.ListQueries)-1]
	if query == nil || query.Selector == nil {
		return "", false
	}

	return *query.Selector, true
}

// ListCallCount reports how many times List has been called.
func (f *FakeClient) ListCallCount() int {
	f.listMu.Lock()
	defer f.listMu.Unlock()

	return len(f.ListQueries)
}

func (f *FakeClient) Get(_ context.Context, in *application.ApplicationQuery) (*v1alpha1.Application, error) {
	if in == nil || in.Name == nil {
		return nil, fmt.Errorf("argocdfakes: Get requires a query with Name set")
	}

	f.callMu.Lock()
	f.GetQueries = append(f.GetQueries, in)
	f.callMu.Unlock()

	if err, ok := f.ErrOnGet[*in.Name]; ok {
		return nil, err
	}

	for _, app := range f.Applications {
		if app.Name == *in.Name {
			appCopy := app
			return &appCopy, nil
		}
	}

	return nil, fmt.Errorf("argocdfakes: application %q not found", *in.Name)
}

func (f *FakeClient) GetApplicationManifests(_ context.Context, in *application.ApplicationManifestQuery) (*repoServerApiClient.ManifestResponse, error) {
	if in == nil || in.Name == nil {
		return nil, fmt.Errorf("argocdfakes: GetApplicationManifests requires a query with Name set")
	}

	f.callMu.Lock()
	f.ManifestQueries = append(f.ManifestQueries, in)
	f.callMu.Unlock()

	if err, ok := f.ErrOnGetManifests[*in.Name]; ok {
		return nil, err
	}

	if f.ManifestsFunc != nil {
		return f.ManifestsFunc(in)
	}

	manifests, ok := f.ManifestsByApp[*in.Name]
	if !ok {
		return nil, fmt.Errorf("argocdfakes: no manifests fixture for application %q", *in.Name)
	}

	return manifests, nil
}

func (f *FakeClient) GetUrl() string {
	return f.Url
}

func (f *FakeClient) GetScheme() string {
	return f.Scheme
}

var _ argocd.IArgoCDClient = (*FakeClient)(nil)

// Close satisfies argocd.IArgoCDClient. There's no connection behind a fake, so
// there's nothing to release.
func (f *FakeClient) Close() error {
	return nil
}
