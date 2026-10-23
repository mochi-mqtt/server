{{- define "mochi.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "mochi.fullname" -}}
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

{{- define "mochi.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{ include "mochi.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "mochi.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mochi.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "mochi.peers" -}}
{{ include "mochi.fullname" . }}-peers
{{- end }}

{{- define "mochi.image" -}}
{{- if .Values.image.digest -}}
{{ .Values.image.repository }}@{{ .Values.image.digest }}
{{- else -}}
{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}
{{- end }}
{{- end }}

{{- define "mochi.gossipSecret" -}}
{{- .Values.cluster.gossipKey.existingSecret | default (printf "%s-gossip" (include "mochi.fullname" .)) }}
{{- end }}

{{- define "mochi.authSecret" -}}
{{- .Values.auth.existingSecret | default (printf "%s-auth" (include "mochi.fullname" .)) }}
{{- end }}

{{- define "mochi.useAuthFile" -}}
{{- if or .Values.auth.existingSecret .Values.auth.ledger }}true{{ end }}
{{- end }}
