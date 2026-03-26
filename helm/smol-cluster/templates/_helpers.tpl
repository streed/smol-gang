{{/*
Expand the name of the chart.
*/}}
{{- define "smol-cluster.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "smol-cluster.fullname" -}}
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
{{- define "smol-cluster.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "smol-cluster.labels" -}}
helm.sh/chart: {{ include "smol-cluster.chart" . }}
{{ include "smol-cluster.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "smol-cluster.selectorLabels" -}}
app.kubernetes.io/name: {{ include "smol-cluster.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Gateway labels
*/}}
{{- define "smol-cluster.gateway.labels" -}}
{{ include "smol-cluster.labels" . }}
app.kubernetes.io/component: gateway
{{- end }}

{{/*
Gateway selector labels
*/}}
{{- define "smol-cluster.gateway.selectorLabels" -}}
{{ include "smol-cluster.selectorLabels" . }}
app.kubernetes.io/component: gateway
{{- end }}

{{/*
PostgreSQL labels
*/}}
{{- define "smol-cluster.postgresql.labels" -}}
{{ include "smol-cluster.labels" . }}
app.kubernetes.io/component: postgresql
{{- end }}

{{/*
PostgreSQL selector labels
*/}}
{{- define "smol-cluster.postgresql.selectorLabels" -}}
{{ include "smol-cluster.selectorLabels" . }}
app.kubernetes.io/component: postgresql
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "smol-cluster.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "smol-cluster.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Gateway fullname
*/}}
{{- define "smol-cluster.gateway.fullname" -}}
{{- printf "%s-gateway" (include "smol-cluster.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
PostgreSQL fullname
*/}}
{{- define "smol-cluster.postgresql.fullname" -}}
{{- printf "%s-postgresql" (include "smol-cluster.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Database URL construction
*/}}
{{- define "smol-cluster.databaseURL" -}}
{{- if .Values.postgresql.embedded }}
{{- printf "postgres://%s:$(DATABASE_PASSWORD)@%s:%d/%s?sslmode=disable" .Values.postgresql.username (include "smol-cluster.postgresql.fullname" .) (int .Values.postgresql.port) .Values.postgresql.database }}
{{- else }}
{{- .Values.postgresql.externalDatabaseURL }}
{{- end }}
{{- end }}

{{/*
Namespace to use - always respect --namespace flag (Release.Namespace)
*/}}
{{- define "smol-cluster.namespace" -}}
{{- .Release.Namespace }}
{{- end }}
