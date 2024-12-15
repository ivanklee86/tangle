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
	ListApplicationsByLabels(labels map[string]string) []string
}

type ArgoCDWrapperOptions struct {
	ListPoolWorkers        int
	DiffPoolWokers         int
	DoNotInstrumentWorkers bool
}

type ArgoCDWrapper struct {
	ArgoCDClientOptions *ArgoCDWrapperOptions
	ListWorkerPool      pond.ResultPool[[]v1alpha1.Application]
	DiffWorkerPool      pond.ResultPool[[]v1alpha1.Application]
	ApplicationClient   IArgoCDClient
}

func New(client *IArgoCDClient, options *ArgoCDWrapperOptions) (IArgoCDWrapper, error) {
	if options.ListPoolWorkers == 0 {
		options.ListPoolWorkers = defaultListPoolWorkers
	}

	if options.DiffPoolWokers == 0 {
		options.DiffPoolWokers = defaultDiffPoolWorkers
	}

	wrapper := ArgoCDWrapper{
		ArgoCDClientOptions: options,
	}

	wrapper.ListWorkerPool = pond.NewResultPool[[]v1alpha1.Application](options.ListPoolWorkers)
	wrapper.DiffWorkerPool = pond.NewResultPool[[]v1alpha1.Application](options.DiffPoolWokers)

	if !options.DoNotInstrumentWorkers {
		instrumentWorkers("list", wrapper.ListWorkerPool)
		instrumentWorkers("diff", wrapper.DiffWorkerPool)
	}

	wrapper.ApplicationClient = *client

	return &wrapper, nil
}

func (a *ArgoCDWrapper) ListApplicationsByLabels(labels map[string]string) []string {
	group := a.ListWorkerPool.NewGroup()
	k8sLabel := ""
	for key, value := range labels {
		k8sLabel = k8sLabel + key + "=" + value + ","
	}

	group.SubmitErr(func() ([]v1alpha1.Application, error) {
		apps, err := a.ApplicationClient.List(context.Background(), &application.ApplicationQuery{
			Selector: &k8sLabel,
		})
		if err != nil {
			return nil, err
		}

		return apps.Items, nil
	})

	results, _ := group.Wait()

	appNames := ""
	for _, result := range results[0] {
		appNames = appNames + result.Name + ","
	}

	return appNames
}
