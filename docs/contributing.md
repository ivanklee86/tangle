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
3. Github Actions will build new Docker image & CLI. For a non-prerelease release, it also triggers an
   automatic Helm chart bump.

## Formatting and Linting

Frontend and backend code both have automatic formatting and linting:

For Go: `task go:fmt` & `task go:lint` respectively.
For frontend: `task ts:fmt` & `task ts:lint` respectively.

Where possible, rules are enforced with [pre-commit](https://pre-commit.com/) hooks.

## Running Tests

Note: Both test suites will (eventually) include unit/integration tests.  You need to run the server locally for tests to pass!

- Go tests can be run with `task go:tests`
- Frontend unit and component tests run with `task ts:test:unit`, and the mocked Playwright suite with `task ts:test:e2e`.

### Screenshot tests

`task ts:test:visual` compares the main screens, in light and dark, against the baselines in `web/e2e/visual/__screenshots__/`. It runs in the pinned Playwright Docker image (see [ADR 0033](adrs/0033-playwright-screenshot-tests-in-a-pinned-image.md)), so you need Docker but no backend: the screens are rendered on a fixture fleet (`web/e2e/fixtures/fleet.ts`).

When you change the UI on purpose, or a dependency update moves pixels, run `task ts:test:visual:update` and look at the changed PNGs before committing them. When CI's `visual` job fails, its `screenshot-diffs` artifact has the expected, actual and diff image for each failed screenshot.

## Documentation

Run `task python:serve` to run local document server.  Edit to your heart's content!

The web-UI screenshots in the README and `docs/index.md` are generated: `task ts:screenshots` renders them from the fixture fleet in the same Docker image and writes `docs/images/Tangle{Home,Applications,Diffs}.png`. The queries they show are set in `web/e2e/docs/readme.spec.ts`, so change the captions with them. `TangleCLI.png` is a terminal screenshot and is updated by hand.
