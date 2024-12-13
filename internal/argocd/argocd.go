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

type IArgoCDClient interface {
	ListApplicationsByLabels(labels map[string]string) []string
}

type ArgoCDClientOptions struct {
	Address                string
	Insecure               bool
	AuthTokenEnvVar        string
	ListPoolWorkers        int
	DiffPoolWokers         int
	DoNotInstrumentWorkers bool
}

type ArgoCDClient struct {
	ArgoCDClientOptions *ArgoCDClientOptions
	ListWorkerPool      pond.ResultPool[string]
	DiffWorkerPool      pond.ResultPool[string]
	ApplicationClient   application.ApplicationServiceClient
}

func New(options *ArgoCDClientOptions) (IArgoCDClient, error) {
	if options.ListPoolWorkers == 0 {
		options.ListPoolWorkers = defaultListPoolWorkers
	}

	if options.DiffPoolWokers == 0 {
		options.DiffPoolWokers = defaultDiffPoolWorkers
	}

	client := ArgoCDClient{
		ArgoCDClientOptions: options,
	}

	client.ListWorkerPool = pond.NewResultPool[string](options.ListPoolWorkers)
	client.DiffWorkerPool = pond.NewResultPool[string](options.DiffPoolWokers)

	if !options.DoNotInstrumentWorkers {
		instrumentWorkers("list", client.ListWorkerPool)
		instrumentWorkers("diff", client.DiffWorkerPool)
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

	client.ApplicationClient = applicationClient

	return &client, nil
}

func (a *ArgoCDClient) ListApplicationsByLabels(labels map[string]string) []string {
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
