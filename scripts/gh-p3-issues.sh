#!/usr/bin/env bash
# P3 — the lease. The phase where the project succeeds or fails.
# Design: docs/LEASE.md. Run after scripts/gh-bootstrap.sh.
set -euo pipefail
M="P3 The lease"

new() { gh issue create --title "$1" --label "$2" --milestone "$M" --body "$3" >/dev/null; echo "  + $1"; }

new "Lease record and state machine" \
"p3:lease,kind:feature" \
'The core type. See `docs/LEASE.md` for states.

- Single DynamoDB item; transitions are **conditional updates**, so concurrency needs no locking
- A transition that loses its condition check is a **no-op, not an error** — two reapers racing is normal and both must be able to run
- States: `Requested → Admitted → Provisioning → Active → Draining → Reaped`, plus `Denied` and `Failed`

**Acceptance:** a table-driven test drives every legal transition and asserts every illegal one is rejected; two concurrent `Draining` transitions both return success.'

new "Deadline math: collapse budget into wall clock" \
"p3:lease,concern:no-clocks" \
'The design decision that makes P3 cheap.

```
deadline = min(requested_expiry, started_at + remaining_budget / hourly_rate)
```

Cost Explorer lags hours and is useless for a ten-minute lease. But spend is a deterministic function of time — `truffle` knows the rate at launch and EC2 dominates the bill so completely that storage and transfer are rounding errors.

Recompute on any rate-changing event: spot price move, instance added or removed, budget adjustment. **Event-driven, never periodic.**

**Acceptance:** a lease with a $0.50 budget on a known hourly rate produces the arithmetically correct deadline, and adding a second instance halves the remaining time.'

new "EventBridge Scheduler one-shot per lease" \
"p3:lease,concern:no-clocks" \
'One schedule per lease, `at()` expression at the deadline, created with the lease and deleted with it. Target is the reaper Lambda.

No polling loop. No sweeper. No `rate()` schedule. If you find yourself writing something that runs every N minutes, the deadline math above is what you actually want.

Recomputing the deadline updates the schedule in place.

**Acceptance:** creating a lease creates exactly one schedule; reaping deletes it; `ListSchedules` is empty after the P3 gate.'

new "Resource ledger: intent before action" \
"p3:lease,concern:security" \
'The worst failure mode in this product is a **stranded instance in someone else`'"'"'`s account** — it costs a stranger money and destroys the trust the BYO-account model depends on.

So: write the ledger entry **before** creating each resource. A crash between intent and creation leaves a reapable record pointing at something that may not exist. Create-then-record leaves orphans nobody can find.

Reap must tolerate `NotFound` on every resource type and treat it as **success**.

Ledger covers: STS grant, security group and its session ingress rule, key material, instance, EBS volumes, Globus GCP endpoint.

**Acceptance:** a fault-injection test that kills the provisioner between ledger write and `RunInstances` still reaps cleanly.'

new "Cross-account substrate via cohort ports" \
"p3:lease,concern:spore" \
'Per the cohort decision record, `spawn/pkg/provider/ec2.go` operates **on-node via IMDS self-identity** — it is not an off-node launcher of externally-named instances, and the refactor explicitly must not turn it into one.

So off-node launching goes through `cohort`'"'"'`s `Actuator`/`Observer` ports, with an Arpeggio-side AWS substrate that assumes the cross-account role. Same shape as queryzero`'"'"'`s `internal/substrate/aws`.

`cohort` must still import nothing from the suite and no cloud SDK. That Arpeggio compiles against an **unmodified** cohort is the proof the abstraction is real — if cohort needs a change to accommodate a third consumer, that is a finding worth recording.

**Acceptance:** an instance launches into a second AWS account through the cohort ports, with `go.mod` pinning an unmodified cohort.'

new "Three independent teardown paths" \
"p3:lease,concern:security" \
'Teardown cannot depend on the control plane being able to reach the target account — the role may be revoked, the trust deleted, the region out.

1. **Portal-initiated** — Scheduler fires, Lambda assumes the role, walks the ledger. Normal path.
2. **On-node** — `spored` carries its own deadline and self-terminates. Requires `InstanceInitiatedShutdownBehavior: terminate`.
3. **EC2-native backstop** — a hard `shutdown` scheduled at boot for deadline plus margin. Survives `spored` dying.

Each must be idempotent and each must be sufficient alone.

**Acceptance:** three tests, each disabling two paths, all ending with no running instance.'

new "spored: deadline, heartbeat, self-terminate" \
"p3:lease,concern:spore" \
'`spored` work needed for the lease:

- accept a deadline at boot
- emit a heartbeat the portal observes to move `Provisioning → Active`
- self-terminate at its own deadline, independent of the portal
- report local accounting (uptime, instance type) for reconciliation

Design question to settle here: does the heartbeat go to the portal API, or does the portal poll EC2 status? The former is a clock on nothing but is one more inbound path; the latter is a poll. Prefer heartbeat-on-state-change plus portal poll only during `Provisioning`.

**Acceptance:** killing the portal`'"'"'`s reaper still leaves nothing running after the deadline.'

new "STS grant revocation" \
"p3:lease,concern:security" \
'Session credentials cannot be revoked directly. Two mechanisms, both used:

- **Short duration.** Cap `DurationSeconds` so an unrevoked credential expires on its own.
- **Inline deny.** On reap, attach a policy denying all actions where `aws:TokenIssueTime` is before now — the standard revoke-sessions pattern.

Record in the ADR that the deny policy is itself a resource needing cleanup, or it accumulates on the role.

**Acceptance:** a credential issued to a lease returns `AccessDenied` within seconds of reap, and the deny policy is cleaned up on the next lease.'

new "Admission control seam" \
"p3:lease,kind:feature" \
'Before anything is created, check: project budget remaining, caller`'"'"'`s portal role, dataset grants the request implies, onboarded account trust validity, instance quota.

Keep this behind a **narrow interface**. P7 makes the role and grant checks much richer and P3 should not have to be reopened when it does.

**Acceptance:** a request exceeding the project budget lands in `Denied` with a message naming which check failed — written for the researcher reading it, not the operator.'

new "Cost reconciliation, not cost enforcement" \
"p3:lease,kind:feature" \
'Tag every resource with lease ID, project, and owner. A post-hoc job compares actual Cost Explorer spend against predicted spend.

This is the audit trail and the calibration loop for the rate model. It is **never** the enforcement path — say so in the code comment, because the temptation to "just check the real cost" will recur.

**Acceptance:** predicted and actual agree within 5% across ten leases of varying length.'

new "P3 gate: the abandoned lease" \
"p3:lease,concern:no-clocks" \
'**The phase gate.**

Launch a ten-minute lease with a fifty-cent budget into a real second AWS account. Walk away.

One hour later, verify:
- no instance
- no EBS volume
- no security group or session ingress rule
- no Globus endpoint
- no EventBridge schedule
- lease in `Reaped`
- Cost Explorer agrees with the prediction

**Done when this passes. Not when the code is written.**'

echo "P3 issues created."
