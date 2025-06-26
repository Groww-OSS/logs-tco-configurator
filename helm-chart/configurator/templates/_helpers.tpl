{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "configurator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "configurator" }}
{{- end }}
{{- end }}


{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "configurator.namespace" -}}
{{- if .Values.namespaceOverride }}
{{- .Values.namespaceOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "kube-logging" }}
{{- end }}
{{- end }}


{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "configurator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}


{{/*
Selector labels
*/}}
{{- define "configurator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "configurator.fullname" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}


{{/*
Common labels
*/}}
{{- define "configurator.labels" -}}
helm.sh/chart: {{ include "configurator.chart" . }}
{{ include "configurator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}


{{- define "configurator.config.configmap.name" -}}
{{ include "configurator.fullname" . }}-config
{{- end }}


{{- define "configurator.config.mountPath" -}}
{{- default "/app/config" .Values.fileMounts.config.mountPath }}
{{- end }}


{{- define "configurator.budget.configmap.name" -}}
{{ include "configurator.fullname" . }}-budget
{{- end }}

{{- define "configurator.budget.mountPath" -}}
{{- default "/app/budget" .Values.fileMounts.budget.mountPath }}
{{- end }}
