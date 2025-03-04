package argocd

import (
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type ListApplicationsResult struct {
	Name         string
	Project      string
	Namespace    string
	Health       v1alpha1.HealthStatus
	SyncStatus   v1alpha1.SyncStatus
	LiveRevision string
}
