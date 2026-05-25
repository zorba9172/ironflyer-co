{{/*
Image reference resolver. Inputs: name (service name), Values (chart values).
Repository defaults to `ironflyer-<name>`; tag defaults to .Values.imageTag.
*/}}
{{- define "ironflyer.image" -}}
{{- $svc := index .Values.images .name -}}
{{- $repo := default (printf "ironflyer-%s" .name) $svc.repository -}}
{{- $tag  := default .Values.imageTag $svc.tag -}}
{{- printf "%s/%s:%s" .Values.imageRegistry $repo $tag -}}
{{- end -}}

{{/*
Chart fullname. The chart is single-instance per namespace (services keep
their canonical names `orchestrator`, `runtime`, `web`, `postgres`), so
we just expose the release-aware fullname for cross-cutting resources
(PriorityClass, namespace-scoped policies, etc.).
*/}}
{{- define "ironflyer.fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ironflyer.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{/*
Common labels applied to every object the chart owns. `ironflyer.io/region`
makes multi-cluster GitOps queries + DR drills trivial — every kubectl
get -l ironflyer.io/region=eu returns exactly the EU release's resources.
*/}}
{{- define "ironflyer.labels" -}}
app.kubernetes.io/part-of: ironflyer
app.kubernetes.io/managed-by: {{ .Release.Service | default "helm" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
helm.sh/chart: {{ include "ironflyer.chart" . }}
ironflyer.io/region: {{ .Values.region | default "unknown" | quote }}
{{- end -}}

{{/*
Per-service selector labels. Inputs: name, release.
*/}}
{{- define "ironflyer.selectorLabels" -}}
app.kubernetes.io/name: {{ .name }}
app.kubernetes.io/instance: {{ .release }}
{{- end -}}

{{/*
ServiceAccount name (per-service). Inputs: name (service name), .
*/}}
{{- define "ironflyer.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- printf "%s-%s" (include "ironflyer.fullname" .) .name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
PriorityClass name shared by all critical workloads.
*/}}
{{- define "ironflyer.priorityClassName" -}}
{{- if .Values.priorityClass.create -}}
{{- printf "%s-critical" (include "ironflyer.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- default "" .Values.priorityClass.name -}}
{{- end -}}
{{- end -}}

{{/*
Pod-level securityContext defaults used by every workload.
*/}}
{{- define "ironflyer.podSecurityContext" -}}
runAsNonRoot: {{ .Values.securityContext.runAsNonRoot }}
runAsUser: {{ .Values.securityContext.runAsUser }}
runAsGroup: {{ .Values.securityContext.runAsUser }}
fsGroup: {{ .Values.securityContext.runAsUser }}
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{/*
Container-level securityContext defaults. Aligns with the PodSecurity
`restricted` admission baseline: nonroot, drop ALL caps, no privilege
escalation, read-only rootfs, RuntimeDefault seccomp. Workloads that
need a writable root (e.g. runtime workspace driver) override
readOnlyRootFilesystem inline.
*/}}
{{- define "ironflyer.containerSecurityContext" -}}
runAsNonRoot: {{ .Values.securityContext.runAsNonRoot }}
runAsUser: {{ .Values.securityContext.runAsUser }}
allowPrivilegeEscalation: {{ .Values.securityContext.allowPrivilegeEscalation }}
readOnlyRootFilesystem: {{ .Values.securityContext.readOnlyRootFilesystem }}
capabilities:
  drop: ["ALL"]
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{/*
Pod anti-affinity + topology spread template. Inputs: name (service name).
*/}}
{{- define "ironflyer.spread" -}}
topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: kubernetes.io/hostname
    whenUnsatisfiable: ScheduleAnyway
    labelSelector:
      matchLabels:
        app.kubernetes.io/name: {{ .name }}
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          topologyKey: kubernetes.io/hostname
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: {{ .name }}
{{- end -}}

{{/*
Postgres Secret name. When .Values.postgres.existingSecret is set, the chart
references that external Secret verbatim; otherwise we mint one named
`<fullname>-postgres`. Keys used downstream: `postgres-password` and
`POSTGRES_URL` (full DSN). When you BYO the secret it must expose both keys
under the same names.
*/}}
{{- define "ironflyer.postgresSecretName" -}}
{{- if .Values.postgres.existingSecret -}}
{{- .Values.postgres.existingSecret -}}
{{- else -}}
{{- printf "%s-postgres" (include "ironflyer.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Sentry Secret name. When .Values.sentry.existingSecret is set, references it
verbatim; otherwise mints `<fullname>-sentry` (only rendered when
sentry.createSecret=true). Expected keys: SENTRY_DSN, RUNTIME_SENTRY_DSN,
NEXT_PUBLIC_SENTRY_DSN. Empty values mean "Sentry disabled" — each service's
SDK no-ops when its env var is empty.
*/}}
{{- define "ironflyer.sentrySecretName" -}}
{{- if .Values.sentry.existingSecret -}}
{{- .Values.sentry.existingSecret -}}
{{- else -}}
{{- printf "%s-sentry" (include "ironflyer.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Whether the orchestrator/runtime should wire Sentry env-from-secretRef. True
when either an existing Sentry secret was supplied, or the chart was asked
to mint one. False means skip the env entries entirely so an unconfigured
chart doesn't reference a non-existent Secret.
*/}}
{{- define "ironflyer.sentryEnabled" -}}
{{- if or .Values.sentry.existingSecret .Values.sentry.createSecret -}}
true
{{- end -}}
{{- end -}}

{{/*
Annotations applied to pods that should be scraped + traced.
*/}}
{{- define "ironflyer.podAnnotations" -}}
prometheus.io/scrape: "true"
prometheus.io/path: "/metrics"
prometheus.io/port: "8080"
{{- if .Values.otel.enabled }}
sidecar.opentelemetry.io/inject: "true"
instrumentation.opentelemetry.io/inject-sdk: "true"
{{- end }}
{{- end -}}
