{{- define "fireFlyCredentialsSecret" -}}
{{- if .Values.existingFireFlyCredentialsSecret -}}
{{- printf "%s" .Values.existingFireFlyCredentialsSecret }}
{{- else -}}
{{- printf "%s-credentials" .Release.Name -}}
{{- end -}}
{{- end -}}
