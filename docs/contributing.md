## Local Development Workflow

1. Start development container (preferably in Visual Studio Code).
2. Run `task services` to start Kubernetes cluster, install ArgoCD + ingress controller, and test `Applications`.
3. Run `task go:run` (to just run the server) or `task go:run:reloading` (reload the server).
4. Run `task ts:install` to install Node dependencies.
5. Run `task ts:dev` to start Svelte development server.

## Submitting a PR

Pull requests are the best way to propose changes to the codebase (we use [Github Flow](https://docs.github.com/en/get-started/using-github/github-flow)). We actively welcome your pull requests:

1. Fork the repo and create your branch from main.
2. If you've added code that should be tested, add tests.
3. If you've changed APIs, update the documentation.
4. Ensure the test suite passes.
5. Make sure your code lints.
6. Issue that pull request!

## Cutting a release

1. Land your changes on `main`.
2. Create a [Release](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository) with a new [semver](https://semver.org/) tag.
3. Github Actions will build new Docker image & CLI.
4. Update the [tangle-deployments](https://github.com/ivanklee86/tangle-deployments) repository with new application version (`Renovate` usually gives you a handy PR!).
5. Bump chart version and create a new Helm release.

## Formatting and Linting

Frontend and backend code both have automatic formatting and linting:

For Go: `task go:fmt` & `task go:lint` respectively.
For frontend: `task ts:fmt` & `task ts:lint` respectively.

Where possible, rules are enforced with [pre-commit](https://pre-commit.com/) hooks.

## Running Tests
Note: Both test suites will (eventually) include unit/integration tests.  You need to run the server locally for tests to pass!

- Go tests can be run with `task go:tests`
- Frontend tests are WIP. 😭

## Documentation

Run `task python:serve` to run local document server.  Edit to your heart's content!
