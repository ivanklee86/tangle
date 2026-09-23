## Health

Healthcheck can be found at `/health`.

## Metrics

Prometheus metrics are exposed at `/metrics`.

### ArgoCD connections

These metrics show how the connection to each ArgoCD is holding up:

| Metric | What it tells you |
| --- | --- |
| `argocd_client_retries_total{argocd,method,reason}` | Calls to ArgoCD that were repeated after a failure. `reason="transient"` means the response was lost on the way back, which is usually a reverse proxy or ingress in front of ArgoCD closing the connection. `reason="reconnect"` means Tangle's own connection had to be re-established. |
| `argocd_client_dials_total{argocd,reason,result}` | Connections opened to each ArgoCD, and whether they worked. |
| `argocd_client_connection_generation{argocd}` | How many times the connection to each ArgoCD has been replaced since startup. |

An occasional transient retry is normal and invisible to users. A steadily rising rate points to the proxy in front of ArgoCD. Check its idle and response timeouts, and whether it's being restarted.
