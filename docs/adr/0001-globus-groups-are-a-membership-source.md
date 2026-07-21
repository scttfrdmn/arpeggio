# ADR 0001 — Globus Groups are a membership source, not the authorization model

**Status:** accepted

## Context

Globus Groups are hierarchical and federated, already exist at many campuses,
and are administered by PIs rather than by us. That is genuinely valuable and
hard to replicate.

But Globus defines exactly three group roles — admin, manager, member — and has
no concept of a research project. A portal needs budgets, dataset grants, lease
policy, DUA state, and an audit trail, none of which a group can hold.

## Decision

Projects, roles, and grants are portal-native (DynamoDB). Globus Groups are
synced at login and referenced by projects.

The three Globus roles govern **membership administration** — who can add and
remove people. What that membership entitles stays portal-side.

Role derivation from Globus roles is offered, per-project and opt-in, but each
portal role carries a `derivable` flag. Roles that grant spend or restricted-data
access are never derivable.

## Consequences

- A Globus group admin can shape the team but cannot manufacture entitlement.
- Open-tier projects can run with zero portal-side administration: the PI makes
  a group, adds people, and it works.
- Derived role changes happen out-of-band in Globus, so a sync that changes a
  derived role must emit an audit event, or access will shift on a Tuesday with
  nothing explaining why.
- Persisting the derivation source (not just the resulting role) is required to
  distinguish a stale derived role from a deliberate portal-side grant.

## Alternatives rejected

**Model projects as special Globus groups.** Rejected: a project has attributes
Globus cannot hold, so budget state would end up encoded in group names.

**Key entitlement directly on group membership.** Rejected for restricted data:
it makes whoever administers the group an authority over portal access.
