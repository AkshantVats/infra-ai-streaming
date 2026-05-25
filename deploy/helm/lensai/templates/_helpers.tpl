{{- define "lensai.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "lensai.fullname" -}}
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

{{- define "lensai.labels" -}}
helm.sh/chart: {{ include "lensai.name" . }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "lensai.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "lensai.selectorLabels" -}}
app.kubernetes.io/name: {{ include "lensai.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "lensai.httpProbeTiming" -}}
{{- $probe := index . 0 -}}
{{- $defaults := index . 1 -}}
initialDelaySeconds: {{ default $defaults.initialDelaySeconds $probe.initialDelaySeconds }}
periodSeconds: {{ default $defaults.periodSeconds $probe.periodSeconds }}
timeoutSeconds: {{ default $defaults.timeoutSeconds $probe.timeoutSeconds }}
failureThreshold: {{ default $defaults.failureThreshold $probe.failureThreshold }}
{{- end }}

{{- define "lensai.execProbeTiming" -}}
{{- $probe := index . 0 -}}
{{- $defaults := index . 1 -}}
initialDelaySeconds: {{ default $defaults.initialDelaySeconds $probe.initialDelaySeconds }}
periodSeconds: {{ default $defaults.periodSeconds $probe.periodSeconds }}
timeoutSeconds: {{ default $defaults.timeoutSeconds $probe.timeoutSeconds }}
failureThreshold: {{ default $defaults.failureThreshold $probe.failureThreshold }}
{{- end }}
