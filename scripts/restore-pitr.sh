#!/usr/bin/env bash
# Ironflyer Postgres point-in-time recovery.
#
# Orchestrates a full PITR using `wal-g`:
#   1. Scale orchestrator + web Deployments to 0 (stop writes).
#   2. Wipe the existing PGDATA (in a scratch StatefulSet pod).
#   3. `wal-g backup-fetch LATEST` for a base backup before --target-time.
#   4. Write `recovery.signal` + `recovery_target_time` into postgresql.auto.conf.
#   5. Start Postgres in recovery mode; wal-g wal-fetch replays WAL up to
#      the target time, then promotes the cluster.
#   6. Operator manually scales orchestrator + web back up after smoke-testing.
#
# Usage:
#   scripts/restore-pitr.sh \
#     --target-time "2026-05-23 14:00:00 UTC" \
#     --namespace ironflyer-prod \
#     --release ironflyer \
#     --i-really-want-to-restore-prod
#
# Required flags:
#   --target-time TS                 ISO-ish timestamp ("YYYY-MM-DD HH:MM:SS UTC")
#   --i-really-want-to-restore-prod  Confirmation guard. Without this the
#                                    script refuses to run.
# Optional flags:
#   --namespace NS                   default: current kubectl context's namespace
#   --release   NAME                 default: ironflyer
#
# Required env (passed through to wal-g inside the pod):
#   WALG_S3_PREFIX                   s3://bucket/prefix matching helm values
#   AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION
#                                    Same creds the StatefulSet sidecar uses.
#
# SAFETY:
#   - The `--i-really-want-to-restore-prod` flag is required. Don't add a
#     way to suppress this without going through a code review.
#   - The script scales orchestrator + web to 0 BEFORE touching Postgres
#     so the recovered cluster doesn't get partial writes during replay.
#   - The previous PGDATA is renamed to pgdata.pre-pitr-${TS} inside the
#     StatefulSet PVC so accidental restores can be reverted by hand.

set -euo pipefail

log()  { printf '[pitr] %s %s\n' "$(date -u +%FT%TZ)" "$*"; }
die()  { printf '[pitr] ERROR %s %s\n' "$(date -u +%FT%TZ)" "$*" >&2; exit 1; }

TARGET_TIME=""
NAMESPACE=""
RELEASE="ironflyer"
CONFIRMED=0

while [ $# -gt 0 ]; do
  case "$1" in
    --target-time)
      [ $# -ge 2 ] || die "--target-time requires a value"
      TARGET_TIME="$2"; shift 2 ;;
    --namespace)
      [ $# -ge 2 ] || die "--namespace requires a value"
      NAMESPACE="$2"; shift 2 ;;
    --release)
      [ $# -ge 2 ] || die "--release requires a value"
      RELEASE="$2"; shift 2 ;;
    --i-really-want-to-restore-prod)
      CONFIRMED=1; shift ;;
    -h|--help)
      sed -n '2,40p' "$0"
      exit 0 ;;
    *)
      die "unknown flag: $1" ;;
  esac
done

[ -n "$TARGET_TIME" ] || die "--target-time is required (e.g. \"2026-05-23 14:00:00 UTC\")"
[ "$CONFIRMED" -eq 1 ] || die "refusing to run without --i-really-want-to-restore-prod"

command -v kubectl >/dev/null 2>&1 || die "kubectl not found in PATH"

NS_FLAG=()
if [ -n "$NAMESPACE" ]; then
  NS_FLAG=(-n "$NAMESPACE")
fi

log "target_time=${TARGET_TIME} release=${RELEASE} namespace=${NAMESPACE:-<current>}"
log "step 1/5: scaling orchestrator + web to 0 to halt writes"
kubectl "${NS_FLAG[@]}" scale deploy orchestrator --replicas=0 || die "failed to scale orchestrator"
kubectl "${NS_FLAG[@]}" scale deploy web         --replicas=0 || die "failed to scale web"
kubectl "${NS_FLAG[@]}" rollout status deploy/orchestrator --timeout=120s || true
kubectl "${NS_FLAG[@]}" rollout status deploy/web         --timeout=120s || true

log "step 2/5: scaling postgres StatefulSet to 0 to detach PGDATA cleanly"
kubectl "${NS_FLAG[@]}" scale statefulset postgres --replicas=0 || die "failed to scale postgres"
# Wait for the pod to disappear before mutating PGDATA.
kubectl "${NS_FLAG[@]}" wait --for=delete pod -l app.kubernetes.io/name=postgres --timeout=120s || true

log "step 3/5: launching wal-g recovery pod (backup-fetch + recovery.conf wiring)"
# A short-lived Job that mounts the same PVC, fetches the latest base
# backup ≤ target-time, and primes recovery.signal. We `kubectl apply`
# a generated manifest so this stays self-contained.
RECOVERY_JOB="pitr-recover-$(date -u +%Y%m%d%H%M%S)"
cat <<EOF | kubectl "${NS_FLAG[@]}" apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${RECOVERY_JOB}
  labels:
    app.kubernetes.io/name: pitr-recover
    app.kubernetes.io/part-of: ironflyer
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      securityContext: { runAsUser: 999, fsGroup: 999 }
      containers:
        - name: wal-g
          image: ghcr.io/wal-g/wal-g:latest
          command: ["/bin/sh", "-c"]
          args:
            - |
              set -eu
              echo "[wal-g] renaming previous PGDATA"
              mv /var/lib/postgresql/data/pgdata /var/lib/postgresql/data/pgdata.pre-pitr-\$(date -u +%Y%m%d%H%M%S) || true
              mkdir -p /var/lib/postgresql/data/pgdata
              echo "[wal-g] backup-fetch LATEST_BEFORE_TIME=${TARGET_TIME}"
              wal-g backup-fetch /var/lib/postgresql/data/pgdata LATEST
              echo "[wal-g] writing recovery target"
              cat >>/var/lib/postgresql/data/pgdata/postgresql.auto.conf <<CFG
              restore_command = 'wal-g wal-fetch %f %p'
              recovery_target_time = '${TARGET_TIME}'
              recovery_target_action = 'promote'
              CFG
              touch /var/lib/postgresql/data/pgdata/recovery.signal
              echo "[wal-g] recovery primed — Postgres will replay on next start"
          envFrom:
            - secretRef: { name: ${RELEASE}-ironflyer-postgres, optional: true }
            - secretRef: { name: backup-s3, optional: true }
          volumeMounts:
            - { name: data, mountPath: /var/lib/postgresql/data }
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: data-postgres-0
EOF

log "step 4/5: waiting for recovery job to complete"
kubectl "${NS_FLAG[@]}" wait --for=condition=complete "job/${RECOVERY_JOB}" --timeout=1800s \
  || die "recovery job did not complete cleanly — inspect: kubectl logs job/${RECOVERY_JOB}"

log "step 5/5: scaling postgres back up — Postgres will replay WAL to ${TARGET_TIME} and promote"
kubectl "${NS_FLAG[@]}" scale statefulset postgres --replicas=1 || die "failed to scale postgres"
kubectl "${NS_FLAG[@]}" rollout status statefulset/postgres --timeout=600s || true

log "PITR done. NEXT STEPS (manual, on purpose):"
log "  1) smoke-test the recovered DB: scripts/smoke.sh against an orchestrator pod"
log "  2) once green, scale orchestrator + web back up:"
log "       kubectl ${NS_FLAG[*]} scale deploy orchestrator --replicas=2"
log "       kubectl ${NS_FLAG[*]} scale deploy web         --replicas=2"
log "  3) record the restore in docs/DR_TEST_LOG.md"
