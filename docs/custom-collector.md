# Using the receiver in your own collector

Add it to your [OCB](https://opentelemetry.io/docs/collector/custom-collector/)
manifest — `gomod` is the Go module, `import` is the receiver package:

```yaml
receivers:
  - gomod: github.com/mbaykara/otelcol-fritzbox v0.2.0
    import: github.com/mbaykara/otelcol-fritzbox/receiver/fritzbox
```

Then wire it into a metrics pipeline:

```yaml
service:
  pipelines:
    metrics:
      receivers: [fritzbox]
      processors: [batch]
      exporters: [otlp]
```
