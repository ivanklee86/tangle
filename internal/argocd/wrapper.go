package argocd

import (
	"context"

	"github.com/alitto/pond/v2"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
)

const (
	defaultListPoolWorkers      = 10
	defaultManifestsPoolWorkers = 5
)

type IArgoCDWrapper interface {
	ListApplicationsByLabels(ctx context.Context, labels map[string]string) ([]ListApplicationsResult, error)
	GetManifests(ctx context.Context, applicationName string, currentRef string, compareRef string) (*GetManifestsResponse, error)
	GetUrl() string
}

type ArgoCDWrapperOptions struct {
	ListPoolWorkers        int
	ManifestsPoolWorkers   int
	DoNotInstrumentWorkers bool
}

type ArgoCDWrapper struct {
	ArgoCDClientOptions *ArgoCDWrapperOptions
	ListWorkerPool      pond.ResultPool[[]ListApplicationsResult]
	ManifestsWorkerPool pond.ResultPool[[]string]
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

	wrapper := ArgoCDWrapper{
		ArgoCDClientOptions: options,
	}

	wrapper.ListWorkerPool = pond.NewResultPool[[]ListApplicationsResult](options.ListPoolWorkers)
	wrapper.ManifestsWorkerPool = pond.NewResultPool[[]string](options.ManifestsPoolWorkers)

	if !options.DoNotInstrumentWorkers {
		instrumentWorkers("list", argoCDName, wrapper.ListWorkerPool)
		instrumentWorkers("diff", argoCDName, wrapper.ManifestsWorkerPool)
	}

	wrapper.ApplicationClient = client

	return &wrapper, nil
}

func (a *ArgoCDWrapper) ListApplicationsByLabels(ctx context.Context, labels map[string]string) ([]ListApplicationsResult, error) {
	group := a.ListWorkerPool.NewGroup()
	k8sLabel := ""
	for key, value := range labels {
		if len(k8sLabel) == 0 {
			k8sLabel = k8sLabel + key + "=" + value
		} else {
			k8sLabel = k8sLabel + key + "=" + value + ","
		}
	}

	group.SubmitErr(func() ([]ListApplicationsResult, error) {
		var query *application.ApplicationQuery
		if len(k8sLabel) > 0 {
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

			results = append(results, ListApplicationsResult{
				Name:           app.Name,
				Project:        project,
				Namespace:      app.Namespace,
				Health:         app.Status.Health,
				SyncStatus:     app.Status.Sync,
				TargetRevision: app.Spec.Source.TargetRevision,
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
