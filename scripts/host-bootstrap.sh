#!/usr/bin/env bash
# ============================================================================
# Ironflyer — Hetzner host bootstrap.
#
# Run once on a fresh Ubuntu 24.04 LTS Hetzner dedicated (AX102/AX42) or
# cloud (CCX) server. Installs Docker + gVisor + sets up the /data NVMe
# tree + locks down SSH. Idempotent.
#
#   curl -fsSL https://raw.githubusercontent.com/zorba9172/ironflyer/main/scripts/host-bootstrap.sh \
#     | bash -s -- --role primary --domain ironflyer.ai
#
# Or from a checkout:
#   sudo bash scripts/host-bootstrap.sh --role primary --domain ironflyer.ai
#
# Roles:
#   primary  — runs docker-compose.prod.yml (the AX102)
#   standby  — same compose, but env IRONFLYER_ROLE=standby (the AX42)
#   stateless — orchestrator/web nodes (CCX23)
#   runtime  — runtime/sandbox nodes (CCX23)
# ============================================================================
set -euo pipefail

ROLE=""
DOMAIN=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --role)   ROLE="$2";   shift 2 ;;
    --domain) DOMAIN="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$ROLE" || -z "$DOMAIN" ]]; then
  echo "usage: $0 --role <primary|standby|stateless|runtime> --domain <name>" >&2
  exit 1
fi

log() { printf "\033[1;36m[bootstrap]\033[0m %s\n" "$*"; }
require_root() { [[ $EUID -eq 0 ]] || { echo "must run as root"; exit 1; }; }

require_root

# ---------------------------------------------------------------------------
# 1. Base OS hardening
# ---------------------------------------------------------------------------
log "updating apt + installing baseline tools"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq \
  ca-certificates curl gnupg lsb-release \
  ufw fail2ban unattended-upgrades \
  htop iotop iftop ncdu jq vim git tmux \
  zfsutils-linux

# Enable unattended security upgrades.
dpkg-reconfigure -f noninteractive unattended-upgrades

# ---------------------------------------------------------------------------
# 2. Firewall — public only on 22/80/443; everything else docker-internal.
# ---------------------------------------------------------------------------
log "configuring ufw"
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp comment 'ssh'
ufw allow 80/tcp comment 'http (acme + redirect)'
ufw allow 443/tcp comment 'https'
ufw allow 443/udp comment 'http/3'
# Private network (Hetzner vSwitch / Cloud network) — allow PG replication,
# Redis sentinel, MinIO mirror, k3s join (later).
ufw allow from 10.0.0.0/8 comment 'private mesh'
ufw --force enable

# ---------------------------------------------------------------------------
# 3. Sysctl + kernel tuning for high-fd, low-latency net, many containers.
# ---------------------------------------------------------------------------
log "applying sysctls"
cat >/etc/sysctl.d/99-ironflyer.conf <<'EOF'
# Network
net.core.somaxconn = 65535
net.core.netdev_max_backlog = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.ip_local_port_range = 10240 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15
net.ipv4.tcp_keepalive_time = 300
# File descriptors
fs.file-max = 2097152
fs.nr_open = 2097152
# vm — Postgres+ClickHouse want little swap
vm.swappiness = 10
vm.overcommit_memory = 1
vm.max_map_count = 262144
# Inotify (for promtail/Caddy hot-reload)
fs.inotify.max_user_watches = 524288
fs.inotify.max_user_instances = 8192
EOF
sysctl -p /etc/sysctl.d/99-ironflyer.conf

# ulimits
cat >/etc/security/limits.d/99-ironflyer.conf <<'EOF'
*    soft nofile 1048576
*    hard nofile 1048576
*    soft nproc  unlimited
*    hard nproc  unlimited
root soft nofile 1048576
root hard nofile 1048576
EOF

# ---------------------------------------------------------------------------
# 4. Docker (latest stable from Docker repo, not apt's distro version)
# ---------------------------------------------------------------------------
if ! command -v docker >/dev/null 2>&1; then
  log "installing docker"
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io \
    docker-buildx-plugin docker-compose-plugin
fi

# ---------------------------------------------------------------------------
# 5. gVisor (runsc) — sandbox isolation runtime for the per-user containers.
#    Skipped on stateless/primary roles where we don't run user code.
# ---------------------------------------------------------------------------
if [[ "$ROLE" == "primary" || "$ROLE" == "runtime" ]]; then
  if ! command -v runsc >/dev/null 2>&1; then
    log "installing gVisor runsc"
    ARCH=$(uname -m)
    URL="https://storage.googleapis.com/gvisor/releases/release/latest/${ARCH}"
    curl -fsSL "${URL}/runsc" -o /usr/local/bin/runsc
    curl -fsSL "${URL}/containerd-shim-runsc-v1" -o /usr/local/bin/containerd-shim-runsc-v1
    chmod +x /usr/local/bin/runsc /usr/local/bin/containerd-shim-runsc-v1
    runsc install
  fi
fi

# ---------------------------------------------------------------------------
# 6. Docker daemon config — enable gVisor runtime + json-file log limits +
#    enable metrics endpoint for vmagent scrape.
# ---------------------------------------------------------------------------
log "writing /etc/docker/daemon.json"
mkdir -p /etc/docker
cat >/etc/docker/daemon.json <<EOF
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m",
    "max-file": "5"
  },
  "default-ulimits": {
    "nofile": { "Name": "nofile", "Hard": 1048576, "Soft": 1048576 }
  },
  "live-restore": true,
  "metrics-addr": "127.0.0.1:9323",
  "experimental": true$([ "$ROLE" = "primary" ] || [ "$ROLE" = "runtime" ] && echo ',
  "runtimes": {
    "runsc": {
      "path": "/usr/local/bin/runsc",
      "runtimeArgs": ["--network=sandbox", "--platform=systrap"]
    }
  }')
}
EOF
systemctl restart docker

# ---------------------------------------------------------------------------
# 7. /data tree on the second NVMe (assumed mounted at /mnt/data already
#    via Hetzner installimage; if not, the next block creates it on /data).
# ---------------------------------------------------------------------------
log "preparing /data tree"
if [[ -d /mnt/data && ! -L /data ]]; then
  ln -sf /mnt/data /data
fi
mkdir -p /data/{postgres,redis,surrealdb,minio,redpanda,clickhouse,workspaces,grafana,loki,victoriametrics,vmagent,caddy/data,caddy/config}

# Postgres needs uid 999 (the official image's postgres user)
chown -R 999:999 /data/postgres
# Redis
chown -R 999:1000 /data/redis
# MinIO
chown -R 1000:1000 /data/minio
# Grafana
chown -R 472:0 /data/grafana
# Loki
chown -R 10001:10001 /data/loki

# ---------------------------------------------------------------------------
# 8. Hostname + role marker.
# ---------------------------------------------------------------------------
HOSTNAME_NEW="ironflyer-${ROLE}-$(hostname -s | sed 's/[^a-z0-9-]//g')"
hostnamectl set-hostname "$HOSTNAME_NEW"
echo "$ROLE" > /etc/ironflyer-role
echo "$DOMAIN" > /etc/ironflyer-domain

# ---------------------------------------------------------------------------
# 9. Time sync (chrony) — critical for TLS + Postgres replication.
# ---------------------------------------------------------------------------
apt-get install -y -qq chrony
systemctl enable --now chrony

# ---------------------------------------------------------------------------
# 10. Final report.
# ---------------------------------------------------------------------------
log "done. role=${ROLE} domain=${DOMAIN} hostname=${HOSTNAME_NEW}"
log "next:"
log "  cd /opt/ironflyer && git clone https://github.com/zorba9172/ironflyer.git ."
log "  cp infra/compose/.env.prod.example infra/compose/.env.prod"
log "  chmod 600 infra/compose/.env.prod"
log "  \$EDITOR infra/compose/.env.prod"
log "  docker compose -f infra/compose/docker-compose.prod.yml \\"
log "    --env-file infra/compose/.env.prod up -d"
