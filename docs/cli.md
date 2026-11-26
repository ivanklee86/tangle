The `tangle-cli` allows you to easily leverage `tangle` in your CI/CD pipelines.  For example, you can quickly generate manifests for all your deplyments with `tangle-cli generate-manifests` and run [kubeconform](https://github.com/yannh/kubeconform) or [conftest](https://www.conftest.dev/) to give your developers fast feedback!

Example usage:

```shell
tangle-cli generate-manifests --server-address localhost:8081 --insecure --folder ./tmpdir --target-ref test_gitops --fail-on-error
```

## Commands

### tangle-cli

```text
Usage:
  tangle-cli [flags]
  tangle-cli [command]

Available Commands:
  completion         Generate the autocompletion script for the specified shell
  generate-manifests Generate manifests.
  help               Help about any command

Flags:
  -h, --help                    help for tangle-cli
      --insecure                Don't validate SSL certificate on client request
      --server-address string   ArgoCD server address

Use "tangle-cli [command] --help" for more information about a command.
```

### tangle-cli generate-manifests

```text
Generate manifests for ArgoCD applications.

Usage:
  tangle-cli generate-manifests [flags]

Flags:
      --exclude-label strings            Labels to exclude projects on in format 'key=value'.  Can be used multiple times.
      --fail-on-error[=true|false]       Fail command if errors are found.
      --folder string                    Folder to generate manifests in.  Defaults to current folder.
  -h, --help[=true|false]                help for generate-manifests
      --infer-target-repo[=true|false]   With no --target-repo, let a multi-source application whose git sources are all in one repo use that repo.
      --label strings                    Labels to filter projects on in format 'key=value'.  Can be used multiple times.
      --retries int                      Number of retried for failed calls.  Must be between 0 (no retries) and 5.
      --target-ref string                Git reference to generate manifests at: a branch, a tag or a commit SHA.
      --target-repo stringArray          Repo the target ref belongs to. Sources in it get the target ref, every other source stays at its live revision, and applications with no source in it are not affected.  Can be used multiple times.

Global Flags:
      --insecure[=true|false]   Don't validate SSL certificate on client request
      --server-address string   ArgoCD server address
```

## Multi-source applications

An Application with `spec.sources` takes its manifests from several places, so a target ref only applies to the sources in the repo it was pushed to. Pass that repo with `--target-repo`, and Tangle sends `--target-ref` to every source in it while every other source stays at its live revision. Any spelling of the URL matches (`https://`, `ssh://`, `git@host:`, with or without `.git`). In CI, the repo the pipeline runs in is usually the one to pass:

```shell
tangle-cli generate-manifests --server-address tangle.example.com --target-ref "$GITHUB_SHA" --target-repo "https://github.com/$GITHUB_REPOSITORY" --fail-on-error
```

- An application with no source in any `--target-repo` is reported as **Not affected**. Nothing is rendered for it, and it never fails the run.
- A multi-source application with no `--target-repo` is reported as **Needs target repo**, with an `error-<argocd>-<app>.txt` listing the repos it uses. It fails the run under `--fail-on-error`. Pass `--target-repo`, or `--infer-target-repo` to let an application whose git sources are all in one repo use that repo.
- `--target-repo` can be repeated when the same branch was pushed to several repos.
- Single-source applications behave as before when no `--target-repo` is passed.
