package auth

import (
	"context"
	"time"

	"github.com/scttfrdmn/arpeggio/internal/config"
)

// FakeDirectory is a Directory that returns a fixed principal with no network
// access and no Globus client configured. It backs the ARP_FAKE_GLOBUS
// development seam so the SPA and the API can be exercised end to end without
// real Globus, which is rate-limited and miserable to develop against.
//
// It is a runtime seam, not a test helper, so it lives beside the real
// Directory rather than in a _test.go file: the SPA work needs a signed-in
// /api/me at runtime.
//
// The fixture is shaped to exercise the parts of P0 that matter: two linked
// identities under one subject (the linked-identity resolution the P0 gate
// tests) and three groups at the three distinct Globus roles (so RBAC work in
// P7 has admin, manager, and member all present).
type FakeDirectory struct{}

// NewFakeDirectory returns the stand-in Directory.
func NewFakeDirectory() *FakeDirectory { return &FakeDirectory{} }

// Fixed identifiers for the fake principal. Stable across runs so the SPA can
// hard-code them in fixtures and so audit-trail output is reproducible.
const (
	FakeSubjectID = "11111111-1111-4111-8111-111111111111"

	// The linked ORCID identity — a second institutional identity resolving to
	// the same human, which is exactly what the P0 gate checks.
	FakeORCIDSubjectID = "22222222-2222-4222-8222-222222222222"

	FakeGroupAdminID   = "aaaaaaaa-0000-4000-8000-000000000001"
	FakeGroupManagerID = "aaaaaaaa-0000-4000-8000-000000000002"
	FakeGroupMemberID  = "aaaaaaaa-0000-4000-8000-000000000003"
)

// fakePrincipal is the fixed subject the fake seam signs in as.
func fakePrincipal() *Principal {
	return &Principal{
		Primary: Identity{
			SubjectID:    FakeSubjectID,
			Username:     "ada@example.edu",
			Email:        "ada@example.edu",
			Name:         "Ada Lovelace",
			Organization: "Example University",
			IdentityType: "login",
		},
		LinkedIDs: []Identity{
			{
				SubjectID:    FakeSubjectID,
				Username:     "ada@example.edu",
				Email:        "ada@example.edu",
				Name:         "Ada Lovelace",
				Organization: "Example University",
				IdentityType: "login",
			},
			{
				SubjectID:    FakeORCIDSubjectID,
				Username:     "0000-0002-1825-0097@orcid.org",
				Email:        "ada@example.edu",
				Name:         "Ada Lovelace",
				Organization: "ORCID",
				IdentityType: "link",
			},
		},
	}
}

// Introspect returns the fixed principal, ignoring the token.
func (FakeDirectory) Introspect(ctx context.Context, accessToken string) (*Principal, error) {
	return fakePrincipal(), nil
}

// Groups returns three memberships, one at each Globus role.
func (FakeDirectory) Groups(ctx context.Context, accessToken string) ([]Group, error) {
	return []Group{
		{ID: FakeGroupAdminID, Name: "Gauss Admins", Role: GroupRoleAdmin},
		{ID: FakeGroupManagerID, Name: "Neuroimaging Lab", Role: GroupRoleManager},
		{ID: FakeGroupMemberID, Name: "Intro to HPC (course)", Role: GroupRoleMember},
	}, nil
}

// DependentTokens returns non-expiring stand-in tokens for the Groups and
// Transfer resource servers. The Groups token must be present or establish()
// skips the Groups call and the fixture would carry no memberships.
func (FakeDirectory) DependentTokens(ctx context.Context, accessToken string) (map[string]Token, error) {
	exp := time.Now().UTC().Add(24 * time.Hour)
	return map[string]Token{
		ResourceServerGroups: {
			ResourceServer: ResourceServerGroups,
			AccessToken:    "fake-groups-token",
			Scope:          "urn:globus:auth:scope:groups.api.globus.org:view_my_groups_and_memberships",
			ExpiresAt:      exp,
		},
		ResourceServerTransfer: {
			ResourceServer: ResourceServerTransfer,
			AccessToken:    "fake-transfer-token",
			Scope:          "urn:globus:auth:scope:transfer.api.globus.org:all",
			ExpiresAt:      exp,
		},
	}, nil
}

// NewFakeAuthenticator builds an Authenticator wired to the FakeDirectory and
// the given session store, skipping OIDC discovery entirely. The real
// NewAuthenticator reaches auth.globus.org at construction; the fake path must
// run with no network, so it never builds the OAuth config or the verifier.
// Only FakeLogin and the session methods are valid on the result — AuthCodeURL
// and Exchange would panic on the nil oauth config, which is intended: the fake
// seam does not use the browser redirect flow.
func NewFakeAuthenticator(cfg *config.Config, sessions SessionStore) *Authenticator {
	return &Authenticator{
		cfg:      cfg,
		dir:      NewFakeDirectory(),
		sessions: sessions,
	}
}
