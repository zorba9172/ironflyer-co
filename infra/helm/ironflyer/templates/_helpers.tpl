{{- define "ironflyer.image" -}}
{{- $svc := index .Values.images .name -}}
{{- $repo := default (printf "ironflyer-%s" .name) $svc.repository -}}
{{- $tag  := default .Values.imageTag $svc.tag -}}
{{- printf "%s/%s:%s" .Values.imageRegistry $repo $tag -}}
{{- end -}}

{{- define "ironflyer.labels" -}}
app.kubernetes.io/part-of: ironflyer
app.kubernetes.io/managed-by: helm
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "ironflyer.selectorLabels" -}}
app.kubernetes.io/name: {{ .name }}
app.kubernetes.io/instance: {{ .release }}
{{- end -}}
