#!/usr/bin/env bash
# Create the labels and milestones this repo's process depends on, plus the two
# P1 issues that don't yet have a per-phase script. Per-phase issues live in
# scripts/gh-p<N>-issues.sh; this script deliberately does NOT create P0 issues.
# Requires: gh CLI, authenticated, run from the repo root.
#
# Project management lives in GitHub. Do not mirror any of this into local
# markdown (CLAUDE.md, "Project management").
set -euo pipefail

repo="$(gh repo view --json nameWithOwner -q .nameWithOwner)"
echo "Bootstrapping ${repo}"

label() { gh label create "$1" --color "$2" --description "$3" --force >/dev/null; echo "  label  $1"; }

# Phase labels
label "p0:skeleton"    "1D3F5C" "Globus login end to end"
label "p1:catalog"     "1D3F5C" "Dataset catalog, read only"
label "p2:onboarding"  "1D3F5C" "BYO AWS account trust"
label "p3:lease"       "8B2E2E" "Ephemeral compute lease, budget, teardown"
label "p4:datapath"    "1D3F5C" "STS grants and S3 gateway endpoint"
label "p5:dcv"         "1D3F5C" "Interactive session"
label "p6:globus"      "1D3F5C" "Portal brokered transfers"
label "p7:rbac"        "1D3F5C" "Projects, roles, budgets in depth"

# Kind
label "kind:feature"   "2F6E5C" "New capability"
label "kind:bug"       "A8321C" "Something is wrong"
label "kind:chore"     "6B7280" "Maintenance, deps, CI"
label "kind:adr"       "6D4AA8" "Architectural decision record"
label "kind:spike"     "A8681C" "Time boxed investigation"

# Cross-cutting concerns worth being able to filter on
label "concern:no-clocks"  "A8681C" "Touches the zero-idle cost guarantee"
label "concern:security"   "8B2E2E" "Trust boundary, credentials, or data access"
label "concern:globus"     "2F6E5C" "Globus Auth, Groups, or Transfer"
label "concern:spore"      "2F6E5C" "spore.host integration"
label "blocked"            "6B7280" "Waiting on something external"

milestone() {
  gh api "repos/${repo}/milestones" -f title="$1" -f description="$2" >/dev/null 2>&1 \
    && echo "  milestone  $1" || echo "  milestone  $1 (exists)"
}

milestone "P0 Walking skeleton" \
  "Globus login end to end. DONE WHEN: logging in from two linked identities resolves to the same subject UUID."
milestone "P1 Catalog" \
  "Read-only dataset catalog over real OpenNeuro BIDS metadata. DONE WHEN: 20 real datasets ingest without a schema change."
milestone "P2 Account onboarding" \
  "Cross-account trust with per-pair ExternalId. DONE WHEN: someone other than the author runs the stack from the doc alone."
milestone "P3 The lease" \
  "spawn/spored/cohort wired to budget and expiry. DONE WHEN: a 10-minute, \$0.50 lease leaves nothing behind and Cost Explorer agrees."
milestone "P4 Data path" \
  "S3 gateway endpoint plus portal-issued STS session credentials. DONE WHEN: cross-project read returns AccessDenied."
milestone "P5 Interactive session" \
  "DCV with session-scoped ingress. DONE WHEN: the SG rule is gone after lease expiry."
milestone "P6 Globus movement" \
  "Portal-brokered transfers and endpoint allowlist. DONE WHEN: an off-allowlist destination is refused."
milestone "P7 RBAC depth" \
  "Projects, derivable roles, budget hierarchy, restricted tier."

issue() {
  gh issue create --title "$1" --body "$2" --label "$3" --milestone "$4" >/dev/null
  echo "  issue  $1"
}

# P0 issues live in scripts/gh-p0-issues.sh, which owns the richer set. Do not
# recreate them here — doing so made duplicate issues that had to be deleted by
# hand. The two issues below are P1 and have no other home yet, so they stay.

issue "ADR 0003: dataset tier model" \
"P1 needs a tier attribute on every catalog entry from the start, because retrofitting the restricted tier is painful.

Decide: tier vocabulary (open / registered / restricted?), what each requires, and how tier interacts with derivable roles.

Blocks: the catalog schema in P1." \
"p1:catalog,kind:adr,concern:security" "P1 Catalog"

issue "Choose the P1 stand-in datasets" \
"Candidates discussed:

- **OpenNeuro** (Registry of Open Data, CC0, BIDS-structured) — the zero-friction open tier
- **TUH EEG Corpus** (Temple) — clinically grounded, registration/DUA gated, so it exercises the restricted path
- **1000 Genomes / gnomAD** (RODA) — third modality, proves the catalog is domain-agnostic
- **NSRR** (SHHS, MESA, CHAT) — sleep; needs checking whether it is RODA-hosted or DUA-only

Decide which ship in the demo and which are catalog entries only." \
"p1:catalog,kind:spike" "P1 Catalog"

echo "Done. Create the Project board manually and add the P0 milestone to it."
