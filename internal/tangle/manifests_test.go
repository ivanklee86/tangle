package tangle

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManifests(t *testing.T) {
	manifestsCurrent := []string{
		"{\"apiVersion\":\"v1\",\"automountServiceAccountToken\":true,\"kind\":\"ServiceAccount\",\"metadata\":{\"labels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/managed-by\":\"Helm\",\"app.kubernetes.io/name\":\"test\",\"app.kubernetes.io/version\":\"1.16.0\",\"argocd.argoproj.io/instance\":\"test-1\",\"helm.sh/chart\":\"test-0.1.0\"},\"name\":\"test-1\"}}",
		"{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"labels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/managed-by\":\"Helm\",\"app.kubernetes.io/name\":\"test\",\"app.kubernetes.io/version\":\"1.16.0\",\"argocd.argoproj.io/instance\":\"test-1\",\"helm.sh/chart\":\"test-0.1.0\"},\"name\":\"test-1\"},\"spec\":{\"ports\":[{\"name\":\"http\",\"port\":80,\"protocol\":\"TCP\",\"targetPort\":\"http\"}],\"selector\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/name\":\"test\"},\"type\":\"ClusterIP\"}}",
		"{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"labels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/managed-by\":\"Helm\",\"app.kubernetes.io/name\":\"test\",\"app.kubernetes.io/version\":\"1.16.0\",\"argocd.argoproj.io/instance\":\"test-1\",\"helm.sh/chart\":\"test-0.1.0\"},\"name\":\"test-1\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/name\":\"test\"}},\"template\":{\"metadata\":{\"labels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/managed-by\":\"Helm\",\"app.kubernetes.io/name\":\"test\",\"app.kubernetes.io/version\":\"1.16.0\",\"helm.sh/chart\":\"test-0.1.0\"}},\"spec\":{\"containers\":[{\"image\":\"nginx:1.16.0\",\"imagePullPolicy\":\"IfNotPresent\",\"livenessProbe\":{\"httpGet\":{\"path\":\"/\",\"port\":\"http\"}},\"name\":\"test\",\"ports\":[{\"containerPort\":80,\"name\":\"http\",\"protocol\":\"TCP\"}],\"readinessProbe\":{\"httpGet\":{\"path\":\"/\",\"port\":\"http\"}},\"resources\":{},\"securityContext\":{}}],\"securityContext\":{},\"serviceAccountName\":\"test-1\"}}}}",
		"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{\"helm.sh/hook\":\"test\"},\"labels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/managed-by\":\"Helm\",\"app.kubernetes.io/name\":\"test\",\"app.kubernetes.io/version\":\"1.16.0\",\"argocd.argoproj.io/instance\":\"test-1\",\"helm.sh/chart\":\"test-0.1.0\"},\"name\":\"test-1-test-connection\"},\"spec\":{\"containers\":[{\"args\":[\"test-1:80\"],\"command\":[\"wget\"],\"image\":\"busybox\",\"name\":\"wget\"}],\"restartPolicy\":\"Never\"}}",
	}

	// manifestsCompare := []string{
	// 	"{\"apiVersion\":\"v1\",\"automountServiceAccountToken\":true,\"kind\":\"ServiceAccount\",\"metadata\":{\"labels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/managed-by\":\"Helm\",\"app.kubernetes.io/name\":\"test\",\"app.kubernetes.io/version\":\"1.16.0\",\"argocd.argoproj.io/instance\":\"test-1\",\"helm.sh/chart\":\"test-0.1.0\"},\"name\":\"test-1\"}}",
	// 	"{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"labels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/managed-by\":\"Helm\",\"app.kubernetes.io/name\":\"test\",\"app.kubernetes.io/version\":\"1.16.0\",\"argocd.argoproj.io/instance\":\"test-1\",\"helm.sh/chart\":\"test-0.1.0\"},\"name\":\"test-1\"},\"spec\":{\"ports\":[{\"name\":\"http\",\"port\":80,\"protocol\":\"TCP\",\"targetPort\":\"http\"}],\"selector\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/name\":\"test\"},\"type\":\"ClusterIP\"}}",
	// 	"{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"labels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/managed-by\":\"Helm\",\"app.kubernetes.io/name\":\"test\",\"app.kubernetes.io/version\":\"1.16.0\",\"argocd.argoproj.io/instance\":\"test-1\",\"helm.sh/chart\":\"test-0.1.0\"},\"name\":\"test-1\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/name\":\"test\"}},\"template\":{\"metadata\":{\"labels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/managed-by\":\"Helm\",\"app.kubernetes.io/name\":\"test\",\"app.kubernetes.io/version\":\"1.16.0\",\"helm.sh/chart\":\"test-0.1.0\"}},\"spec\":{\"containers\":[{\"image\":\"nginx:1.16.0\",\"imagePullPolicy\":\"IfNotPresent\",\"livenessProbe\":{\"httpGet\":{\"path\":\"/\",\"port\":\"http\"}},\"name\":\"test\",\"ports\":[{\"containerPort\":80,\"name\":\"http\",\"protocol\":\"TCP\"}],\"readinessProbe\":{\"httpGet\":{\"path\":\"/\",\"port\":\"http\"}},\"resources\":{},\"securityContext\":{}}],\"securityContext\":{},\"serviceAccountName\":\"test-1\"}}}}",
	// 	"{\"apiVersion\":\"networking.k8s.io/v1\",\"kind\":\"Ingress\",\"metadata\":{\"labels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/managed-by\":\"Helm\",\"app.kubernetes.io/name\":\"test\",\"app.kubernetes.io/version\":\"1.16.0\",\"argocd.argoproj.io/instance\":\"test-1\",\"helm.sh/chart\":\"test-0.1.0\"},\"name\":\"test-1\"},\"spec\":{\"rules\":[{\"host\":\"chart-example.local\",\"http\":{\"paths\":[{\"backend\":{\"service\":{\"name\":\"test-1\",\"port\":{\"number\":80}}},\"path\":\"/\",\"pathType\":\"ImplementationSpecific\"}]}}]}}",
	// 	"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{\"helm.sh/hook\":\"test\"},\"labels\":{\"app.kubernetes.io/instance\":\"test-1\",\"app.kubernetes.io/managed-by\":\"Helm\",\"app.kubernetes.io/name\":\"test\",\"app.kubernetes.io/version\":\"1.16.0\",\"argocd.argoproj.io/instance\":\"test-1\",\"helm.sh/chart\":\"test-0.1.0\"},\"name\":\"test-1-test-connection\"},\"spec\":{\"containers\":[{\"args\":[\"test-1:80\"],\"command\":[\"wget\"],\"image\":\"busybox\",\"name\":\"wget\"}],\"restartPolicy\":\"Never\"}}",
	// }

	t.Run("Test stiching yamls", func(t *testing.T) {
		manifests, err := assembleManifests(manifestsCurrent)

		assert.Nil(t, err)
		assert.NotNil(t, manifests)

		fmt.Print(&manifests)
	})
}
