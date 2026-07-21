#!/usr/bin/env bash
# P0 — walking skeleton. Globus login end to end, no data, no compute.
# Run after scripts/gh-bootstrap.sh has created labels and milestones.
set -euo pipefail
M="P0 Walking skeleton"

new() { # title, labels, body
  gh issue create --title "$1" --label "$2" --milestone "$M" --body "$3" >/dev/null
  echo "  + $1"
}

new "Register the Globus confidential client" \
"p0:skeleton,concern:globus" \
'Register at `app.globus.org/settings/developers`.

- Redirect URL points at the deployed API`'"'"'`s `/api/auth/callback`
- Request **all** scopes at first consent (see `config.DefaultScopes`) including `offline_access` — incremental consent mid-workflow is a poor portal experience, and P6 needs the Transfer scope with a refresh token
- Store the client secret in **SSM Parameter Store SecureString**, not Secrets Manager (that bills $0.40/secret/month and would break the no-clocks floor)

Note: Globus calls the thing you create here a "Project". That is an OAuth client registration and has nothing to do with an Arpeggio research project. Do not let the word leak into portal vocabulary.

**Acceptance:** `GET /api/auth/login` redirects to a Globus consent screen listing all six scopes.'

new "Deploy the P0 control plane" \
"p0:skeleton,concern:no-clocks" \
'`deploy/template.yaml` as written: DynamoDB on-demand with TTL, one Lambda (arm64, `provided.al2023`), HTTP API, S3 web bucket, 14-day log retention.

Check as you go that nothing acquires an hourly floor. The template comments say why each choice was made; if you change one, update the comment.

**Acceptance:** stack deploys clean, `GET /api/health` returns `{"status":"ok"}`, and the AWS Pricing Calculator estimate for the stack at zero traffic is under $1/month.'

new "Serve the SPA and the API from one origin" \
"p0:skeleton,concern:security" \
'The session is an `HttpOnly` cookie. If the SPA and the API are on different origins, every request needs CORS with credentials, `SameSite=None`, and a third-party-cookie story that browsers are actively breaking.

Avoid the whole class of problem: **one CloudFront distribution, two origins.**

- `/` → S3 web bucket (OAC, bucket stays private)
- `/api/*` → the HTTP API

Then the cookie is same-origin, `SameSite=Lax` works, and the CORS block in the template becomes unnecessary.

**Acceptance:** login round trip completes with no CORS headers involved, and `document.cookie` is empty in the console (proving `HttpOnly`).'

new "Wire the Go Globus SDK behind the Directory interface" \
"p0:skeleton,concern:globus" \
'`internal/auth/globus_http.go` is a deliberate stand-in. Replace it with a thin adapter over the Go Globus SDK port.

The three methods are `Introspect`, `Groups`, and `DependentTokens`.

**Constraint:** this must require no changes outside `internal/auth`. If it does, the `Directory` interface is wrong — fix the interface, not the callers.

Two shapes in the stand-in were written from memory and should be checked against the SDK, which is authoritative:
- the dependent-token grant response (an array of per-resource-server token objects)
- `my_groups` and where the caller`'"'"'`s role actually appears

**Acceptance:** `globus_http.go` is deleted and the P0 acceptance test still passes.'

new "Fake Directory for local development" \
"p0:skeleton,kind:chore" \
'Developing against real Globus for every UI change is miserable and rate-limited.

Add an `ARP_FAKE_GLOBUS=1` seam that swaps in a `Directory` returning a fixed principal with two linked identities and three groups at different roles — same pattern as the `DEMO_FAKE=1` seam in `aws-agentcore-demo`.

The fake belongs in `internal/auth`, not in a test file: the SPA work needs it at runtime.

**Acceptance:** `ARP_FAKE_GLOBUS=1 go run ./cmd/arpd` serves a signed-in `/api/me` with no network access and no Globus client configured.'

new "Session store: verify DynamoDB TTL actually reaps" \
"p0:skeleton,concern:no-clocks" \
'Sessions carry a `ttl` attribute so DynamoDB deletes them for free — no sweeper Lambda, no schedule. This is load-bearing for the no-clocks claim and it is easy to get subtly wrong.

Things that go wrong: the attribute is not Unix **seconds**, TTL is enabled on the wrong attribute name, or the value is written as a string.

Note that DynamoDB deletes on a best-effort basis up to 48 hours late, so `Session.Valid()` must remain the authority. TTL is garbage collection, not expiry.

**Acceptance:** a session written with a 60-second TTL is gone from the table within 48 hours, and is rejected by `/api/me` within 60 seconds regardless.'

new "ADR 0002: session model" \
"p0:skeleton,kind:adr" \
'Record why the session is a server-side record keyed by an opaque cookie rather than a self-contained JWT.

The reason is golden rule 5: the session holds the dependent Globus tokens, and those must stay server-side. A stateless token would either carry them to the browser or require a second lookup anyway.

Also record the `SameSite=Lax` choice (Strict drops the cookie on the Globus callback redirect) and the session-TTL-versus-Globus-token-lifetime relationship.'

new "P0 acceptance: linked identities resolve to one subject" \
"p0:skeleton,concern:security" \
'**The phase gate.**

Log in as a campus InCommon identity. Log out. Log in as a second identity linked to the same Globus account — ORCID or Google.

`GET /api/me` must return the **same** `subject` both times, with both identities present under `linked_identities`.

This is the test that catches authorization keyed on username strings instead of the identity UUID. That bug does not show up in single-identity testing, and it surfaces later as intermittent authorization failures that are extremely hard to diagnose.

**Done when this passes against real Globus, not the fake.**'

new "CI green on main" \
"p0:skeleton,kind:chore" \
'`make check` (vet, gofmt, `go test -race`) and `make build` pass in GitHub Actions.

Add a table-driven test for `config.Load()` covering the missing-key aggregation — it should name every missing key, not just the first.

**Acceptance:** the workflow badge is green and a PR with a gofmt violation is blocked.'

echo "P0 issues created."
