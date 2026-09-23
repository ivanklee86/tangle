# Use Cases

This contains a list of important use cases.

Each scenario runs as an e2e test against the live stack (`task services:cicd`). The CLI steps are in [`cmd/tangle-cli/main_e2e_test.go`](../../cmd/tangle-cli/main_e2e_test.go) and the "User clicks on it" steps are in [`web/e2e/live/ci-scenarios.spec.ts`](../../web/e2e/live/ci-scenarios.spec.ts). "The change the user pushed" is the `test_gitops` branch, which is a test fixture: see [Live test fixtures](internals.md#live-test-fixtures).

## CI

Tests: `TestE2E_CIScenario` and `CI scenario (live)`, filtering on `bazz:buzz` (`test-1` changed, `test-2` unchanged).

```gherkin
Scenario: User has pushed a change to manifests.

  When CI renders Application URL and User clicks on it.
  Then the User should be able to see high-level application state in UI.

  When CI renders Diffs URL and User clicks on it.
  Then the User should be able to see high-level application state in UI.

  When CI runs `tangle-cli generate-manifests`.
  Then manifests should be present on disk and they should match manifests from ArgoCD.
  And the diffs should be present on disk and should match changes made.
```

## CI with failure

Tests: `TestE2E_CIFailureScenario` and `CI with failure scenario (live)`, filtering on `foo:bar` (adds `test-3`, whose manifests fail to generate, and the unchanged `test-4`).

```gherkin
Scenario: User has pushed a change to manifests.

  When CI renders Application URL and User clicks on it.
  Then the User should be able to see high-level application state in UI.

  When CI renders Diffs URL and User clicks on it.
  Then the User should be able to see high-level application state in UI.

  When CI runs `tangle-cli generate-manifests --fail-on-error`.
  Then manifests should be present on disk and they should match manifests from ArgoCD.
  And the diffs should be present on disk and should match changes made.
  And CLI exits with error code.
```
