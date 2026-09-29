# AmneziaWG probes

AWG probes use the native `amneziawg-go/v3` userspace engine. This preserves
AWG 2 padding (`S3`, `S4`), header ranges (`H1`–`H4`), and signature chains
(`I1`–`I5`) that cannot be represented by the older Xray outbound adapter.
`Jc`, `Jmin`, `Jmax`, `S1`, `S2`, scalar headers, preshared keys, MTU,
AllowedIPs, and persistent keepalive are also passed to the engine.

The existing `ctx=awg://<base64 INI>` format remains supported. URL-encode
the complete value, including base64 `+`, `/`, and `=` characters.

For deployments that should keep private keys out of Prometheus target URLs,
mount a directory containing dedicated client profiles read-only and set
`WHITEBOX_AWG_CONFIG_DIR` (default `/etc/whitebox/clients`). A profile named
`example.conf` can then be selected with:

```sh
curl -sG http://localhost:9116/probe \
  --data-urlencode 'ctx=awgfile://example' \
  --data-urlencode 'target=https://www.gstatic.com/generate_204'
```

Profile names accept letters, digits, underscores, and hyphens only. The
service user needs read access to the directory and files. Restrict access
to the exporter to trusted monitoring clients: a caller can use a mounted
profile to probe destinations reachable through that VPN.

Each request creates a fresh tunnel and closes it after HTTP transport cleanup.
No kernel TUN device, network capabilities, or host route changes are required.
Target DNS is resolved through the tunnel using the profile's DNS IP addresses;
only a hostname in the VPN Endpoint uses the host resolver, under the probe
deadline. Profiles without DNS can probe literal IP targets. One peer per
profile is supported. Use a separate VPN client identity for monitoring.

Concurrent probes of the same private/public key pair are serialized to avoid
endpoint roaming. Other peers can run concurrently. Queueing, endpoint lookup,
handshake, and the HTTP request share the scope timeout, so allow enough time
for simultaneous targets. Native AWG probes do not use the Xray instance cache.

Additional per-probe gauges:

| Metric | Meaning |
| --- | --- |
| `tun_probe_awg_last_handshake_time_sec` | Unix timestamp of this tunnel's successful handshake, or zero |
| `tun_probe_awg_rx_bytes` | Bytes received during this fresh tunnel's lifetime |
| `tun_probe_awg_tx_bytes` | Bytes transmitted during this fresh tunnel's lifetime |

Byte values are gauges, not cumulative process counters. A successful handshake
alone does not imply `tun_probe_success=1`: the HTTP status and configured
response checks must also pass. Initialization errors return a non-2xx response;
monitor Prometheus `up` as well as `tun_probe_success`.
