The `tangle-cli` allows you to easily leverage `tangle` in your CI/CD pipelines.  For example, you can quickly generate manifests for all your deplyments with `tangle-cli generate-manifests` and run [kubeconform](https://github.com/yannh/kubeconform) or [conftest](https://www.conftest.dev/) to give your developers fast feedback!

Example usage:
```shell
tangle-cli generate-manifests --server-address localhost:8081 --insecure --folder ./tmpdir --target-ref test_gitops --fail-on-error
```

## Commands

### tangle-cli
```
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
```
Generate manifests for ArgoCD applications.

Usage:
  tangle-cli generate-manifests [flags]

Flags:
      --fail-on-error       Fail command if errors are found.
      --folder string       Folder to generate manifests in.  Defaults to current folder.
  -h, --help                help for generate-manifests
      --label strings       Labels to filter projects on in format 'key=value'.  Can be used multiple times.
      --retries int         Number of retried for failed calls.  Must be between 0 (no retries) and 5.
      --target-ref string   Git refernce to generate manifests.

Global Flags:
      --insecure                Don't validate SSL certificate on client request
      --server-address string   ArgoCD server address
```
