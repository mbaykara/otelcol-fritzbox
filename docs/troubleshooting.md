# Troubleshooting

**`context deadline exceeded` on discovery** — the box is unreachable (wrong
endpoint, wrong network) or DNS for `fritz.box` resolves to a public parked
address (some DNS setups hijack it). Use the box's IP directly.

**A metric group is missing entirely** — the device doesn't offer that
service (e.g. no `WANDSLInterfaceConfig` on cable/fiber boxes) or the action
faults. The log contains a one-time `warn` per skipped group naming the
service. This is a device capability difference, not an exporter failure.

**`UPnP error 401: Invalid Action` for WAN connection metrics** — some
firmware/WAN-mode combinations advertise `WANIPConnection` but don't
implement its actions. The group is skipped; everything else keeps working.

**Router down vs exporter down** — if the router is unreachable, the
collector logs `Error scraping metrics ... fetching device description` once
per interval and emits nothing. The collector process itself stays up, the
`health_check` extension keeps answering on `:13133`, and self-telemetry is
served on `:8888`. The receiver retries discovery on every scrape and resumes
when the router is back. If the collector process is down, no logs and no
self-telemetry.

**One group fails, others keep reporting** — a failing group is reported as a
partial scrape error (`Error scraping metrics` with the group named) and the
remaining metrics of that interval are still exported.
