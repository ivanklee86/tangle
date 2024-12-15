package argocd

// import (
// 	"context"

// 	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
// 	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
// )

// type mockArgoCDClient struct {
// 	listRes *v1alpha1.ApplicationList
// 	listErr error
// }

// func newMockArgoCDClient(listRes *v1alpha1.ApplicationList, listErr error) IArgoCDClient {
// 	return &mockArgoCDClient{
// 		listRes: listRes,
// 		listErr: listErr,
// 	}
// }

// func (m *mockArgoCDClient) List(ctx context.Context, in *application.ApplicationQuery) (*v1alpha1.ApplicationList, error) {
// 	return m.listRes, m.listErr
// }
