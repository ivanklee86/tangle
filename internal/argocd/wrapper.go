package argocd

import (
	"context"
	"slices"
	"strings"

	"github.com/alitto/pond/v2"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
)

const (
	defaultListPoolWorkers        = 10
	defaultManifestsPoolWorkers   = 5
	defaultHardRefreshPoolWorkers = 5
)

type IArgoCDWrapper interface {
	ListApplicationsByLabels(ctx context.Context, labels map[string]string, excludeLabels map[string]string) ([]ListApplicationsResult, error)
	GetManifests(ctx context.Context, applicationName string, liveRef string, targetRef string) (*GetManifestsResponse, error)
	GetUrl() string
	GetScheme() string
	// Close releases the underlying ArgoCD connection.
	Close() error
}

type ArgoCDWrapperOptions struct {
	ListPoolWorkers        int
	ManifestsPoolWorkers   int
	HardRefreshPoolWorkers int
	DoNotInstrumentWorkers bool
}

type ArgoCDWrapper struct {
	ArgoCDClientOptions *ArgoCDWrapperOptions
	ListWorkerPool      pond.ResultPool[[]ListApplicationsResult]
	ManifestsWorkerPool pond.ResultPool[[]string]
	HardRefreshPool     pond.Pool
	ApplicationClient   IArgoCDClient
}

type GetManifestsResponse struct {
	LiveManifests   []string
	TargetManifests []string
}

func New(client IArgoCDClient, argoCDName string, options *ArgoCDWrapperOptions) (IArgoCDWrapper, error) {
	if options.ListPoolWorkers == 0 {
		options.ListPoolWorkers = defaultListPoolWorkers
	}

	if options.ManifestsPoolWorkers == 0 {
		options.ManifestsPoolWorkers = defaultManifestsPoolWorkers
	}

	if options.HardRefreshPoolWorkers == 0 {
		options.HardRefreshPoolWorkers = defaultHardRefreshPoolWorkers
	}

	wrapper := ArgoCDWrapper{
		ArgoCDClientOptions: options,
	}

	wrapper.ListWorkerPool = pond.NewResultPool[[]ListApplicationsResult](options.ListPoolWorkers)
	wrapper.ManifestsWorkerPool = pond.NewResultPool[[]string](options.ManifestsPoolWorkers)
	wrapper.HardRefreshPool = pond.NewPool(options.HardRefreshPoolWorkers)

	if !options.DoNotInstrumentWorkers {
		instrumentResultPool("list", argoCDName, wrapper.ListWorkerPool)
		instrumentResultPool("diff", argoCDName, wrapper.ManifestsWorkerPool)
		instrumentPool("hard-refresh", argoCDName, wrapper.HardRefreshPool)
	}

	wrapper.ApplicationClient = client

	return &wrapper, nil
}

func (a *ArgoCDWrapper) ListApplicationsByLabels(ctx context.Context, labels map[string]string, excludeLabels map[string]string) ([]ListApplicationsResult, error) {
	group := a.ListWorkerPool.NewGroup()
	labelsSlice := []string{}
	for key, value := range labels {
		labelsSlice = append(labelsSlice, key+"="+value)
	}
	for key, value := range excludeLabels {
		labelsSlice = append(labelsSlice, key+"!="+value)
	}

	// Sorted so the selector string is deterministic: both maps are ranged
	// over above, and Go randomizes map iteration order.
	slices.Sort(labelsSlice)
	k8sLabel := strings.Join(labelsSlice, ",")

	group.SubmitErr(func() ([]ListApplicationsResult, error) {
		// Gated on the joined slice rather than on labels alone: an
		// exclude-only query still needs its "key!=value" selector sent, and
		// sending no selector at all is what ArgoCD reads as "list
		// everything" (#240). With both maps empty there is nothing to
		// filter on, so the query goes out without a selector.
		var query *application.ApplicationQuery
		if len(labelsSlice) > 0 {
			query = &application.ApplicationQuery{
				Selector: &k8sLabel,
			}
		} else {
			query = &application.ApplicationQuery{}
		}

		apps, err := a.ApplicationClient.List(ctx, query)
		if err != nil {
			return nil, err
		}

		results := []ListApplicationsResult{}
		for _, app := range apps.Items {
			project := ""
			if len(app.Spec.Project) > 0 {
				project = app.Spec.Project
			} else {
				// Empty project == default
				project = "default"
			}

			var liveRevision string
			if app.Spec.Source != nil {
				liveRevision = app.Spec.Source.TargetRevision
			} else {
				liveRevision = ""
			}

			results = append(results, ListApplicationsResult{
				Name:         app.Name,
				Project:      project,
				Namespace:    app.Namespace,
				Health:       app.Status.Health,
				SyncStatus:   app.Status.Sync,
				LiveRevision: liveRevision,
			})
		}

		return results, nil
	})

	poolResults, err := group.Wait()
	if err != nil {
		return nil, err
	}

	apps := []ListApplicationsResult{}
	for _, result := range poolResults {
		apps = append(apps, result...)
	}

	return apps, nil
}

func (a *ArgoCDWrapper) GetManifests(ctx context.Context, applicationName string, liveRef string, targetRef string) (*GetManifestsResponse, error) {
	refreshGroup := a.HardRefreshPool.NewGroup()
	refreshGroup.SubmitErr(func() error {
		// This is the one call Tangle makes that isn't a pure read: ArgoCD
		// patches the application's refresh annotation and forces an uncached
		// repo-server comparison. ArgoCDClient retries a lost connection once,
		// so a connection dropped after ArgoCD had already started the work
		// costs one extra hard refresh. Repeating it is safe — the second one
		// supersedes the first — and the alternative, excluding this call from
		// the retry, means the diffs page fails instead of recovering. The
		// pool this runs on is what bounds the cost.
		refresh := "hard"
		_, err := a.ApplicationClient.Get(ctx, &application.ApplicationQuery{Name: &applicationName, Refresh: &refresh})
		return err
	})
	err := refreshGroup.Wait()
	if err != nil {
		return nil, err
	}

	group := a.ManifestsWorkerPool.NewGroup()
	for _, ref := range []string{liveRef, targetRef} {
		group.SubmitErr(func() ([]string, error) {
			manifestsQuery := &application.ApplicationManifestQuery{
				Name:     &applicationName,
				Revision: &ref,
			}

			manifestsResp, err := a.ApplicationClient.GetApplicationManifests(ctx, manifestsQuery)
			if err != nil {
				return nil, err
			}

			return manifestsResp.Manifests, nil
		})
	}

	poolResults, err := group.Wait()
	if err != nil {
		return nil, err
	}

	response := GetManifestsResponse{LiveManifests: poolResults[0], TargetManifests: poolResults[1]}

	return &response, nil
}

func (a *ArgoCDWrapper) GetUrl() string {
	return a.ApplicationClient.GetUrl()
}

func (a *ArgoCDWrapper) GetScheme() string {
	return a.ApplicationClient.GetScheme()
}

func (a *ArgoCDWrapper) Close() error {
	return a.ApplicationClient.Close()
}
