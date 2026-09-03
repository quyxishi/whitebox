<h1 align="center" style="border-bottom: none">
    <img alt="whitebox" src="/docs/images/whitebox-logo.png" width="125"><br>
    whitebox
</h1>

<p align="center">
    <code>whitebox</code> for <a href="https://prometheus.io/" target="_blank">Prometheus</a> provides availability monitoring of external VPN services powered by VMESS, VLESS, TROJAN, WG, AWG, SS and HYSTERIA.
</p>

<div align="center">

[![Go Report Card](https://goreportcard.com/badge/github.com/quyxishi/whitebox)](https://goreportcard.com/report/github.com/quyxishi/whitebox)
[![Docker Image](https://img.shields.io/docker/v/rxyvea/whitebox?logo=docker&label=docker%20hub&sort=semver)](https://hub.docker.com/r/rxyvea/whitebox)
[![Image Size](https://img.shields.io/docker/image-size/rxyvea/whitebox/latest?logo=docker&label=image%20size)](https://hub.docker.com/r/rxyvea/whitebox/tags)
[![License](https://img.shields.io/badge/license-MIT-blue)](/LICENSE.txt)

</div>

The features that distinguish whitebox:

- **Multi-protocol VPN Probing**: Supports probing of external VPN services including VMESS, VLESS, Trojan, Wireguard, AmneziaWG, Shadowsocks and Hysteria2.
- **RESTful API Service**: Exposes HTTP endpoints for on-demand or scheduled connectivity checks.
- **Custom Probe Configuration**: Accepts probe parameters such as connection details, target URLs, response validation rules, and configurable timeouts.
- **Prometheus Metrics Integration**: Exposes key probe results as Prometheus metrics.

## Getting Started

`whitebox` ships a ready-to-run image on Docker Hub: [`rxyvea/whitebox`](https://hub.docker.com/r/rxyvea/whitebox).

```shell
docker run --rm -d --name whitebox -p 9116:9116 rxyvea/whitebox:latest
```

That is a complete deployment. Whitebox starts with a built-in default scope and listens on `:9116`.

### Verify

Probe any target through a VPN connection URI. `ctx` is the URL-encoded connection URI, `target` is what to fetch through the resulting tunnel:

```shell
curl -sG http://localhost:9116/probe \
  --data-urlencode 'ctx=vless://c9f5228c-8870-47bd-a92f-9b38c7c02b08@1.2.3.4:443?type=tcp&encryption=none&security=reality&pbk=DF-3KL2W4RuNB2HgsEDmLqHLvvTTN4_QfwUCUn8Uhy0&fp=firefox&sni=ce-cdn.icloud-content.com&sid=620352b7&spx=%2F&flow=xtls-rprx-vision' \
  --data-urlencode 'target=https://google.com'
```

A Prometheus-style metrics page describing that single probe comes back. Refer for [Reading a probe response](#reading-a-probe-response) below.

The endpoints:

| Endpoint | Purpose |
| --- | --- |
| `/probe?ctx=…&target=…&scope=…` | Probe one tunnel. `scope` is optional and defaults to `default`. |
| `/metrics` | Exporter's own process, Go runtime and xray cache metrics. |

### With a configuration file

Custom scopes, timeouts, request bodies and `fail_if` validation rules live in a YAML file mounted into the container. Grab the annotated example and mount it:

```shell
curl -O https://raw.githubusercontent.com/quyxishi/whitebox/main/whitebox.yml

docker run --rm -d --name whitebox -p 9116:9116 \
  -v "$PWD/whitebox.yml:/etc/whitebox/whitebox.yml:ro" \
  rxyvea/whitebox:latest --config.file=/etc/whitebox/whitebox.yml
```

Select a scope per probe with `&scope=<scope_name>`. See [Whitebox Configuration](#whitebox-configuration) for what the file can express.

> [!TIP]
> Config is reloaded in place on **SIGHUP**: `docker kill -s HUP whitebox`.

### With Docker Compose

Drop this into a `docker-compose.yml` next to your `whitebox.yml`:

```yaml
services:
  whitebox:
    image: rxyvea/whitebox:latest
    container_name: whitebox
    restart: unless-stopped
    ports:
      - ${WHITEBOX_PORT:-9116}:9116
    command:
      - '--config.file=/etc/whitebox/whitebox.yml'
      - '--log.level=info'
    volumes:
      - ./whitebox.yml:/etc/whitebox/whitebox.yml
```

### The whole stack

[`examples/`](/examples) holds a working whitebox + [blackbox](https://github.com/prometheus/blackbox_exporter) + Prometheus + Grafana deployment, already wired together.

```shell
git clone -b main https://github.com/quyxishi/whitebox
cd whitebox/examples
docker compose up -d
```

Grafana lands on http://localhost:3000 (`admin`/`admin` by default), Prometheus on http://localhost:9090 — check its `/targets` page for the `whitebox` job. Point [`whitebox-sd-config.yml`](/examples/whitebox-sd-config.yml) at your own connection URIs to get real data, then import [`hemera-dashboard.json`](/examples/hemera-dashboard.json) into Grafana to visualise it.

### Building from source

Requires **Go 1.26+**, or just Docker for the image build:

```shell
git clone -b main https://github.com/quyxishi/whitebox
cd whitebox

docker build --tag whitebox .
docker run --rm -d -p 9116:9116 whitebox
```

See [Development](#development) for the local (non-Docker) loop.

### Logging

Verbosity is controlled by the `--log.level` flag (`-l`) or the `WHITEBOX_LOG_LEVEL` environment variable, one of `debug`, `info` (default), `warn`, `error`:

```shell
docker run --rm -d -p 9116:9116 -e WHITEBOX_LOG_LEVEL=warn rxyvea/whitebox:latest
```

On `debug`, per-request access logs and verbose `xray-core` tunnel logs are emitted as well; on any higher level both are suppressed.

### Listen address

By default whitebox binds to `:9116`. Use the `--web.listen-address` flag or the `WHITEBOX_LISTEN_ADDRESS` environment variable to change the port (or the interface), accepting either `[host]:port` or a bare port:

```shell
sudo docker run --rm -d -p 9200:9200 -e WHITEBOX_LISTEN_ADDRESS=:9200 whitebox
```

With `docker-compose.yaml` the published host port can be overridden through the `WHITEBOX_PORT` variable, e.g. `WHITEBOX_PORT=9200 sudo docker compose up -d`.

### Reading a probe response

Every probe returns the outcome as Prometheus metrics:

```md
# HELP tun_probe_duration_seconds Returns how long the probe took to complete in seconds
# TYPE tun_probe_duration_seconds gauge
tun_probe_duration_seconds 1.554994
# HELP tun_probe_http_content_length_bytes Length of HTTP content response in bytes
# TYPE tun_probe_http_content_length_bytes gauge
tun_probe_http_content_length_bytes -1
# HELP tun_probe_http_duration_seconds Duration of HTTP request by phase, summed over all traces
# TYPE tun_probe_http_duration_seconds gauge
tun_probe_http_duration_seconds{phase="connect"} 0.1998826
tun_probe_http_duration_seconds{phase="processing"} 0.3262475
tun_probe_http_duration_seconds{phase="resolve"} 0
tun_probe_http_duration_seconds{phase="tls"} 1.2223279
tun_probe_http_duration_seconds{phase="transfer"} 0.0064185
# HELP tun_probe_http_redirects The number of redirects
# TYPE tun_probe_http_redirects gauge
tun_probe_http_redirects 1
# HELP tun_probe_http_ssl Indicates if SSL was used for the final trace
# TYPE tun_probe_http_ssl gauge
tun_probe_http_ssl 1
# HELP tun_probe_http_status_code Response HTTP status code
# TYPE tun_probe_http_status_code gauge
tun_probe_http_status_code 200
# HELP tun_probe_http_uncompressed_body_length_bytes Length of uncompressed response body in bytes
# TYPE tun_probe_http_uncompressed_body_length_bytes gauge
tun_probe_http_uncompressed_body_length_bytes 17650
# HELP tun_probe_instance_cached Indicates if the probe reused an already-built xray instance
# TYPE tun_probe_instance_cached gauge
tun_probe_instance_cached 1
# HELP tun_probe_success Displays whether or not the probe over tunnel was a success
# TYPE tun_probe_success gauge
tun_probe_success 1
```

### Exporter metrics

`/probe` describes a single tunnel. The exporter process itself is exposed separately:

```url
http://localhost:9116/metrics
```

This serves the Go runtime and process collectors (`go_memstats_heap_inuse_bytes`, `go_goroutines`, ...) alongside xray instance cache statistics:

| Metric | Description |
| --- | --- |
| `whitebox_xray_instances` | Instances currently held by the cache |
| `whitebox_xray_instances_in_use` | Instances currently leased by an in-flight probe |
| `whitebox_xray_cache_hits_total` / `_misses_total` | Probes that reused vs. built an instance |
| `whitebox_xray_cache_evictions_total{reason}` | Evictions by `ttl`, `lru`, `reload` or `shutdown` |
| `whitebox_xray_cache_overflow_total` | Times `max_entries` was exceeded because every instance was in use |
| `whitebox_xray_instance_build_duration_seconds` | Cost of constructing and starting an instance |
| `whitebox_xray_instance_build_failures_total` | Failures to construct or start an instance |

In steady state `whitebox_xray_instances` should settle at the number of distinct cacheable connection URIs scraped, and `whitebox_xray_cache_misses_total` should stop increasing. Wireguard targets are the exception: they are never cached, so each of their probes counts as a miss.

## Whitebox Configuration

Refer to the [annotated example configuration](/whitebox.yml) and [code reference](/internal/config/config.go) for implementation details.

### Xray instance reuse

Whitebox reuses started xray instances across probes, keyed by the generated xray config, rather than building one per request.

This is required for bounded memory, not merely an optimisation: xray-core's `xhttp`, `grpc` and `hysteria` dialers cache per-connection state in package-level maps keyed by the `*internet.MemoryStreamConfig` pointer that `core.New` allocates, and never delete from them. One instance per probe therefore leaks one permanent entry per probe, which is [issue #10](https://github.com/quyxishi/whitebox/issues/10).

Consequences worth knowing:

- For `xhttp`, whitebox injects `xmux.hMaxRequestTimes: 1` into generated configs so each probe still establishes a fresh transport connection to the VPN server. An explicit `xmux` in the connection URI is always left untouched.
- For `grpc` and `hysteria2` there is no equivalent knob, so consecutive probes may share the underlying connection. A dead server still fails the probe; what is not re-exercised every probe is the handshake itself. Set `instance_cache.enabled: false` if per-probe handshake coverage on those transports matters more than flat memory — see [Container memory limits](#container-memory-limits) before doing so.
- `wireguard` and `awg` outbounds are never reused, whatever `instance_cache.enabled` says, so `tun_probe_instance_cached` is always `0` for them. xray-core creates the tunnel device lazily, on the first proxied request, and the closure that opens the outer UDP socket captures that request's context; a reused instance would stay bound to a context that was cancelled when the probe which built it returned, and every later probe over it would fail. Nothing is lost by rebuilding them: they have no dialer-level cache to leak, and excluding them also stops a live gVisor netstack being held for a whole `ttl`.
- `raw`/`tcp`, `ws`, `httpupgrade` and `kcp` are unaffected — they have no dialer-level cache and reconnect per probe regardless.

Use `tun_probe_instance_cached` to tell cold probes from warm ones when reading `tun_probe_http_duration_seconds`.

### Container memory limits

The cache above is what keeps the heap bounded. With `instance_cache.enabled: false` or on a build predating the cache, the dialer-map leak from [issue #10](https://github.com/quyxishi/whitebox/issues/10) is back in play, and, left alone, the process grows until the host runs out of memory.

A memory cap that still permits swap is worse than no cap at all: the process thrashes inside its own cgroup instead of being killed. In one deployment `memory.max` was hit 8.7M times with 420M major faults and 1.9 TB read from disk, while `oom_kill` stayed at `0`. That disk I/O degraded everything else on the machine, the probes included, so the monitoring reported a fleet-wide outage that did not exist.

**Cap memory** and **forbid swap**, so the kernel OOM-kills the container and Docker restarts it:

```shell
docker run -d --name whitebox -p 9116:9116 \
  --restart unless-stopped \
  --memory 512m --memory-swap 512m \
  rxyvea/whitebox:latest
```

`--memory-swap` is the combined memory + swap ceiling, so setting it equal to `--memory` leaves no swap at all. The same pair under Compose:

```yaml
services:
  whitebox:
    restart: unless-stopped
    mem_limit: 512m
    memswap_limit: 512m
```

Expect the container to be OOM-killed and restarted periodically (roughly every three days at a 512m cap) instead of dragging the host down.

Verify against the cgroup rather than the flags. The image is distroless, so read it from the host instead of through `docker exec`:

```shell
pid=$(docker inspect -f '{{.State.Pid}}' whitebox)
cgroup=/sys/fs/cgroup$(cut -d: -f3 /proc/$pid/cgroup)

cat "$cgroup/memory.swap.max"  # 0
cat "$cgroup/memory.events"    # oom_kill should climb; max should not run away
```

In cgroup v2 a `memory.swap.max` of `0` means swap is forbidden.

> [!WARNING]
> `docker update` does not apply `--memory-swap` to a running container's cgroup. `docker inspect` will happily report the new swap limit while the cgroup still permits swap; it takes a restart or a recreate to actually apply.

## Prometheus Configuration

Whitebox follows the [multi-target exporter pattern](https://prometheus.io/docs/guides/multi-target-exporter/).

###### Example Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'whitebox'
    metrics_path: /probe
    file_sd_configs:
      - files: [ '/etc/prometheus/whitebox-sd-config.yml' ]  # File service discovery configurations (targets).
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target  # 'target' -> '?target=...'.
      - source_labels: [ctx]
        target_label: __param_ctx     # 'ctx' -> '?ctx=...'.
      - source_labels: [client]
        target_label: client          # Label all probe's with client id.
      - source_labels: [protocol]
        target_label: protocol        # Label all probe's with used protocol.
      - source_labels: [__param_target]
        target_label: target          # Label all probe's with target.
      - target_label: __address__
        replacement: 127.0.0.1:9116   # The whitebox real hostname:port.
```

###### Example file service discovery configuration (targets):

```yaml
- targets: [ "https://google.com" ]
  labels:
    ctx: "vless://c9f5228c-8870-47bd-a92f-9b38c7c02b08@1.2.3.4:443?type=tcp&encryption=none&security=reality&pbk=DF-3KL2W4RuNB2HgsEDmLqHLvvTTN4_QfwUCUn8Uhy0&fp=firefox&sni=ce-cdn.icloud-content.com&sid=dc8wq0b47450f9&spx=%2F&flow=xtls-rprx-vision#ring0-raii-idx0"
    client: "ring0-raii-idx0"  # Client unique identifier
    protocol: "vless"          # VPN protocol
    # You can also add additional labels here:
    #   sni: "web.max.ru"
    # And then, update relabel_configs in prometheus.yml job's config:
    #   - source_labels: [sni]
    #     target_label: sni
- targets: [ "https://cloudflare.com" ]
  labels:
    # Wireguard connection must be supplied as: base64-encoded peer .ini config prefixed with 'wireguard://'
    ctx: "wireguard://W0ludGVyZmFjZV0KUHJpdmF0ZUtleSA9IFNObk5ON0l4YzV0ekNYS2FJNGZXNnEyOFYzbnhGS2YxcmNoYWt4bWdBbHM9CkFkZHJlc3MgPSAxMC4wLjAuMi8zMgpETlMgPSAxLjEuMS4xLCAxLjAuMC4xCk1UVSA9IDE0MjAKCiMgLTEKW1BlZXJdClB1YmxpY0tleSA9IHk2MTdkQ2dNM1g2bEtEanBkdDVhQ3dIWmROWW5OT0FwMFMyanFUbGpmZzA9CkFsbG93ZWRJUHMgPSAwLjAuMC4wLzAsIDo6LzAKRW5kcG9pbnQgPSAxLjIuMy40OjQ0Mw=="
    client: "wg-raii-idx0"
    protocol: "wireguard"
- targets: [ "https://google.com" ]
  labels:
    # AmneziaWG connection with obfuscation parameters (Jc, Jmin, Jmax, S1, S2, H1-H4)
    # Must be supplied as: base64-encoded peer .ini config prefixed with 'awg://'
    ctx: "awg://W0ludGVyZmFjZV0KUHJpdmF0ZUtleSA9IFNObk5ON0l4YzN0emxYS2FJNGY4NnEyOFYzbnhGS2YzcmNoYWt4bWdCbHM9CkFkZHJlc3MgPSAxMC4wLjAuMi8zMgpETlMgPSAxLjEuMS4xLCAxLjAuMC4xCk1UVSA9IDE0MjAKSmMgPSAzCkptaW4gPSA1MApKbWF4ID0gMTAwMApTMSA9IDIwClMyID0gNzgKSDEgPSAzOTEzMTI3OApIMiA9IDgzMjEzODE4NQpIMyA9IDE0MzY5NTc4NTcKSDQgPSAxNjM1ODc3NzQ2CgpbUGVlcl0KUHVibGljS2V5ID0geTYxN2RDZ00zWDZsS0RqcGR0NWFHY0FaZE5Zbk5PQXAwUzNqYVRsamZnMD0KQWxsb3dlZElQcyA9IDAuMC4wLjAvMCwgOjovMApFbmRwb2ludCA9IDEuMi4zLjQ6Mjc3ODkK"
    client: "awg-raii-idx0"
    protocol: "amneziawg"
```

> [!TIP]
> After all of that, reload Prometheus, visit http://localhost:9090/targets and check for your `whitebox` job.

## See in Action

###### Example Grafana dashboard 'jarvis' powered by `whitebox` and `blackbox`
![jarvis-dashboard](/docs/images/jarvis-view.png)

### Prometheus-less Approach

While `whitebox` is designed with Prometheus integration in mind, it can also be used **independently** as a lightweight VPN probing tool.

You can configure periodic probes using a simple `curl` command in a cron job or any task scheduler of your choice.

###### Example Curl probe cron job
```shell
*/5 * * * * curl -s "http://localhost:9116/probe?ctx=<urlencoded_vpn_uri>&target=google.com" >> /var/log/whitebox-probe.log 2>&1
```

## Roadmap

- [x] VLESS XHTTP transport protocol support ([#1](https://github.com/quyxishi/whitebox/pull/1)).
- [x] CI/CD for basic build/test workflow ([#2](https://github.com/quyxishi/whitebox/pull/2)).
- [x] JSON Subscriptions VPN-url's support ([#3](https://github.com/quyxishi/whitebox/pull/3)).
- [x] HTTP Roundtrip tracing w/ duration metrics ([#4](https://github.com/quyxishi/whitebox/pull/4)).
- [x] AmneziaWG protocol support, thanks to [@nsvk13](https://github.com/nsvk13) ([#6](https://github.com/quyxishi/whitebox/pull/6)).
- [x] Whitebox YAML configuration w/ auto-reload by SIGHUP ([#7](https://github.com/quyxishi/whitebox/pull/7)).
- [x] Response status/body validation.
- [x] Custom HTTP-headers qualify support.
- [x] Configuration environment variables interpolation support ([#8](https://github.com/quyxishi/whitebox/pull/8)).
- [x] Hysteria2 + XHTTP extra settings support.
- [ ] Authorization/OAuth 2.0 support.
- [ ] Configuration for TLS protocol of HTTP probe support.
- [ ] More advanced metrics.

## Development

###### Build
```bash
make build
```

###### Running
```bash
make run
```

###### Live-reload
```bash
make watch
```

###### Tests
```bash
make test
make test-race
```

###### Profiling

Set `WHITEBOX_EXPOSE_PPROF=1` to mount `net/http/pprof` under `/debug/pprof`:

```bash
WHITEBOX_EXPOSE_PPROF=1 make run
go tool pprof http://localhost:9116/debug/pprof/heap
```

## License

MIT License, see [LICENSE](https://github.com/quyxishi/whitebox/blob/main/LICENSE.txt).
