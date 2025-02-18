# Server

Tangle is distributed as a Docker image and can be installed on Kubernetes via [Helm charts](https://github.com/ivanklee86/tangle-deployments/tree/main/charts/tangle).

You can try out Tangle locally by:
- Cloning the repository and opening it as a Dev Container.
- Run `task services` to start a Kubernetes cluster with ArgoCD and a few `Applications`.
- Start Tangle with the following command.

```shell
docker run --rm -it -v `pwd`/integration:/config -e TANGLE_CONFIG_PATH=/config/tangle.yaml --network=host --env-file .env ghcr.io/ivanklee86/tangle:latest
```

# CLI

## Homebrew

```shell
brew tap ivanklee86/homebrew-tap
brew install ivanklee86/tap/argonap
```

## Docker

The `tangle-cli` can be found in the `/usr/bin` folder of the Tangle Docker image.
