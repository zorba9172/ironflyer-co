# Pulumi `compute/doks.go` + `data/*.go` — deprecated

**Status (2026-05-27):** the prod stack switched from DigitalOcean DOKS
+ Managed PG/Valkey/Spaces to Hetzner dedicated + Hetzner Cloud +
Compose-on-the-host. See [`DEPLOY.md`](../../DEPLOY.md) and
[`infra/compose/README.prod.md`](../compose/README.prod.md).

## Files frozen in this directory

- [`compute/doks.go`](compute/doks.go) — DOKS cluster + node pools
- [`data/postgres.go`](data/postgres.go) — DO Managed Postgres
- [`data/redis.go`](data/redis.go) — DO Managed Valkey
- [`data/spaces.go`](data/spaces.go) — DO Spaces bucket
- [`data/observability.go`](data/observability.go) — kube-prometheus +
  loki Helm releases
- [`edge/cloudflare.go`](edge/cloudflare.go),
  [`edge/cert_manager.go`](edge/cert_manager.go),
  [`edge/ingress.go`](edge/ingress.go),
  [`edge/sealed_secrets.go`](edge/sealed_secrets.go),
  [`edge/vercel.go`](edge/vercel.go) — DOKS edge wiring

## Replacement (TBD: `compute/hetzner.go`)

The Hetzner Cloud nodes + LB11 + private network + Cloudflare DNS get
provisioned by a slim program that uses:

- `github.com/pulumi/pulumi-hcloud` — Cloud servers, LB, network
- `github.com/pulumi/pulumi-cloudflare` — DNS A records
- cloud-init scripts that wget + run
  [`scripts/host-bootstrap.sh`](../../scripts/host-bootstrap.sh) at first boot

The dedicated AX102 + AX42 are ordered manually via Hetzner Robot (no
first-class Pulumi provider); their IPs go into `Pulumi.prod.yaml`.

Until `compute/hetzner.go` lands, the prod stack is brought up by
following `DEPLOY.md` manually. `pulumi up` will currently fail
because the legacy DOKS/PG/Valkey/Spaces resources are still wired in
`main.go` — comment them out before running, or pin the legacy stack
to a separate Pulumi project.
