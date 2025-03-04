The `tangle.yaml` file is the primary configuration file and specifies how Tangle connects to your ArgoCD servers.

```yaml
argocds: # This section defines ArgoCD instances.
  test:
    address: "localhost:8080" # Address of the ArgoCD instance.  Should NOT have https://
    insecure: true # Optional, can omit if ArgoCD has a proper certification.
    authTokenEnvVar: "ARGOCD_TOKEN"  # Name of environment variable containing ArgoCD JWT.
  prod:
    address: "localhost:8080"
    insecure: true
    authTokenEnvVar: "ARGOCD_PROD_TOKEN"

sortOrder:  # This section allows you to configure the order of ArgoCDs in the web UI.
  - test
  - prod
```

Additional configurations can be configured in the `tangle.yaml` or via environment variables with the `TANGLE_<var>` format.

| Configuration | Required? | Default Value | Description |
|---------------|-----------|---------------|-------------|
| timeout | No | 60 (seconds) | Timeout on ArgoCD queries |
| listWorkers | No | 10 | Control `List` parallelism |
| manifestWorkers | No | 5 | Controls `GetManifests` parallelism |
| hardRefreshWorkers | no | 5 | Controls `Get` with hard refresh parallelism |
