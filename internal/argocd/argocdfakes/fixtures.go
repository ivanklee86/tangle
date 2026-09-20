package argocdfakes

import "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

// ExampleApplications reproduces the four Applications checked in at
// integration/kubernetes/example/application-*.yaml, as v1alpha1.Application
// values suitable for FakeClient. Kept in one place so workstream 2's
// rewritten tests stay aligned with the fixture the live-cluster tests were
// originally written against: test-1/test-2 in the "default" project (what
// the ARGOCD_TOKEN identity's RBAC scopes it to today), test-3/test-4 in
// "my-project" (what ARGOCD_PROD_TOKEN scopes it to).
func ExampleApplications() []v1alpha1.Application {
	return []v1alpha1.Application{
		exampleApplication("test-1", "default", map[string]string{"env": "test", "foo": "bar", "bazz": "buzz"}),
		exampleApplication("test-2", "default", map[string]string{"env": "preprod", "foo": "bar", "bazz": "buzz"}),
		exampleApplication("test-3", "my-project", map[string]string{"env": "prod", "foo": "bar"}),
		exampleApplication("test-4", "my-project", map[string]string{"env": "infra", "foo": "bar"}),
	}
}

func exampleApplication(name string, project string, labels map[string]string) v1alpha1.Application {
	app := v1alpha1.Application{}
	app.Name = name
	app.Namespace = "argocd"
	app.Labels = labels
	app.Spec.Project = project
	app.Spec.Source = &v1alpha1.ApplicationSource{TargetRevision: "main"}

	return app
}
