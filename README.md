<h1 align="center" style="border-bottom: none">
    <img alt="whitebox" src="/docs/images/whitebox-logo.png" width="150"><br>
    whitebox
</h1>

<p align="center">
    <code>whitebox</code> for <a href="https://prometheus.io/" target="_blank">Prometheus</a> provides availability monitoring of external VPN services powered by VMESS, VLESS, TROJAN, WG and SS.
</p>

<div align="center">

[![Go Report Card](https://goreportcard.com/badge/github.com/quyxishi/whitebox)](https://goreportcard.com/report/github.com/quyxishi/whitebox)

</div>

The features that distinguish whitebox:

- **Multi-protocol VPN Probing**: Supports probing of external VPN services including VMESS, VLESS, Trojan, Wireguard and Shadowsocks.
- **RESTful API Service**: Exposes HTTP endpoints for on-demand or scheduled connectivity checks.
- **Custom Probe Configuration**: Accepts probe parameters such as connection details, target URLs, max redirects, and configurable timeouts.
- **Prometheus Metrics Integration**: Exposes key probe results as Prometheus metrics.

### Prerequisites

- Docker (or Golang ≥1.18)

## Getting Started

```shell
git clone -b main https://github.com/quyxishi/whitebox
cd ./whitebox
```

#### via `docker-compose.yaml`

```shell
sudo docker compose up --build -d
```

#### via `Dockerfile`

###### Build
```shell
sudo docker build --tag whitebox .
```

###### Running
```shell
sudo docker run --rm -d -p 9116:9116 whitebox
```

### Checking the results

After deploying, you can validate VPN tunnel probing by visiting:
```url
http://localhost:9116/probe?ctx=<urlencoded_vpn_uri>&target=google.com
```

After that, you will see a Prometheus-style metrics page showing the results of the probe.

For example, the output may look like:
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
# HELP tun_probe_success Displays whether or not the probe over tunnel was a success
# TYPE tun_probe_success gauge
tun_probe_success 1
```

## Prometheus Configuration

Whitebox follows the [multi-target exporter pattern](https://prometheus.io/docs/guides/multi-target-exporter/).

###### Example Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'whitebox'
    metrics_path: /probe
    params:
      max_redirects: [ '0' ]  # Do not follow redirects. (Optional)
      timeout_ms: [ '5000' ]  # Probe timeout in ms. (Optional)
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
    ctx: "vless://c9f5228c-8870-47bd-a92f-9b38c7c02b08@1.2.3.4:443?type=tcp&encryption=none&security=reality&pbk=DF-3KL2W4RuNB2HgsEDmLqHLvvTTN4_QfwUCUn8Uhy0&fp=firefox&sni=web.max.ru&sid=dc8wq0b47450f9&spx=%2F&flow=xtls-rprx-vision#ring0-raii-idx0"
    client: "ring0-raii-idx0"  # Client unique identifier
    protocol: "vless"          # VPN protocol
    # You can also add additional labels here:
    #   sni: "web.max.ru"
    # And then, update relabel_configs in prometheus.yml job's config:
    #   - source_labels: [sni]
    #     target_label: sni
- targets: [ "https://x.com" ]
  labels:
    ctx: "vless://e8d12cd5-b436-4ed1-967c-7001efe6cf06@1.2.3.4:443?type=tcp&encryption=none&security=reality&pbk=DF-3KL2W4RuNB2HgsEDmLqHLvvTTN4_QfwUCUn8Uhy0&fp=firefox&sni=web.max.ru&sid=dc8wq0b47450f9&spx=%2F&flow=xtls-rprx-vision#gate-raii-idx0"
    client: "gate-raii-idx0"
    protocol: "vless"
- targets: [ "https://cloudflare.com" ]
  labels:
    # Wireguard connection must be supplied as: base64-encoded peer .ini config prefixed with 'wireguard://'
    ctx: "wireguard://W0ludGVyZmFjZV0KUHJpdmF0ZUtleSA9IFNObk5ON0l4YzV0ekNYS2FJNGZXNnEyOFYzbnhGS2YxcmNoYWt4bWdBbHM9CkFkZHJlc3MgPSAxMC4wLjAuMi8zMgpETlMgPSAxLjEuMS4xLCAxLjAuMC4xCk1UVSA9IDE0MjAKCiMgLTEKW1BlZXJdClB1YmxpY0tleSA9IHk2MTdkQ2dNM1g2bEtEanBkdDVhQ3dIWmROWW5OT0FwMFMyanFUbGpmZzA9CkFsbG93ZWRJUHMgPSAwLjAuMC4wLzAsIDo6LzAKRW5kcG9pbnQgPSAxLjIuMy40OjQ0Mw=="
    client: "wg-raii-idx0"
    protocol: "wireguard"
```

> [!TIP]
> After all of that, reload Prometheus, visit http://localhost:9090/targets and check for your `whitebox` job.

## See in Action

###### Example Grafana dashboard 'hemera' powered by `whitebox` and `blackbox`
![hemera-dashboard](/docs/images/hemera-view.png)

### Prometheus-less Approach

While `whitebox` is designed with Prometheus integration in mind, it can also be used **independently** as a lightweight VPN probing tool.

You can configure periodic probes using a simple `curl` command in a cron job or any task scheduler of your choice.

###### Example Curl probe cron job
```shell
*/5 * * * * curl -s "http://localhost:9116/probe?ctx=<urlencoded_vpn_uri>&target=google.com" >> /var/log/whitebox-probe.log 2>&1
```

## Roadmap

- [x] VLESS XHTTP transport protocol support.
- [x] CI/CD for basic build/test workflow.
- [x] JSON Subscriptions VPN-url's support.
- [ ] Whitebox YAML configuration w/ auto-reload.
- [ ] AmneziaWG protocol support.
- [ ] Authorization/OAuth 2.0 support.
- [ ] Configuration for TLS protocol of HTTP probe support.
- [ ] Response status/body validation.
- [ ] Custom HTTP-headers qualify support.
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

## License

MIT License, see [LICENSE](https://github.com/quyxishi/whitebox/blob/main/LICENSE.txt).
