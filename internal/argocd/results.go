package argocd

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type ListApplicationsResult struct {
	Name         string
	Project      string
	Namespace    string
	Health       v1alpha1.AppHealthStatus
	SyncStatus   v1alpha1.SyncStatus
	LiveRevision string
}
