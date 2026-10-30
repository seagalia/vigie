<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg">
  <img alt="vigie" src="assets/logo.svg" width="400">
</picture>

# vigie

vigie is a small endpoint discovery server. It reads the endpoints of named services from one source each (`dns`, `file`, `url` or `kubernetes`) and answers `GET /services?service_name=<name>` with a JSON list of `ip`, `port` and `load_balancing_weight`, which any HTTP client can poll to follow the endpoints of a service. The same answer comes from the gRPC service `vigie.eds.v1.EndpointDiscoveryService`, on the same port, with a Go client in [sdk/](sdk/).

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

```console
$ grpcurl -plaintext -d '{"service_name":"web"}' localhost:8080 vigie.eds.v1.EndpointDiscoveryService/GetEndpoints
```

The service answers over gRPC or the Connect protocol, with TLS, or in clear text (h2c) when `server.tls` is not set. It answers `NotFound` for an unknown service and `Unavailable` when the source of the service fails. The gRPC health service and reflection are served too, so `grpcurl` needs no proto file.

```go
client, err := clients.NewEndpointDiscovery("http://vigie:8080", clients.Options{Username: "flock", Password: "secret"})
resp, err := client.GetEndpoints(ctx, &eds.GetEndpointsRequest{ServiceName: "web"})
```

The server also serves a page at `/` listing the services with the endpoints each one gives when the page loads, and `GET /healthz` for probes. `POST /reload` or `SIGHUP` reloads the services of the file. With `server.basic_auth`, every endpoint but `/healthz` and the gRPC health service asks for the username and password of one of its users.

## Run it in a container

```bash
make docker
docker run --rm -p 8080:8080 -v "$PWD/config.yml:/etc/vigie/config.yml:ro" vigie:dev
```

## Layout

```
main.go          the vigie CLI (serve, check-config, version)
app/             wiring of the HTTP server and the gRPC services, reload by request and by SIGHUP
api/             the protobuf definition of the EDS service (buf module)
sdk/             the Go code generated from api/, and the client in sdk/clients
config/          the YAML configuration, its defaults and its validation
services/        the sources of endpoints (dns, file, url, kubernetes), the /services handler and the EDS service
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
make generate     # regenerate sdk/ from api/ with buf
make api-lint     # buf lint of api/
```

See [CONTRIBUTING.md](CONTRIBUTING.md).
