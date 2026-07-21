# Architecture

Durable structure only. Anything with a date or a status belongs in an Issue.

## Two planes

**Control plane** (this repo) is serverless and free at rest. It holds identity,
RBAC, the catalog, budgets, and the audit trail. It is not VPC-attached.

**Compute plane** is ephemeral EC2 in the *user's* AWS account, launched and
reaped through spore.host. The portal holds a role in that account; the user
does not hold a role in the portal's.

## The two authentications

They authenticate different kinds of thing, and only one is a login.

| | Globus → portal | AWS account onboarding |
|---|---|---|
| Authenticates | a human | an account |
| Frequency | every session | once |
| Who does it | the researcher | often the campus cloud team |
| Mechanism | OIDC authorization code + PKCE | CloudFormation stack with a per-pair ExternalId |

The ExternalId is generated per (identity, account) pair. That someone deployed
a stack carrying it is itself evidence they held admin in that account at that
moment — and it gives standard confused-deputy protection.

## Identity

A Globus account holds a *set* of linked identities. Authorization keys on the
primary subject UUID, never the username string. The full set is recorded in the
session because a DUA attestation must say which institutional identity was
asserted, not merely which human was behind it.

## RBAC

Globus Groups are a **membership source**, not the authorization model. Globus
defines exactly three group roles — admin, manager, member — and has no research
projects. (Globus "Projects" at developers.globus.org register OAuth clients and
are unrelated; do not let that word leak into portal vocabulary.)

- **Project** is portal-native: budget, account binding, dataset grants, lease
  policy, audit trail. It *references* Globus group UUIDs.
- **Portal roles** are ours, assigned to an identity UUID or a group UUID, and
  synced to DynamoDB at login so the hot authorization path is one read.
- **Role derivation** from Globus roles is per-project and opt-in. Each portal
  role is flagged derivable or not. Membership administration and compute launch
  may be derived; approving a DUA or changing a budget ceiling may not. This is
  what stops a Globus group admin from manufacturing entitlement.
- Store the derivation (`role=X, source=derived-from-group-UUID`), not just the
  result, so "why does this person have access" is answerable.

## Two-key access

Group membership is *necessary*. For restricted tiers a portal-side grant — DUA
acceptance, PI approval, expiry — is *also* required. Open-tier data needs only
the first key. This is what lets open and restricted data share one catalog.

## Data movement

Portal storage reaches ephemeral compute over an S3 **gateway** endpoint (free)
with portal-issued STS session credentials, scoped to the project prefix and
session-tagged. Globus Connect Personal on the instance handles boundary
crossings — the user's campus collection, their laptop.

Transfers are submitted **by the portal**, holding the dependent Transfer token
server-side, with both endpoints checked against the project's allowlist. No
Transfer token is placed on the instance. That is what makes the allowlist
enforced rather than advisory.

## What this architecture does not claim

The session user has a shell. Exfiltration is not prevented — `curl`, an
unprivileged GCP install, or the DCV clipboard all work regardless of the
allowlist. The honest claim is **portal-mediated movement with a full audit
trail**. Stronger claims need egress filtering or a Nitro Enclave data path, and
neither is in scope while the catalog is de-identified.

Unprivileged sessions exist to protect the **lease machinery** — the budget
daemon, the endpoint registration — not to prevent data egress.
