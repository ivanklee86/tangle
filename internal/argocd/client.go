package argocd

import (
	"context"
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// ArgoCDClient defines the interface for interacting with ArgoCD
type IArgoCDClient interface {
	// List returns all ArgoCD applications
	List(ctx context.Context, in *application.ApplicationQuery) (*v1alpha1.ApplicationList, error)
}

type ArgoCDClientOptions struct {
	Address         string
	Insecure        bool
	AuthTokenEnvVar string
}

type ArgoCDClient struct {
	Options *ArgoCDClientOptions

	applicationsClient application.ApplicationServiceClient
}

func NewArgoCDClient(options *ArgoCDClientOptions) (IArgoCDClient, error) {
	client := ArgoCDClient{
		Options: options,
	}

	authToken, found := os.LookupEnv(options.AuthTokenEnvVar)
	if !found {
		return nil, fmt.Errorf("auth token not found")
	}

	argocdClient := apiclient.NewClientOrDie(&apiclient.ClientOptions{
		ServerAddr: client.Options.Address,
		Insecure:   client.Options.Insecure,
		AuthToken:  authToken,
		GRPCWeb:    true,
	})

	_, client.applicationsClient = argocdClient.NewApplicationClientOrDie()

	return &client, nil
}

func (c *ArgoCDClient) List(ctx context.Context, query *application.ApplicationQuery) (*v1alpha1.ApplicationList, error) {
	appList, err := c.applicationsClient.List(ctx, query)
	if err != nil {
		return nil, err
	}

	return appList, nil
}
