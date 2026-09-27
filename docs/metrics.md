# Metrics

| Metric | Type | Unit | Description |
|---|---|---|---|
| `hw.network.io` | Sum (monotonic) | By | WAN bytes received/transmitted (`network.io.direction`) |
| `hw.network.packets` | Sum (monotonic) | {packet} | WAN + WLAN packets (`hw.id`, `network.io.direction`) |
| `hw.network.bandwidth.limit` | Gauge | By/s | WAN link speed |
| `hw.network.bandwidth.utilization` | Gauge | 1 | WAN bandwidth utilization fraction |
| `hw.network.up` | Gauge | 1 | Link status (WAN + per WLAN radio) |
| `hw.errors` | Sum (monotonic) | {error} | DSL line errors (`error.type=fec/crc/hec`, direction) |
| `fritzbox.device.uptime` | Gauge | s | Device uptime |
| `fritzbox.wan.connection.status` | Gauge | 1 | 1 = Connected |
| `fritzbox.wan.connection.uptime` | Gauge | s | WAN connection uptime |
| `fritzbox.wan.external_ip` | Gauge | 1 | **Opt-in.** External IP in attribute |
| `fritzbox.dsl.rate.current` / `.rate.max` | Gauge | bit/s | DSL sync/max rate per direction |
| `fritzbox.dsl.noise_margin` / `.attenuation` | Gauge | dB | DSL SNR/attenuation per direction |
| `fritzbox.dsl.error_seconds` | Sum (monotonic) | s | Errored/severely-errored seconds |
| `fritzbox.wlan.channel` | Gauge | 1 | WLAN channel per radio (`hw.id`, `ssid`) |
| `fritzbox.wlan.clients` | Gauge | {client} | Associated clients per radio |
| `fritzbox.hosts.total` | Gauge | {host} | Known hosts |
| `fritzbox.hosts.active` | Gauge | {host} | **Opt-in.** Active hosts (N extra SOAP calls) |
| `fritzbox.hosts.info` | Gauge | 1 | **Opt-in.** Per-device identity: hostname, ip, mac, interface_type, active, guest, friendly_name |

Resource attributes: `hw.vendor=AVM`, `hw.model`, `hw.serial_number`,
`fritzbox.device.software_version`, `server.address`.

Cable/fiber boxes without a DSL service simply skip the DSL group; boxes
without WLAN skip WLAN metrics.

## Tested devices

| Model | FRITZ!OS | Connection | Verified groups | Notes |
|---|---|---|---|---|
| FRITZ!Box 7590 (HW 226) | 8.25 | VDSL | device, wan traffic/link, dsl, wlan (3 radios), hosts, host info | WAN connection status/uptime unavailable (service faults); utilization requires extra permission scope |

Expected to work but **not verified**: other 75xx/56xx/66xx models on
FRITZ!OS 7.5+, cable and fiber variants (DSL group auto-skips), and
Fritz!Repeater devices (subset: device, wlan, hosts). Reports welcome.
