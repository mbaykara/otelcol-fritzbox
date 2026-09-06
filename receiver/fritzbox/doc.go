// Package fritzbox implements an OpenTelemetry Collector metrics receiver
// that scrapes AVM Fritz!Box routers via their TR-064 API.
package fritzbox

//go:generate go tool go.opentelemetry.io/collector/cmd/mdatagen metadata.yaml
