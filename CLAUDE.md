# CLAUDE.md

Operating instructions for Claude Code in this repo. **Coding conventions and guardrails only.**
Project management does not live here — it lives in GitHub (see *Project management*).

## What this is

`arpeggio` (binary `arp`) is an AWS-native, browser-first research computing portal. Users
authenticate with Globus Auth (InCommon-federated), browse a de-identified data catalog, and
launch **ephemeral, budgeted, time-bound** compute **into their own AWS account**.

The portal never runs the compute. It brokers it.

```
arpeggio                     control plane: web, identity, RBAC, catalog, budgets  ← this repo
    └── spore.host (libs)    truffle (discovery), spawn (launch + lifecycle),
                             spored (on-node daemon), lagotto (capacity),
                             cohort (gang scheduling), DCV support
        └── AWS
```

## Golden rules (do not violate)

1. **Two planes, two cost rules.**
   - **Control plane = NO CLOCKS.** The portal must cost ~$0 at rest. Lambda + API Gateway
     HTTP API + DynamoDB (**on-demand only**) + S3/CloudFront + EventBridge + Step Functions.
   - **Compute plane = ephemeral and metered.** Every instance has a lease: an owner, a
     budget, an expiry, and a teardown path. Nothing outlives its lease.

2. **No clocks means no clocks.** Before adding any resource, check it bills per-use:
   - **Banned without an explicit ADR:** NAT Gateway, ALB/NLB, EKS control plane, provisioned
     DynamoDB capacity, OpenSearch, RDS, WAF, customer-managed KMS keys, Secrets Manager,
     interface VPC endpoints.
   - **Use instead:** public subnet + zero inbound, S3 **gateway** endpoint (free), SSM
     Parameter Store SecureString, AWS-managed KMS, session-scoped security group rules.

3. **Lambda stays out of the VPC.** It talks to STS, EC2, DynamoDB, and Globus over public
   AWS endpoints. A VPC-attached Lambda needs NAT or interface endpoints — that is how a
   serverless design quietly acquires a monthly floor. Only ephemeral compute is in a VPC.

4. **Globus is a membership and movement source, not the authorization model.**
   Projects, roles, grants, and budgets are portal-native (DynamoDB). Globus Groups are
   synced at login. Portal roles marked non-derivable may **never** be assigned from a
   Globus role mapping.

5. **No user Transfer token on the instance.** All Globus transfers are submitted by the
   portal, holding the dependent token server-side, with both endpoints checked against the
   project's allowlist. This is what makes the allowlist enforced rather than advisory.

6. **Endpoint lifecycle is lease lifecycle.** A GCP endpoint created for a session is deleted
   at teardown, alongside the instance, the SG rule, and the STS grant.

7. **Structured logging is not built yet, and nothing may foreclose it.** Thread a
   `context.Context` through every call path. Never log with the stdlib default logger, never
   `fmt.Println` in library code. When `slog` lands it should be a wiring change, not a
   refactor.

## Go conventions

- Go 1.24+. `internal/` for everything not intended for external import.
- Errors wrapped with `%w` and context: `fmt.Errorf("assume role in %s: %w", acct, err)`.
- Interfaces defined by the **consumer**, not the producer. `internal/auth` declares the
  narrow Globus surface it needs; the SDK adapter satisfies it.
- No global state. Dependencies are struct fields, injected at construction.
- Table-driven tests. `go test ./... -race` must pass before any PR.
- One exported type per file where practical; file named for the type.

## The Globus SDK seam

Scott maintains a Go port of the Globus SDK. This repo must **not** call Globus HTTP APIs
directly outside `internal/auth/globus_http.go`, which exists only as a stand-in until the
SDK is wired. The `Directory` and `Transfers` interfaces in `internal/auth` are the contract.
Replacing the stand-in with the SDK adapter must require no changes outside that package.

## Project management

**GitHub only.** Do not create local status files, roadmaps, sprint notes, or `TODO.md`.

- Work is an **Issue**, grouped by **Milestone** (P0…P7), tracked on the **Project** board.
- `docs/` holds **architecture and decisions only** — things true across phases. Anything
  with a date or a status goes in an Issue.
- Decisions that constrain future work get an ADR in `docs/adr/NNNN-title.md`.
- Branches: `p0/short-description`. Commits: conventional (`feat:`, `fix:`, `docs:`).
- Every PR references its Issue and states which golden rule it might stress.

## Definition of done for a phase

A phase is done when it has a demoable path **and** the test named in its milestone
description passes against real AWS. Not when the code is written.
