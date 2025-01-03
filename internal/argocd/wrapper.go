package argocd

import (
	"context"

	"github.com/alitto/pond/v2"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

const (
	defaultListPoolWorkers = 10
	defaultDiffPoolWorkers = 5
)

type IArgoCDWrapper interface {
	ListApplicationsByLabels(ctx context.Context, labels map[string]string) ([]ListApplicationsResult, error)
	GetUrl() string
}

type ArgoCDWrapperOptions struct {
	ListPoolWorkers        int
	DiffPoolWokers         int
	DoNotInstrumentWorkers bool
}

type ArgoCDWrapper struct {
	ArgoCDClientOptions *ArgoCDWrapperOptions
	ListWorkerPool      pond.ResultPool[[]ListApplicationsResult]
	DiffWorkerPool      pond.ResultPool[[]v1alpha1.Application]
	ApplicationClient   IArgoCDClient
}

func New(client IArgoCDClient, argoCDName string, options *ArgoCDWrapperOptions) (IArgoCDWrapper, error) {
	if options.ListPoolWorkers == 0 {
		options.ListPoolWorkers = defaultListPoolWorkers
	}

	if options.DiffPoolWokers == 0 {
		options.DiffPoolWokers = defaultDiffPoolWorkers
	}

	wrapper := ArgoCDWrapper{
		ArgoCDClientOptions: options,
	}

	wrapper.ListWorkerPool = pond.NewResultPool[[]ListApplicationsResult](options.ListPoolWorkers)
	wrapper.DiffWorkerPool = pond.NewResultPool[[]v1alpha1.Application](options.DiffPoolWokers)

	if !options.DoNotInstrumentWorkers {
		instrumentWorkers("list", argoCDName, wrapper.ListWorkerPool)
		instrumentWorkers("diff", argoCDName, wrapper.DiffWorkerPool)
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
				Name:      app.Name,
				Project:   project,
				Namespace: app.Namespace,
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

func (a *ArgoCDWrapper) GetUrl() string {
	return a.ApplicationClient.GetUrl()
}
