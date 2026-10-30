<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg">
  <img alt="vigie" src="assets/logo.svg" width="400">
</picture>

# vigie

vigie is a small endpoint discovery server. It reads the endpoints of named services from one source each (`dns`, `file`, `url` or `kubernetes`) and answers `GET /services?service_name=<name>` with a JSON list of `ip`, `port` and `load_balancing_weight`, which any HTTP client can poll to follow the endpoints of a service.

Used by [flock](https://github.com/seagalia/flock), whose backends with an `eds` block poll that answer, and by meridian.

## Usage

```bash
vigie serve -c ./config.yml          # run the server
vigie check-config -c ./config.yml   # check the file, unknown keys included
vigie version                        # print the version, commit and build date
```

`-c/--config-path` defaults to `./config.yml`. Sample configuration: [assets/config.yml](assets/config.yml). Configuration reference: [docs/configuration.md](docs/configuration.md).

```console
$ curl -s 'http://localhost:8080/services?service_name=web'
[
  {
    "ip": "10.0.0.5",
    "port": 8080,
    "load_balancing_weight": 0
  }
]
```

The server also serves a page at `/` listing the services with the endpoints each one gives when the page loads, and `GET /healthz` for probes. `POST /reload` or `SIGHUP` reloads the services of the file.

## Run it in a container

```bash
make docker
docker run --rm -p 8080:8080 -v "$PWD/config.yml:/etc/vigie/config.yml:ro" vigie:dev
```

## Layout

```
main.go          the vigie CLI (serve, check-config, version)
app/             wiring of the HTTP server, reload by request and by SIGHUP
config/          the YAML configuration, its defaults and its validation
services/        the sources of endpoints (dns, file, url, kubernetes) and the /services handler
ui/              the page served at /, with the vigie brand assets
logging/         zerolog setup
assets/          logos, sample configuration, systemd unit
docs/            configuration reference
```

## Development

```bash
go build .        # the vigie binary
make test         # unit tests with the race detector
make test-e2e     # kubernetes and consul against kind and a container, needs docker
make lint         # golangci-lint, as in CI
```

See [CONTRIBUTING.md](CONTRIBUTING.md).
