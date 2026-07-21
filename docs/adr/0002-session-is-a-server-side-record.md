# ADR 0002 — The portal session is a server-side record, not a stateless token

**Status:** accepted

## Context

The portal needs a session: a way to recognise a signed-in user across requests
without re-running the Globus login flow each time. The default reach for a
serverless API is a stateless token — a signed JWT in a cookie or `Authorization`
header — because it needs no server-side store and no lookup on the hot path.

But a portal session is not only an identity claim. Golden rule 5 requires the
portal to hold the user's **dependent Globus tokens** — the Transfer token in
particular — server-side, and to submit every Globus transfer itself. That is
what makes the endpoint allowlist enforced rather than advisory: if the token
never reaches the browser or the instance, the allowlist cannot be bypassed by
someone who has a shell.

A stateless session token cannot hold the dependent tokens. It would either
carry them to the browser — defeating rule 5 outright — or omit them and force a
server-side lookup on every request anyway, which is the store a JWT was
supposed to avoid. The one thing a JWT buys, we cannot use.

## Decision

The session is a **server-side record in DynamoDB**, keyed by an opaque random
identifier. The browser holds only that identifier, in an `HttpOnly` cookie.

- **Opaque cookie, not a JWT.** The cookie value is 32 bytes of `crypto/rand`,
  URL-safe encoded (`auth.randomID`). It carries no claims. Every request that
  needs the principal or the tokens reads the DynamoDB item (`store.Table.Get`).
- **`HttpOnly`, `Secure`, `SameSite=Lax`.** `HttpOnly` keeps the identifier out
  of `document.cookie` and out of reach of page script. `SameSite=Lax` rather
  than `Strict`: the Globus callback is a top-level cross-site navigation
  (`auth.globus.org` → the portal), and `Strict` would drop the cookie on that
  redirect, so the freshly established session would not be sent on the very
  next request. `Secure` is set whenever `PublicBaseURL` is `https://`.
- **The session carries its own TTL, independent of the Globus token lifetimes.**
  `SessionTTL` (default 12h, `ARP_SESSION_TTL`) bounds how long the portal will
  act for a user without a fresh login. It is written to the DynamoDB item both
  as `ExpiresAt` (checked at read time by `Session.Valid`) and as a Unix-seconds
  `ttl` attribute, so DynamoDB reaps expired sessions for free.

## Consequences

- **Rule 5 holds by construction.** The dependent tokens live in the session
  item, never in the cookie. Revoking a session is a single `DeleteItem`; a JWT
  would remain valid until expiry with no server-side kill switch.
- **No clock, no sweeper (rule 2).** Expiry is enforced two ways, both free:
  `Session.Valid` gates on `ExpiresAt` at read time, and the DynamoDB TTL
  attribute deletes the stale item with no Lambda and no schedule. Verifying
  that the TTL actually reaps is its own P0 issue.
- **Session TTL and Globus token lifetime are decoupled on purpose.** A dependent
  token may expire before the session does; `Token.Expired` reports this with a
  30-second margin so a token is not used mid-flight. Refreshing a dependent
  token is a token-store concern, not a reason to end the session — and the
  session ending is not a reason to assume the tokens are dead. Keeping the two
  clocks separate is what lets each be managed on its own terms.
- **One DynamoDB read per authenticated request.** Accepted: the table is
  on-demand and single-digit-millisecond, and the read is the same lookup a
  JWT-plus-token-store design would need anyway.
- **The cookie is same-origin.** This assumes the SPA and the API are served
  from one origin, so `SameSite=Lax` is sufficient and no `SameSite=None` /
  third-party-cookie story is needed. Serving them from one CloudFront
  distribution is its own P0 issue; this ADR depends on that outcome.

## Alternatives rejected

**Stateless signed JWT in the cookie.** Rejected: it cannot hold the dependent
Globus tokens without either shipping them to the browser (violates rule 5) or
requiring a server-side lookup anyway (removes the only benefit). It also has no
revocation short of a blocklist, which is itself a server-side store.

**Token in an `Authorization` header instead of a cookie.** Rejected: the SPA is
browser-first and the login flow ends in a top-level redirect, which lands as a
cookie, not a header. A header-bearer scheme would push token custody into page
script — the opposite of `HttpOnly`.

**`SameSite=Strict`.** Rejected: it drops the cookie on the Globus callback
navigation, breaking the session on the first request after login.
