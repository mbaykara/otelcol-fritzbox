{{/* Chart name, truncated to the DNS label limit. */}}
{{- define "otelcol-fritzbox.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Fully qualified app name. */}}
{{- define "otelcol-fritzbox.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "otelcol-fritzbox.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "otelcol-fritzbox.selectorLabels" -}}
app.kubernetes.io/name: {{ include "otelcol-fritzbox.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "otelcol-fritzbox.labels" -}}
helm.sh/chart: {{ include "otelcol-fritzbox.chart" . }}
{{ include "otelcol-fritzbox.selectorLabels" . }}
app.kubernetes.io/version: {{ .Values.image.tag | default .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "otelcol-fritzbox.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "otelcol-fritzbox.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "otelcol-fritzbox.image" -}}
{{- if .Values.image.digest }}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest }}
{{- else }}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}
{{- end }}
{{- end }}

{{/* Non-empty when the receiver gets credentials. Fails on half-set inline credentials. */}}
{{- define "otelcol-fritzbox.fritzboxCredentials" -}}
{{- $f := .Values.fritzbox }}
{{- if and (not $f.existingSecret) (ne (empty $f.username) (empty $f.password)) }}
{{- fail "fritzbox.username and fritzbox.password must be set together" }}
{{- end }}
{{- if or $f.existingSecret $f.username }}true{{ end }}
{{- end }}

{{/* Non-empty when the OTLP exporter uses basic auth. */}}
{{- define "otelcol-fritzbox.otlpBasicAuth" -}}
{{- $o := .Values.otlp }}
{{- if and $o.endpoint (eq $o.auth "basic") }}
{{- if and (not $o.existingSecret) (or (empty $o.username) (empty $o.password)) }}
{{- fail "otlp.auth=basic needs otlp.existingSecret or both otlp.username and otlp.password (set otlp.auth=none for an unauthenticated endpoint)" }}
{{- end }}
{{- true }}
{{- end }}
{{- end }}

{{/* Non-empty when the chart must create its own Secret. */}}
{{- define "otelcol-fritzbox.createSecret" -}}
{{- if or (and (not .Values.fritzbox.existingSecret) .Values.fritzbox.username) (and (include "otelcol-fritzbox.otlpBasicAuth" .) (not .Values.otlp.existingSecret)) }}true{{ end }}
{{- end }}

{{- define "otelcol-fritzbox.hasCA" -}}
{{- if and .Values.fritzbox.tls.caSecret .Values.fritzbox.tls.caConfigMap }}
{{- fail "set only one of fritzbox.tls.caSecret and fritzbox.tls.caConfigMap" }}
{{- end }}
{{- if or .Values.fritzbox.tls.caSecret .Values.fritzbox.tls.caConfigMap }}true{{ end }}
{{- end }}

{{/* Collector configuration generated from values, before .Values.config is merged. */}}
{{- define "otelcol-fritzbox.baseConfig" -}}
{{- $basic := include "otelcol-fritzbox.otlpBasicAuth" . }}
{{- $f := .Values.fritzbox }}
extensions:
  health_check:
    endpoint: ${env:MY_POD_IP}:13133
  {{- if $basic }}
  basicauth/otlp:
    client_auth:
      username: ${env:OTLP_USERNAME}
      password: ${env:OTLP_PASSWORD}
  {{- end }}
receivers:
  fritzbox:
    endpoint: {{ $f.endpoint | quote }}
    {{- if include "otelcol-fritzbox.fritzboxCredentials" . }}
    username: ${env:FRITZBOX_USERNAME}
    password: ${env:FRITZBOX_PASSWORD}
    {{- end }}
    collection_interval: {{ $f.collectionInterval }}
    timeout: {{ $f.timeout }}
    {{- if or (include "otelcol-fritzbox.hasCA" .) $f.tls.serverNameOverride }}
    tls:
      {{- if include "otelcol-fritzbox.hasCA" . }}
      ca_file: /etc/otelcol-fritzbox/tls/{{ $f.tls.caKey }}
      {{- end }}
      {{- with $f.tls.serverNameOverride }}
      server_name_override: {{ . | quote }}
      {{- end }}
    {{- end }}
    {{- with $f.metrics }}
    metrics:
      {{- toYaml . | nindent 6 }}
    {{- end }}
processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: {{ .Values.memoryLimiter.limitPercentage }}
    spike_limit_percentage: {{ .Values.memoryLimiter.spikeLimitPercentage }}
  batch: {}
exporters:
  {{- if .Values.otlp.endpoint }}
  otlp_http:
    endpoint: {{ .Values.otlp.endpoint | quote }}
    {{- if $basic }}
    auth:
      authenticator: basicauth/otlp
    {{- end }}
  {{- else }}
  debug:
    verbosity: basic
  {{- end }}
service:
  extensions:
    - health_check
    {{- if $basic }}
    - basicauth/otlp
    {{- end }}
  telemetry:
    metrics:
      readers:
        - pull:
            exporter:
              prometheus:
                host: ${env:MY_POD_IP}
                port: 8888
  pipelines:
    metrics:
      receivers: [fritzbox]
      processors: [memory_limiter, batch]
      exporters: [{{ if .Values.otlp.endpoint }}otlp_http{{ else }}debug{{ end }}]
{{- end }}

{{- define "otelcol-fritzbox.config" -}}
{{- $base := include "otelcol-fritzbox.baseConfig" . | fromYaml }}
{{- mergeOverwrite $base (deepCopy .Values.config) | toYaml }}
{{- end }}
