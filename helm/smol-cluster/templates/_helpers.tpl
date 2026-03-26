{{/*
Expand the name of the chart.
*/}}
{{- define "smol-gang.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "smol-gang.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "smol-gang.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "smol-gang.labels" -}}
helm.sh/chart: {{ include "smol-gang.chart" . }}
{{ include "smol-gang.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "smol-gang.selectorLabels" -}}
app.kubernetes.io/name: {{ include "smol-gang.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Gateway labels
*/}}
{{- define "smol-gang.gateway.labels" -}}
{{ include "smol-gang.labels" . }}
app.kubernetes.io/component: gateway
{{- end }}

{{/*
Gateway selector labels
*/}}
{{- define "smol-gang.gateway.selectorLabels" -}}
{{ include "smol-gang.selectorLabels" . }}
app.kubernetes.io/component: gateway
{{- end }}

{{/*
PostgreSQL labels
*/}}
{{- define "smol-gang.postgresql.labels" -}}
{{ include "smol-gang.labels" . }}
app.kubernetes.io/component: postgresql
{{- end }}

{{/*
PostgreSQL selector labels
*/}}
{{- define "smol-gang.postgresql.selectorLabels" -}}
{{ include "smol-gang.selectorLabels" . }}
app.kubernetes.io/component: postgresql
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "smol-gang.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "smol-gang.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Gateway fullname
*/}}
{{- define "smol-gang.gateway.fullname" -}}
{{- printf "%s-gateway" (include "smol-gang.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
PostgreSQL fullname
*/}}
{{- define "smol-gang.postgresql.fullname" -}}
{{- printf "%s-postgresql" (include "smol-gang.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Database URL construction
*/}}
{{- define "smol-gang.databaseURL" -}}
{{- if .Values.postgresql.embedded }}
{{- printf "postgres://%s:$(DATABASE_PASSWORD)@%s:%d/%s?sslmode=disable" .Values.postgresql.username (include "smol-gang.postgresql.fullname" .) (int .Values.postgresql.port) .Values.postgresql.database }}
{{- else }}
{{- .Values.postgresql.externalDatabaseURL }}
{{- end }}
{{- end }}

{{/*
Namespace to use - always respect --namespace flag (Release.Namespace)
*/}}
{{- define "smol-gang.namespace" -}}
{{- .Release.Namespace }}
{{- end }}
