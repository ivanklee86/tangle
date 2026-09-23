//go:build e2e

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// These tests are the CLI half of the scenarios in docs/agents/test_cases.md
// ("CI" and "CI with failure"). They run the real tangle-cli binary against
// the live stack `task services:cicd` brings up: tangle-server on :8081,
// ArgoCD on :8080.
//
// The "change the user pushed" is the test_gitops branch on origin, compared
// against main on GitHub (not the local checkout):
//
//   - test-1 (test ArgoCD): ingress.enabled flips to true, so the diff adds
//     one Ingress.
//   - test-2 (test), test-4 (prod): unchanged.
//   - test-3 (prod): values.yaml is deliberately invalid, so manifest
//     generation fails in helm template.
//
// Rebasing or "fixing" test_gitops breaks these tests.
//
// Every top-level test is prefixed TestE2E_ so tasks/go.yaml's
// `-run '^TestE2E_'` selects it.

const (
	e2eServerAddress = "localhost:8081"
	e2eArgoCDAddress = "localhost:8080"
	e2eTargetRef     = "test_gitops"
)

// Maps each ArgoCD in integration/tangle.yaml to the .env variable holding
// its token.
var e2eArgoCDTokenEnvVars = map[string]string{
	"test": "ARGOCD_TOKEN",
	"prod": "ARGOCD_PROD_TOKEN",
}

var (
	buildOnce  sync.Once
	binaryPath string
	buildErr   error
)

// buildCLI compiles the real binary once per test run. The CLI has to run
// out of process: TangleCLI.Error calls os.Exit, which would kill the test
// binary, and the exit code is part of what the scenarios promise.
func buildCLI(t *testing.T) string {
	t.Helper()

	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "tangle-cli-e2e")
		if err != nil {
			buildErr = err
			return
		}
		binaryPath = filepath.Join(dir, "tangle-cli")
		output, err := exec.Command("go", "build", "-o", binaryPath, ".").CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("go build: %w\n%s", err, output)
		}
	})
	require.NoError(t, buildErr)

	return binaryPath
}

type cliRun struct {
	Folder   string
	Stdout   string
	Stderr   string
	ExitCode int
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// runCLI runs tangle-cli in a fresh folder, which is both its working
// directory and its --folder, and returns what CI would observe.
func runCLI(t *testing.T, args ...string) cliRun {
	t.Helper()

	folder := t.TempDir()
	command := exec.Command(buildCLI(t), args...)
	command.Dir = folder

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	exitCode := 0
	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		require.True(t, errors.As(err, &exitErr), "running tangle-cli: %v", err)
		exitCode = exitErr.ExitCode()
	}

	run := cliRun{
		Folder:   folder,
		Stdout:   ansiEscape.ReplaceAllString(stdout.String(), ""),
		Stderr:   ansiEscape.ReplaceAllString(stderr.String(), ""),
		ExitCode: exitCode,
	}
	t.Logf("tangle-cli %s\nexit: %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), run.ExitCode, run.Stdout, run.Stderr)

	return run
}

// generateManifests runs the invocation the scenarios describe: CI points
// the CLI at the server and the pushed ref, and filters by labels. It leaves
// out --folder, so output lands in the working directory, as its help text
// promises.
func generateManifests(t *testing.T, labels []string, extraArgs ...string) cliRun {
	t.Helper()

	args := []string{
		"generate-manifests",
		"--server-address", e2eServerAddress,
		"--insecure",
		"--target-ref", e2eTargetRef,
		"--retries", "2",
	}
	for _, label := range labels {
		args = append(args, "--label", label)
	}

	return runCLI(t, append(args, extraArgs...)...)
}

func (r cliRun) readFile(t *testing.T, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(r.Folder, name))
	require.NoError(t, err, "expected %s on disk", name)

	return string(content)
}

func (r cliRun) files(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(r.Folder)
	require.NoError(t, err)

	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names
}

// assertTableRow checks the CLI's summary table reports an application's
// manifest-generation outcome.
func assertTableRow(t *testing.T, stdout, argocd, application string, failed bool) {
	t.Helper()

	status := `False ✅`
	if failed {
		status = `True 🔥`
	}
	row := regexp.MustCompile(fmt.Sprintf(`(?m)^\s*%s\s+%s\s+%s\s*$`, argocd, application, status))
	assert.Regexp(t, row, stdout)
}

// argocdManifests asks ArgoCD itself, through the argocd CLI rather than
// Tangle, for an application's manifests at the target ref. It's the
// independent oracle for "manifests should match manifests from ArgoCD".
func argocdManifests(t *testing.T, argocd, application string) (string, error) {
	t.Helper()

	command := exec.Command("argocd", "app", "manifests", application,
		"--revision", e2eTargetRef,
		"--server", e2eArgoCDAddress,
		"--plaintext",
		"--auth-token", os.Getenv(e2eArgoCDTokenEnvVars[argocd]),
	)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

var yamlDocumentSeparator = regexp.MustCompile(`(?m)^---\s*$`)

// manifestSet parses a multi-document YAML stream into its resources, keyed
// by apiVersion/kind/namespace/name, so two renders can be compared without
// depending on document order or formatting.
func manifestSet(t *testing.T, manifests string) map[string]map[string]any {
	t.Helper()

	resources := map[string]map[string]any{}
	for _, document := range yamlDocumentSeparator.Split(manifests, -1) {
		if strings.TrimSpace(document) == "" {
			continue
		}

		resource := map[string]any{}
		require.NoError(t, yaml.Unmarshal([]byte(document), &resource))

		metadata, _ := resource["metadata"].(map[string]any)
		key := fmt.Sprintf("%v/%v/%v/%v", resource["apiVersion"], resource["kind"], metadata["namespace"], metadata["name"])
		require.NotContains(t, resources, key, "duplicate resource in manifests")
		resources[key] = resource
	}

	return resources
}

// assertManifestsMatchArgoCD is the "manifests should match manifests from
// ArgoCD" step for one application that renders successfully.
func assertManifestsMatchArgoCD(t *testing.T, run cliRun, argocd, application string) {
	t.Helper()

	written := run.readFile(t, fmt.Sprintf("manifests-%s-%s.yaml", argocd, application))
	require.NotEmpty(t, written)

	expected, err := argocdManifests(t, argocd, application)
	require.NoError(t, err)

	assert.Equal(t, manifestSet(t, expected), manifestSet(t, written))
}

var addedIngress = regexp.MustCompile(`(?m)^\+\s+kind: Ingress$`)

// assertDiffAddsOnlyIngress is the "diffs should match changes made" step for
// test-1: test_gitops enables its ingress and changes nothing else.
func assertDiffAddsOnlyIngress(t *testing.T, run cliRun) {
	t.Helper()

	diff := run.readFile(t, "diff-test-test-1.yaml")
	assert.Regexp(t, addedIngress, diff)

	for line := range strings.SplitSeq(diff, "\n") {
		// "--- <file>" is the unified diff header, not a removal.
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			t.Errorf("diff removes a line, but test_gitops only adds an Ingress: %q", line)
		}
	}
}

func setupE2E(t *testing.T) {
	t.Helper()

	require.NoError(t, godotenv.Load("../../.env"))
}

// Scenario "CI": the pushed change renders cleanly. bazz=buzz selects test-1
// (changed) and test-2 (unchanged), both on the test ArgoCD.
func TestE2E_CIScenario(t *testing.T) {
	setupE2E(t)

	run := generateManifests(t, []string{"bazz=buzz"})

	t.Run("cli_succeeds_and_reports_every_application", func(t *testing.T) {
		assert.Equal(t, 0, run.ExitCode)
		assert.Contains(t, run.Stdout, "Applications found: 2")
		assertTableRow(t, run.Stdout, "test", "test-1", false)
		assertTableRow(t, run.Stdout, "test", "test-2", false)
		assert.Empty(t, run.Stderr)
	})

	t.Run("cli_writes_manifests_matching_argocd", func(t *testing.T) {
		assertManifestsMatchArgoCD(t, run, "test", "test-1")
		assertManifestsMatchArgoCD(t, run, "test", "test-2")
	})

	t.Run("cli_writes_diffs_matching_change", func(t *testing.T) {
		assertDiffAddsOnlyIngress(t, run)
		assert.Empty(t, run.readFile(t, "diff-test-test-2.yaml"))
		assert.ElementsMatch(t, []string{
			"diff-test-test-1.yaml", "manifests-test-test-1.yaml",
			"diff-test-test-2.yaml", "manifests-test-test-2.yaml",
		}, run.files(t), "no error-*.txt when every application renders")
	})

	// --fail-on-error only fires when something failed.
	t.Run("fail_on_error_passes_when_nothing_failed", func(t *testing.T) {
		strict := generateManifests(t, []string{"bazz=buzz"}, "--fail-on-error")
		assert.Equal(t, 0, strict.ExitCode)
	})
}

// Scenario "CI with failure": the pushed change breaks one application.
// foo=bar selects all four; test-3's values.yaml is invalid on test_gitops.
func TestE2E_CIFailureScenario(t *testing.T) {
	setupE2E(t)

	run := generateManifests(t, []string{"foo=bar"}, "--fail-on-error")

	t.Run("cli_exits_with_error_code", func(t *testing.T) {
		assert.Equal(t, 1, run.ExitCode)
		assert.Contains(t, run.Stderr, "Failures found in manifest generation!")
	})

	t.Run("cli_reports_which_application_failed", func(t *testing.T) {
		assert.Contains(t, run.Stdout, "Applications found: 4")
		assertTableRow(t, run.Stdout, "test", "test-1", false)
		assertTableRow(t, run.Stdout, "test", "test-2", false)
		assertTableRow(t, run.Stdout, "prod", "test-3", true)
		assertTableRow(t, run.Stdout, "prod", "test-4", false)
	})

	t.Run("cli_writes_manifests_matching_argocd", func(t *testing.T) {
		assertManifestsMatchArgoCD(t, run, "test", "test-1")
		assertManifestsMatchArgoCD(t, run, "test", "test-2")
		assertManifestsMatchArgoCD(t, run, "prod", "test-4")

		// ArgoCD can't render test-3 either, and both report the same
		// helm failure. Match on "helm template" only: ArgoCD may prefix
		// a cached error with "(cached)".
		assert.Empty(t, run.readFile(t, "manifests-prod-test-3.yaml"))
		_, err := argocdManifests(t, "prod", "test-3")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "helm template")
		assert.Contains(t, run.readFile(t, "error-prod-test-3.txt"), "helm template")
	})

	t.Run("cli_writes_diffs_matching_change", func(t *testing.T) {
		assertDiffAddsOnlyIngress(t, run)
		assert.Empty(t, run.readFile(t, "diff-test-test-2.yaml"))
		assert.Empty(t, run.readFile(t, "diff-prod-test-4.yaml"))
		assert.Empty(t, run.readFile(t, "diff-prod-test-3.yaml"))
	})

	// --fail-on-error is the only thing that turns a generation error into
	// a non-zero exit; without it CI still gets every file.
	t.Run("without_fail_on_error_exits_zero_with_the_same_files", func(t *testing.T) {
		lenient := generateManifests(t, []string{"foo=bar"})
		assert.Equal(t, 0, lenient.ExitCode)
		assert.ElementsMatch(t, run.files(t), lenient.files(t))
	})
}

func TestE2E_CLIEdgeCases(t *testing.T) {
	setupE2E(t)

	t.Run("no_matching_applications_succeeds_without_files", func(t *testing.T) {
		run := generateManifests(t, []string{"env=foobar"}, "--fail-on-error")
		assert.Equal(t, 0, run.ExitCode)
		assert.Contains(t, run.Stdout, "Applications found: 0")
		assert.Empty(t, run.files(t))
	})

	t.Run("unreachable_server_exits_with_error_code", func(t *testing.T) {
		run := runCLI(t, "generate-manifests",
			"--server-address", "localhost:1",
			"--insecure",
			"--target-ref", e2eTargetRef,
		)
		assert.Equal(t, 1, run.ExitCode)
		assert.Contains(t, run.Stderr, "Error getting applications")
		assert.Empty(t, run.files(t))
	})
}
