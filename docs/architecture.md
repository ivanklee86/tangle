
## Technology

Tangle consists of two components:

- A server written in [`Golang`](https://go.dev) that implements the API and serves web assets.
    - It uses the [chi](https://github.com/go-chi/chi) router since it offers useful, clearly beneficial features (route groups/middleware support) while being lightweight and using standard http handlers.
    - It uses the [ArgoCD SDK](https://pkg.go.dev/github.com/argoproj/argo-cd@v1.8.7/pkg/apiclient) to talk to ArgoCD servers.
    - Each ArgoCD API has a [`alitto/pond`](https://github.com/alitto/pond) worker pool that allows us to balance concurrency against putting excess demand on the `repo` microservice..
    - Endpoints are documented with [OpenAPI](https://prometheus.io/) and server includes an [embedded Swagger UI](https://github.com/swaggest/swgui) in the `/swagger/` endpoint.
    - It uses standard observability tools like:
        - [hellofresh/health-go](https://prometheus.io/) `/health` endpoints.
        - [Prometheus](https://prometheus.io/) `/metrics` endpoint.
        - [httplog](https://prometheus.io/) for logging.
- A static website written with [`Svelte`](https://svelte.dev/) and [`Typescript`](https://www.typescriptlang.org/).
    - It uses [flowbite-svelte](https://flowbite-svelte.com/) as its design system.

Additionally it uses the following tools to improve our development experience:

- [Github Actions](https://github.com/features/actions) for CI/CD.
- [Development Containers](https://containers.dev/) to create a reproducable development environment.
- [pre-commit](https://pre-commit.com/) to automate formatting & checks.
- [k3d](https://k3d.io/stable/) to run Kubernetes cluster for testing locally & in CI/CD.
- [Task](https://taskfile.dev/) as a command runner/orchestrator.
- [Air](https://github.com/air-verse/air) for live reloading.
- [Goreleaser](https://goreleaser.com/) to build/distribute CLI.
- [mkdocs](https://www.mkdocs.org/) && [mkdocs-material](https://squidfunk.github.io/mkdocs-material/) for documentation.

## Principles
1. Features should have feature parity for CI/CD systems (via API and CLI tools) and humans (via website).
2. Interactions with ArgoCD should not interfere with its core job (deploying stuff!).  Users should be able to set sane limits.
