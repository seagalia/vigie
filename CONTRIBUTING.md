# Contributing to Vigie

Thank you for your interest in Vigie. This guide covers code and documentation changes in this repository.

Before you contribute, read the [code of conduct](CODE_OF_CONDUCT.md). It applies to every issue and pull request, and to every discussion of the project.

## Vigie overview

Vigie is an endpoint discovery server, in one Go module, `github.com/seagalia/vigie`, and one binary, `vigie`. The [README](README.md) lists what each directory holds.

The answer of `GET /services` is read by the flock agents and by meridian. Keep its JSON fields (`ip`, `port`, `load_balancing_weight`) and its query parameter (`service_name`) stable, or change the consumers in the same release.

## Before you start

Install these tools before you build or test Vigie:

- Go, at the version of the `go` line of `go.mod`.
- Docker, to run the unit tests in a Linux container and to build the image.
- golangci-lint v2 and [pre-commit](https://pre-commit.com/).

## Environment setup

1. Clone the repository.

    ```bash
    git clone https://github.com/seagalia/vigie.git
    cd vigie
    ```

2. Install the pre-commit hooks, which run golangci-lint and the YAML and whitespace fixers before each commit.

    ```bash
    pip install pre-commit
    pre-commit install
    ```

3. Build and run the binary with a configuration of your own, starting from the sample.

    ```bash
    go build .
    cp assets/config.yml config.yml
    ./vigie serve -c config.yml
    ```

## Test your change

The unit tests use Ginkgo v2 and Gomega. Each package has a `*_suite_test.go`.

| Command           | What it runs                                                    |
|-------------------|-----------------------------------------------------------------|
| `make test`       | Every unit test on the current platform, with the race detector |
| `make test-linux` | The same tests in a `golang:1.27` container, as in production   |
| `make test-e2e`   | The `kubernetes` source against a kind cluster and Consul through the `dns` source, in containers. Needs Docker, takes a few minutes |
| `make lint`       | golangci-lint, with the configuration CI uses                   |

To run one package or one spec, call Ginkgo directly:

```bash
go tool ginkgo -r ./services
go tool ginkgo --focus "some describe text" ./app
```

Everything builds with `CGO_ENABLED=0`, and the Makefile exports it. Only the unit tests turn cgo on, because the race detector needs it on Linux. Never add a dependency that needs cgo.

## Code conventions

- Log with zerolog. Get a component logger with `logging.Component("<name>")` from `logging`, and assign it to a variable before you call `.Info()`. Every log event ends with `Msg`, `Msgf` or `Send`, which `zerologlint` checks.
- Validation and defaults of the configuration live in the `UnmarshalYAML` of each type in `config/`.
- The page at `/` (`ui/`) is plain HTML, CSS and JavaScript embedded in the binary, with no build step. Its brand assets in `ui/brand/` come from seagalia-brand `exports/vigie`.

## Contribution workflow

### Report issues and bugs

Open an issue in the [GitHub issues of Vigie](https://github.com/seagalia/vigie/issues). Give the version (`vigie version`), the configuration that fails, and what you expected.

### Commit messages

Commits follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/), for example `feat(services): …`, `fix(config): …` or `docs: …`. Mark a breaking change with `!` after the type and a `BREAKING CHANGE:` footer.

### Pull requests

1. Fork the repository and create a branch from `main`.
2. Make your change, and add its tests and its documentation.
3. Run `make lint` and `make test`.
4. Open a pull request against `main`.

CI runs the lint and the unit tests on every pull request.

### Releases

A tag `v*` pushed to the repository runs goreleaser, which publishes the binaries for Linux, macOS and Windows on amd64 and arm64 to the GitHub releases, with `checksums.txt`.
