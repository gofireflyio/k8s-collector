{{- define "fireFlyCredentialsSecret" -}}
{{- if .Values.existingFireFlyCredentialsSecret -}}
{{- printf "%s" .Values.existingFireFlyCredentialsSecret }}
{{- else -}}
{{- printf "%s-%s" (.Release.Name | trunc ((sub 63 (len "-credentials")) | int) | trimSuffix "-") "credentials" }}
{{- end -}}
{{- end -}}

{{- define "fireFlyCollectorCrojobName"}}
{{- printf "%s-%s" (.Release.Name | trunc ((sub 52 (len "-cronjob")) | int) | trimSuffix "-") "cronjob" }}
{{- end -}}

{{- define "fireFlyCollectorContaierName"}}
{{- (.Release.Name | trunc 63 | trimSuffix "-") }}
{{- end -}}

{{- define "fireFlyArgocdContainerName"}}
{{- printf "%s-%s" (.Release.Name | trunc ((sub 53 (len "-argocd")) | int) | trimSuffix "-") "argocd" }}
{{- end -}}

{{- define "fireFlyArgoConfigMap"}}
{{- printf "%s-%s" (.Release.Name | trunc ((sub 63 (len "-argocd-config")) | int) | trimSuffix "-") "argocd-config" }}
{{- end -}}

{{- define "fireFlyCollectorConfigMap"}}
{{- printf "%s-%s" (.Release.Name | trunc ((sub 63 (len "-config")) | int) | trimSuffix "-") "config" }}
{{- end -}}

{{- define "fireFlyCollectorOffBoarderJob"}}
{{- printf "%s-%s" (.Release.Name | trunc ((sub 53 (len "-collector-off-boarder")) | int) | trimSuffix "-") "collector-off-boarder" }}
{{- end -}}

{{- define "fireFlyCollectorOnBoarderJob"}}
{{- printf "%s-%s" (.Release.Name | trunc ((sub 53 (len "-collector-on-boarder")) | int) | trimSuffix "-") "collector-on-boarder" }}
{{- end -}}

{{- define "fireFlyArgocdCredentials" -}}
{{- if .Values.argocd.secretNameOverride -}}
{{- printf "%s" .Values.argocd.secretNameOverride }}
{{- else -}}
{{- printf "%s-%s" (.Release.Name | trunc ((sub 63 (len "-argocd-credentials")) | int) | trimSuffix "-") "argocd-credentials" }}
{{- end -}}
{{- end -}}

{{- define "validateClusterId" -}}
{{- if not .Values.clusterId -}}
{{- fail "clusterId value is required and cannot be empty" -}}
{{- end -}}
{{- if not (regexMatch "^[a-z0-9-_]+$" .Values.clusterId) -}}
{{- fail "clusterId value must contain only must contain only lowercase letters, numbers, hyphens, and underscores." -}}
{{- end -}}
{{- end -}}
