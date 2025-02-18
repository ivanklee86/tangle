Tangle uses a JWT to authenticate to ArgoCD. This can be configured in the Helm chart as follows:
```yaml
configs:
  cm:
    accounts.YOUR_ACCOUNT_NAME: apiKey

  rbac:
    policy.csv: |
      p, role:tangle, applications, get, *, allow
      g, YOUR_ACCOUNT_NAME, role:tangle
```

A JWT can be then generated using the ArgoCD CLI using the following command:
```shell
argocd login # Using username/password or SSO
argocd account generate-token --account YOUR_ACCOUNT
```
