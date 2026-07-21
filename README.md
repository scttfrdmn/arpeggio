# Arpeggio

A browser-first research computing portal. Sign in with your institutional
identity, browse de-identified data, and launch **ephemeral, budgeted,
time-bound** compute **into your own AWS account**.

The portal never runs your compute. It brokers it.

```
arpeggio                     control plane: web, identity, RBAC, catalog, budgets
    └── spore.host (libs)    truffle · spawn · spored · lagotto · cohort · DCV
        └── AWS
```

## Why it is built this way

**No clocks.** The control plane costs approximately nothing at rest. Lambda,
API Gateway HTTP API, DynamoDB on-demand, S3, CloudFront, EventBridge — all
per-use. No NAT Gateway, no load balancer, no cluster, no idle capacity. The
floor is a Route53 hosted zone and a fortnight of logs.

**Bring your own account.** A researcher at one institution can use a portal
run by another, with compute billed to a third party's AWS account — their own.
Globus Auth federates the identity through InCommon; the AWS side is
institution-agnostic. No bilateral agreements.

**Every workload has a lease.** An owner, a budget, an expiry, and a teardown
path. Nothing outlives its lease — not the instance, not the security group
rule, not the credential grant, not the Globus endpoint.

## Status

**P0 — walking skeleton.** Globus login end to end. No data, no compute yet.
Phases are tracked as GitHub milestones; there is no roadmap file here on purpose.

## Running it

```sh
make tidy          # resolve dependencies
make check         # vet, gofmt, tests
make build         # linux/arm64 Lambda binary
```

Local development wants a real Globus client and a DynamoDB table:

```sh
export ARP_GLOBUS_CLIENT_ID=...
export ARP_GLOBUS_CLIENT_SECRET=...
export ARP_GLOBUS_REDIRECT_URL=http://localhost:8080/api/auth/callback
export ARP_PUBLIC_BASE_URL=http://localhost:8080
export ARP_TABLE_NAME=arpeggio-dev
go run ./cmd/arpd
```

Deployment lives in `deploy/template.yaml`. Read it before you run it — every
resource is chosen for its billing model, and the comments say why.

## Repository conventions

- `CLAUDE.md` is the coding contract. Read it before changing anything.
- `docs/` holds architecture and decisions only. Anything with a date or a
  status belongs in a GitHub Issue.
- `scripts/gh-bootstrap.sh` creates the labels, milestones, and opening issues.

## License

Apache 2.0. Copyright 2026 Scott Friedman.
