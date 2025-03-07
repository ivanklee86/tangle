package argocd

import (
	"context"
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	repoServerApiClient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

// ArgoCDClient defines the interface for interacting with ArgoCD
type IArgoCDClient interface {
	// List returns all ArgoCD applications
	List(ctx context.Context, in *application.ApplicationQuery) (*v1alpha1.ApplicationList, error)
	GetApplicationManifests(ctx context.Context, in *application.ApplicationManifestQuery) (*repoServerApiClient.ManifestResponse, error)
	Get(ctx context.Context, in *application.ApplicationQuery) (*v1alpha1.Application, error)
	GetUrl() string
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

func (c *ArgoCDClient) GetApplicationManifests(ctx context.Context, query *application.ApplicationManifestQuery) (*repoServerApiClient.ManifestResponse, error) {
	manifests, err := c.applicationsClient.GetManifests(ctx, query)
	if err != nil {
		return nil, err
	}

	return manifests, nil
}

func (c *ArgoCDClient) Get(ctx context.Context, query *application.ApplicationQuery) (*v1alpha1.Application, error) {
	app, err := c.applicationsClient.Get(ctx, query)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (c *ArgoCDClient) GetUrl() string {
	return c.Options.Address
}
