{{/*
Expand the name of the chart.
*/}}
{{- define "token-tumbler.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "token-tumbler.fullname" -}}
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

{{/*
Validate probe settings. Probes use the HTTP /healthz endpoint.
*/}}
{{- define "token-tumbler.validateProbes" -}}
{{- if or .Values.startupProbe.useExec .Values.livenessProbe.useExec }}
{{- fail "exec probes are not supported; keep useExec=false and enable metrics for HTTP /healthz probes" }}
{{- end }}
{{- if and (not .Values.metrics.enabled) (or .Values.startupProbe.enabled .Values.livenessProbe.enabled) }}
{{- fail "startupProbe/livenessProbe require metrics.enabled=true so /healthz is available" }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "token-tumbler.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "token-tumbler.labels" -}}
helm.sh/chart: {{ include "token-tumbler.chart" . }}
{{ include "token-tumbler.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "token-tumbler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "token-tumbler.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "token-tumbler.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "token-tumbler.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Validate values that could cause unsafe concurrent token rotations.
*/}}
{{- define "token-tumbler.validateReplicaSafety" -}}
{{- if not .Values.leaderElection.enabled }}
{{- if and (not .Values.autoscaling.enabled) (gt (int .Values.replicaCount) 1) }}
{{- fail "replicaCount must be 1 unless leaderElection.enabled is true" }}
{{- end }}
{{- if and .Values.autoscaling.enabled (gt (int .Values.autoscaling.maxReplicas) 1) }}
{{- fail "autoscaling.maxReplicas must be 1 unless leaderElection.enabled is true" }}
{{- end }}
{{- end }}
{{- end }}
