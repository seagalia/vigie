# Configuration

vigie reads the endpoints of each service from one source (`dns`, `file`, `url` or `kubernetes`) and serves them as JSON over HTTP to any client that polls them. The flock agents use it as the endpoint discovery service (EDS) of their backends, and meridian reads it too. This page describes the YAML file and the HTTP endpoints of the server. A sample file is in [`assets/config.yml`](../assets/config.yml).

## Load and check the file

| Command | Description |
|---|---|
| `vigie serve -c <path>` | Runs the server with the file. The default path is `./config.yml`. |
| `vigie check-config -c <path>` | Checks the file without running the server, and reports unknown keys. |

`serve` ignores the keys it doesn't know, so run `check-config` after each change.

## File structure

```yaml
server:
  listen_addr: :8080
  tls:
    cert_file: /etc/vigie/vigie.crt
    key_file: /etc/vigie/vigie.key
log:
  level: info
  no_color: false
  in_json: false
disable_reload_by_request: false
cache:
  ttl: 5s
services:
  - name: default
    default_port: 8080
    file:
      path: /etc/vigie/endpoints.txt
```

| Key | Type | Required | Default | Description |
|---|---|---|---|---|
| `server.listen_addr` | string | no | `:8080` | Address the server listens on. |
| `server.tls.cert_file` | path | with `server.tls` | none | Certificate of the server, in PEM. |
| `server.tls.key_file` | path | with `server.tls` | none | Private key of the certificate, in PEM. |
| `log.level` | string | no | `info` | `trace`, `debug`, `info`, `warn` (or `warning`), `error`, `fatal` or `panic`. |
| `log.no_color` | boolean | no | `false` | Prints text without ANSI colours. |
| `log.in_json` | boolean | no | `false` | Prints one JSON object per line instead of text. |
| `disable_reload_by_request` | boolean | no | `false` | Removes the `POST /reload` endpoint. |
| `cache.ttl` | duration | no | `5s` | How long vigie reuses the answer of a source, for the services without their own `cache_ttl`. `0s` reads the source on every request. |
| `services` | list | yes | none | Services the server answers for, at least one. |

Without `server.tls`, the server speaks plain HTTP. With it, the server speaks HTTPS only.

| Error | Cause |
|---|---|
| `services is empty` | `services` is empty or missing. |
| `server tls: cert_file and key_file are required` | `server.tls` is set with an empty `cert_file` or `key_file`. |
| `cache ttl is negative` | `cache.ttl` is below `0s`. |

## `services[]`

| Key | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | string | yes | none | Name clients ask for. A request without a name gets the service named `default`. |
| `default_port` | integer | no | `80` | Port of the endpoints that don't give their own. |
| `cache_ttl` | duration | no | `cache.ttl` | How long vigie reuses the answer of this service's source. `0s` reads the source on every request. |
| `dns` | object | one source | none | Reads the endpoints from DNS. |
| `file` | object | one source | none | Reads the endpoints from a local file. |
| `url` | object | one source | none | Reads the endpoints from an HTTP endpoint. |
| `kubernetes` | object | one source | none | Reads the endpoints from a Kubernetes Service. |

Give each service one source. When a service has several, the first of `kubernetes`, `url`, `file` and `dns` wins, and vigie ignores the others.

vigie keeps the answer of each source for `cache_ttl` (`cache.ttl`, 5 seconds by default), then reads the source again on the next request. A change to the DNS records, the file, the remote list or the Kubernetes endpoints shows within that time, without a reload. Requests arriving while the answer has expired wait for a single read of the source, so many clients polling together cost one read. A source that fails isn't cached, the next request reads it again. A reload starts with an empty cache. Durations are written like `500ms`, `5s` or `1m`.

| Error | Cause |
|---|---|
| `service name is empty` | `name` is empty. |
| `service dns, file, url and kubernetes are empty, you must specify one of them` | The service has no source. |
| `service <name> cache_ttl is negative` | `cache_ttl` is below `0s`. |

A missing `default_port` only logs a warning and takes `80`.

### `dns`

```yaml
services:
  - name: api
    default_port: 8080
    dns:
      name: api.example.com
      type: SRV
      srv_service: http
      srv_proto: tcp
```

| Key | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | string | yes | none | DNS name to look up. |
| `type` | string | no | `IP` | `IP`, `SRV` or `TXT`, in any case. |
| `srv_service` | string | no | `http` with `SRV` | Service part of the SRV lookup. |
| `srv_proto` | string | no | `tcp` with `SRV` | Protocol part of the SRV lookup. |
| `resolver` | `host:port` | no | empty | DNS server to query instead of the resolver of the system, such as `127.0.0.1:8600`. An IPv6 address goes in brackets, `[::1]:8600`. |

vigie builds the SRV name from the three keys, so `name: api.example.com` with `srv_service: http` and `srv_proto: tcp` looks up `_http._tcp.api.example.com`.

With `resolver` set, every lookup of the service goes to that server over TCP, the lookups of the SRV targets included, and each request gives up after 8 seconds. TCP keeps a long answer whole, where a UDP answer can be cut. Without `resolver`, vigie uses the resolver of the system.

| `type` | Lookup | Endpoints |
|---|---|---|
| `IP` | A and AAAA records of `name` | Each address, on `default_port`. |
| `SRV` | SRV records of `name` | Each address of each target, on the port of the record or `default_port` when it's `0`, with the weight of the record. |
| `TXT` | TXT records of `name` | Each record starting with `eds=`, followed by an endpoint line as described in [Endpoint lines](#endpoint-lines). Other records are skipped. |

| Error | Cause |
|---|---|
| `service dns name is empty` | `name` is empty. |
| `service dns lookup is invalid, valid are IP, TXT, SRV` | `type` is none of the three values. |
| `service dns resolver is invalid: <err>` | `resolver` isn't a `host:port` with both parts. |

#### Consul

Consul answers DNS for its services, so the `dns` source reads them without a Consul-specific source. Point `resolver` at the DNS port of a Consul agent, or have the host forward the `.consul` domain to it and leave `resolver` empty.

```yaml
services:
  - name: api
    default_port: 8080
    dns:
      name: service.consul
      type: SRV
      srv_service: api
      srv_proto: tcp
      resolver: 127.0.0.1:8600
```

This looks up `_api._tcp.service.consul`, Consul's RFC 2782 form. Each instance gives one endpoint, with the port it registered and its weight `Weights.Passing`, which is `1` when the instance registered none.

- To keep only the instances with a tag, put the tag in place of `tcp`: `srv_proto: primary` looks up `_api._primary.service.consul`.
- To query another datacenter, put it in the name: `name: service.dc1.consul`.

Two settings of the Consul agent (`dns_config`) matter here:

- `only_passing = true`. By default, Consul DNS leaves out the instances whose checks are critical but keeps those in `warning`, and vigie would hand them to its clients.
- `enable_truncate = true`, when vigie goes through the resolver of the system rather than `resolver`. Over UDP, Consul cuts a long answer to a few records without telling the client, so a service with many instances would lose some of them. With `resolver`, vigie asks over TCP and gets every record.

### `file`

```yaml
services:
  - name: web
    default_port: 8080
    file:
      path: /etc/vigie/web-endpoints.txt
```

| Key | Type | Description |
|---|---|---|
| `path` | path | File holding one [endpoint line](#endpoint-lines) per line. |

### `url`

```yaml
services:
  - name: db
    default_port: 5432
    url:
      url: https://inventory.example.com/db-endpoints.txt
      basic_auth:
        username: vigie
        password: change-me
      insecure_skip_verify: false
```

| Key | Type | Required | Default | Description |
|---|---|---|---|---|
| `url` | URL | yes | none | Endpoint answering a `GET` with one [endpoint line](#endpoint-lines) per line. |
| `basic_auth.username` | string | no | empty | User name for HTTP basic authentication. Sent only with `password`. |
| `basic_auth.password` | string | no | empty | Password for HTTP basic authentication. Sent only with `username`. |
| `insecure_skip_verify` | boolean | no | `false` | Skips the TLS check of the remote server. |

The request times out after 8 seconds. An answer other than `200` fails the request with `bad status code: <code>, <body>`.

| Error | Cause |
|---|---|
| `service url is empty` | `url` is empty. |
| `service url is invalid: <err>` | `url` isn't a valid URL. |

### `kubernetes`

```yaml
services:
  - name: web
    default_port: 8080
    kubernetes:
      namespace: shop
      service: web
      port_name: http
      kubeconfig: ~/.kube/config
      context: prod
```

| Key | Type | Required | Default | Description |
|---|---|---|---|---|
| `service` | string | yes | none | Name of the Kubernetes Service. |
| `namespace` | string | no | `default` | Namespace of the Service. |
| `port_name` | string | no | empty | Name of the port to use, among the ports of the Service. |
| `kubeconfig` | path | no | empty | Kubeconfig to connect with. A leading `~/` is the home directory. |
| `context` | string | no | empty | Context of the kubeconfig. Empty means its current context. |

Without `kubeconfig`, vigie looks for `$KUBECONFIG`, then `~/.kube/config`, then the service account of the pod it runs in.

vigie reads the Service, then its EndpointSlices, and gives each address of each ready endpoint. An endpoint without a ready condition counts as ready, as in Kubernetes. Endpoints that aren't ready, terminating pods for instance, are left out. A dual-stack Service gives each pod once per address family, and slices of host names (`FQDN`) are skipped.

- The port is the one named `port_name`. Without `port_name`, it's the port of the Service when it has only one, else `default_port`.
- The weight is `0`, Kubernetes has no weight per endpoint.

vigie loads the kubeconfig when the services load, at start and on each reload, and connects to the cluster each time the cache of the service expires, with an 8 second timeout. Each read sends two requests to the Kubernetes API, with no rate limit on the vigie side, so the limits of the API server apply. A kubeconfig that can't be loaded stops vigie at start, and keeps the previous services on reload. `check-config` doesn't load the kubeconfig, so it can't catch that.

In a cluster, the service account of vigie needs to read the Service and list its EndpointSlices:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: vigie
  namespace: shop
rules:
  - apiGroups: [""]
    resources: [services]
    verbs: [get]
  - apiGroups: [discovery.k8s.io]
    resources: [endpointslices]
    verbs: [list]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: vigie
  namespace: shop
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: vigie
subjects:
  - kind: ServiceAccount
    name: vigie
    namespace: vigie
```

| Error | Cause |
|---|---|
| `service kubernetes service is empty` | `service` is empty. |
| `load kubernetes client config: <err>` | At load, the kubeconfig or its context can't be loaded, or vigie runs outside a cluster without a kubeconfig. |
| `get service <namespace>/<service>: <err>` | On a request, the Service doesn't exist, or the cluster can't be reached or refuses vigie. |
| `port <port_name> not found in service <namespace>/<service>` | On a request, the Service has no port named `port_name`. |

A Service that exists without any ready endpoint answers `200` with `[]`.

## Endpoint lines

Files, remote lists and `eds=` TXT records hold one endpoint per line:

```text
# web servers
10.0.0.5
10.0.0.6:8081
10.0.0.7:8080#3
http://10.0.0.8:8080#1
```

- The form is `host[:port][#weight]`, with an optional `scheme://` in front, which vigie ignores.
- Without a port, the endpoint takes `default_port`. Without a weight, its weight is `0`.
- Empty lines and lines starting with `#` are skipped.
- vigie doesn't resolve host names, so write IP addresses. The flock agent refuses an endpoint whose `ip` isn't an IP address.

A line with a weight or a port that isn't a number fails the whole request with `invalid weight` or `invalid port`.

## Server endpoints

| Method and path | Answer |
|---|---|
| `GET /services?service_name=<name>` | `200` with the endpoints of the service as JSON. Without `service_name`, the service named `default`. |
| `GET /` | The page listing the services, their source, and the endpoints each one gives when the page loads. The source of a `url` service is shown without its credentials. |
| `GET /healthz` | `200` with `ok` while the server runs. |
| `POST /reload` | Reads the file again and replaces the services, then answers `200` with `services reloaded`. When the new file is invalid, or a source of it can't be built, such as a `kubernetes` source whose kubeconfig can't be loaded, it keeps serving the previous services and answers `422` with `services not reloaded: <error>`. Absent with `disable_reload_by_request: true`, it then answers `404`. Other methods get `405`. |

```console
$ curl -s 'http://localhost:8080/services?service_name=web'
[
  {
    "ip": "10.0.0.5",
    "port": 8080,
    "load_balancing_weight": 0
  },
  {
    "ip": "10.0.0.7",
    "port": 8080,
    "load_balancing_weight": 3
  }
]
```

| Status | Cause |
|---|---|
| `404` | `service <name> not found`, no service has this name. |
| `500` | `resolve service <name> failed: <err>`, the source failed. |

`POST /reload` has no authentication. Set `disable_reload_by_request: true` when other hosts can reach the server, and reload with a signal instead.

## Reload

Sending `SIGHUP` to the process has the same effect as `POST /reload`, and works with `disable_reload_by_request: true`.

```shell
kill -HUP "$(pgrep -x vigie)"
```

A reload only replaces `services`. A change to `server`, `log` or `disable_reload_by_request` needs a restart. When the new file is invalid, vigie logs the error and keeps the services it had.
