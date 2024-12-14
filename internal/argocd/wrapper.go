package argocd

import (
	"fmt"
	"os"

	"github.com/alitto/pond/v2"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
)

const (
	defaultListPoolWorkers = 10
	defaultDiffPoolWorkers = 5
)

type IArgoCDWrapper interface {
	ListApplicationsByLabels(labels map[string]string) []string
}

type ArgoCDWrapperOptions struct {
	Address                string
	Insecure               bool
	AuthTokenEnvVar        string
	ListPoolWorkers        int
	DiffPoolWokers         int
	DoNotInstrumentWorkers bool
}

type ArgoCDWrapper struct {
	ArgoCDClientOptions *ArgoCDWrapperOptions
	ListWorkerPool      pond.ResultPool[string]
	DiffWorkerPool      pond.ResultPool[string]
	ApplicationClient   application.ApplicationServiceClient
}

func New(client *ArgoCDClient, options *ArgoCDWrapperOptions) (IArgoCDWrapper, error) {
	if options.ListPoolWorkers == 0 {
		options.ListPoolWorkers = defaultListPoolWorkers
	}

	if options.DiffPoolWokers == 0 {
		options.DiffPoolWokers = defaultDiffPoolWorkers
	}

	wrapper := ArgoCDWrapper{
		ArgoCDClientOptions: options,
	}

	wrapper.ListWorkerPool = pond.NewResultPool[string](options.ListPoolWorkers)
	wrapper.DiffWorkerPool = pond.NewResultPool[string](options.DiffPoolWokers)

	if !options.DoNotInstrumentWorkers {
		instrumentWorkers("list", wrapper.ListWorkerPool)
		instrumentWorkers("diff", wrapper.DiffWorkerPool)
	}

	authToken, found := os.LookupEnv(options.AuthTokenEnvVar)
	if !found {
		return nil, fmt.Errorf("%s not found", options.AuthTokenEnvVar)
	}

	apiClient, err := apiclient.NewClient(&apiclient.ClientOptions{
		ServerAddr: options.Address,
		Insecure:   options.Insecure,
		AuthToken:  authToken,
		GRPCWeb:    true,
	})
	if err != nil {
		return nil, err
	}

	_, applicationClient, err := apiClient.NewApplicationClient()
	if err != nil {
		return nil, err
	}

	wrapper.ApplicationClient = applicationClient

	return &wrapper, nil
}

func (a *ArgoCDWrapper) ListApplicationsByLabels(labels map[string]string) []string {
	group := a.ListWorkerPool.NewGroup()

	for key := range labels {
		value := labels[key]
		group.SubmitErr(func() (string, error) {
			return fmt.Sprintf("Value %s", value), nil
		})
	}

	results, _ := group.Wait()
	return results
}
