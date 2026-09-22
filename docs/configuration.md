The `tangle.yaml` file is the primary configuration file and specifies how Tangle connects to your ArgoCD servers.

```yaml
argocds: # This section defines ArgoCD instances.
  test:
    address: "localhost:8080" # Address of the ArgoCD instance.  Should NOT have https://
    insecure: true # Optional, skips TLS certificate verification (e.g. a self-signed cert).
    plainText: true # Optional, connects without TLS at all (e.g. ArgoCD behind a plain-HTTP Gateway/Ingress).
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
| --------------- | ----------- | --------------- | ------------- |
| domain | No | (the browser's own address) | Public URL of this Tangle, used for the links the web UI offers to copy |
| timeout | No | 60 (seconds) | Timeout on ArgoCD queries |
| listWorkers | No | 10 | Control `List` parallelism |
| manifestsWorkers | No | 5 | Controls `GetManifests` parallelism |
| hardRefreshWorkers | no | 5 | Controls `Get` with hard refresh parallelism |

### `domain`

The web UI shows a copyable link for every query. By default it builds that link from the address in
the browser's own address bar, which is right whenever the address you reached Tangle on is the one
you'd send to somebody.

Set `domain` when it isn't — most often when people reach Tangle through a `kubectl port-forward`, so
their browser says `localhost:8081` and a copied link is useless to a colleague:

```yaml
domain: "https://tangle.your-company.com"
```

It must be an absolute `http://` or `https://` URL. A trailing slash is dropped; a sub-path is kept,
so `https://example.com/tangle` works. **A malformed value stops Tangle at startup** rather than being
ignored — the alternative is every copied link quietly pointing somewhere wrong.

## Environment variables

Any setting above can also be set with a `TANGLE_`-prefixed environment variable, which takes
precedence over `tangle.yaml` — `TANGLE_TIMEOUT=30`, `TANGLE_LISTWORKERS=20`. The name is matched
against the configuration keys without regard to case, so `TANGLE_LISTWORKERS` and
`TANGLE_listWorkers` both set `listWorkers`. An ArgoCD instance's settings are reachable as
`TANGLE_ARGOCDS_<instance>_<setting>`, e.g. `TANGLE_ARGOCDS_PROD_ADDRESS`.

A `TANGLE_`-prefixed variable that doesn't name a configuration key is ignored, and Tangle logs the
names of every variable it ignored once at startup. Check that line first if a setting you expected
to apply didn't — a typo'd name shows up there.

### Running on Kubernetes

Kubernetes injects `<SERVICE>_SERVICE_HOST`, `<SERVICE>_PORT` and `<SERVICE>_PORT_<port>_<proto>_ADDR`
style variables into every pod sharing a namespace with a Service, named after that Service. If your
Tangle Service is named `tangle`, those land in Tangle's own `TANGLE_` namespace. Tangle ignores them,
but you can drop them entirely by setting `enableServiceLinks: false` on the pod spec:

```yaml
spec:
  template:
    spec:
      enableServiceLinks: false
```
