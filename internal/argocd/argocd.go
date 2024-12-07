package argocd

import (
	"fmt"

	"github.com/alitto/pond/v2"
)

const (
	defaultListPoolWorkers = 10
	defaultDiffPoolWorkers = 5
)

type IArgoCDClient interface{}

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
}

func New(options *ArgoCDClientOptions) ArgoCDClient {
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

	return client
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
