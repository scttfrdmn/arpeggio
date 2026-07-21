package auth

import (
	"context"
	"time"
)

// Identity is one linked Globus identity. A person may hold several — a campus
// InCommon identity, an ORCID, a Google account — all resolving to the same
// Globus account. Authorization keys on the primary SubjectID (a UUID), never
// on the username string, which can change and can differ between logins.
type Identity struct {
	SubjectID    string `dynamodbav:"subject_id"`
	Username     string `dynamodbav:"username"`
	Email        string `dynamodbav:"email"`
	Name         string `dynamodbav:"name"`
	Organization string `dynamodbav:"organization"`
	IdentityType string `dynamodbav:"identity_type"`
}

// Principal is the authenticated subject for a request: the primary identity,
// every identity linked to it, and its Globus group memberships.
//
// LinkedIDs matters for auditing a DUA-gated dataset: the trail must record
// which institutional identity was asserted at access time, not merely which
// human was behind it.
type Principal struct {
	Primary   Identity
	LinkedIDs []Identity
	Groups    []Group
	IssuedAt  time.Time
}

// HasGroup reports membership in a Globus group by UUID.
func (p Principal) HasGroup(id string) bool {
	for _, g := range p.Groups {
		if g.ID == id {
			return true
		}
	}
	return false
}

// GroupRole returns the caller's role in a group and whether they are a member.
func (p Principal) GroupRole(id string) (string, bool) {
	for _, g := range p.Groups {
		if g.ID == id {
			return g.Role, true
		}
	}
	return "", false
}

// Group is a Globus group membership. Role is one of admin, manager, or member
// — Globus defines exactly these three. Portal roles are richer and live in
// internal/rbac; this is a membership fact, not an entitlement.
type Group struct {
	ID       string `dynamodbav:"id"`
	Name     string `dynamodbav:"name"`
	Role     string `dynamodbav:"role"`
	ParentID string `dynamodbav:"parent_id,omitempty"`
}

// The complete set of Globus group roles.
const (
	GroupRoleAdmin   = "admin"
	GroupRoleManager = "manager"
	GroupRoleMember  = "member"
)

// Token is an access token for one Globus resource server.
type Token struct {
	ResourceServer string    `dynamodbav:"resource_server"`
	AccessToken    string    `dynamodbav:"access_token"`
	RefreshToken   string    `dynamodbav:"refresh_token,omitempty"`
	Scope          string    `dynamodbav:"scope"`
	ExpiresAt      time.Time `dynamodbav:"expires_at"`
}

// Expired reports whether the token is past its lifetime, with a margin so a
// token does not expire mid-flight.
func (t Token) Expired() bool {
	return time.Now().Add(30 * time.Second).After(t.ExpiresAt)
}

// Well-known Globus resource server identifiers, used as dependent-token keys.
const (
	ResourceServerGroups   = "groups.api.globus.org"
	ResourceServerTransfer = "transfer.api.globus.org"
)

// Directory is the narrow slice of Globus this package needs. It is declared
// here, by the consumer, so that the Go Globus SDK can satisfy it with a thin
// adapter and the stand-in HTTP client can be deleted without touching
// anything outside this package. See CLAUDE.md, "The Globus SDK seam".
type Directory interface {
	// Introspect resolves an access token to its principal, including the
	// full linked identity set.
	Introspect(ctx context.Context, accessToken string) (*Principal, error)

	// Groups lists the caller's group memberships. Requires a token carrying
	// the groups scope — in practice a dependent token.
	Groups(ctx context.Context, accessToken string) ([]Group, error)

	// DependentTokens exchanges a user access token for downstream resource
	// server tokens, keyed by resource server. This is what lets the portal
	// act as the user without ever handling a campus credential.
	DependentTokens(ctx context.Context, accessToken string) (map[string]Token, error)
}
