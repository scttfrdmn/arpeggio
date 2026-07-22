package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/scttfrdmn/globus-go-sdk/v4/pkg/authorizers"
	"github.com/scttfrdmn/globus-go-sdk/v4/pkg/core"
	sdkauth "github.com/scttfrdmn/globus-go-sdk/v4/pkg/services/auth"
	sdkgroups "github.com/scttfrdmn/globus-go-sdk/v4/pkg/services/groups"
)

// SDKDirectory is the Directory implementation over the Go Globus SDK. It
// replaces the hand-rolled HTTPDirectory stand-in; see CLAUDE.md, "The Globus
// SDK seam". This is the only file in the repo that imports the Globus SDK, and
// nothing outside internal/auth changed to introduce it — the Directory
// interface was satisfiable as written.
//
// Two distinct authentications are in play, which is why the introspect/
// dependent-token calls and the groups call use separately-built clients:
//
//   - Introspect and DependentTokens authenticate the *confidential client*
//     itself (HTTP Basic, client_id:client_secret), per RFC 7662. A user Bearer
//     token is rejected on those endpoints.
//   - Groups authenticates as the *user*, with the dependent Groups access
//     token the portal already holds — passed in by the caller, not stored here.
type SDKDirectory struct {
	// authClient is built once with client-Basic auth; it serves Introspect and
	// DependentTokens, neither of which varies per user credential.
	authClient *sdkauth.Client

	// newGroupsClient builds a groups client bound to a specific user access
	// token. Groups membership is read as the user, so the client is per-call.
	newGroupsClient func(ctx context.Context, accessToken string) (*sdkgroups.Client, error)
}

// introspectScopes / groupsScopes: v4 requires explicit scopes at client
// construction (core.Config.Validate). Introspection and the dependent-token
// grant are client-authenticated management calls; openid is the minimal
// declared scope. Groups reads require the groups view scope.
var (
	introspectScopes = []string{"openid"}
	groupsScopes     = []string{"urn:globus:auth:scope:groups.api.globus.org:view_my_groups_and_memberships"}
)

// NewSDKDirectory builds the SDK-backed Directory for a confidential client.
func NewSDKDirectory(ctx context.Context, clientID, clientSecret string) (*SDKDirectory, error) {
	authClient, err := sdkauth.NewClient(ctx, &core.Config{
		Authorizer: authorizers.NewBasicAuthAuthorizer(clientID, clientSecret),
		Scopes:     introspectScopes,
	})
	if err != nil {
		return nil, fmt.Errorf("build globus auth client: %w", err)
	}

	return &SDKDirectory{
		authClient: authClient,
		newGroupsClient: func(ctx context.Context, accessToken string) (*sdkgroups.Client, error) {
			return sdkgroups.NewClient(ctx, &core.Config{
				AccessToken: accessToken,
				Scopes:      groupsScopes,
			})
		},
	}, nil
}

// Introspect resolves an access token to its principal, including the full
// linked identity set. identity_set_detail returns the linked records inline,
// so no second GetIdentities round trip is needed.
func (d *SDKDirectory) Introspect(ctx context.Context, accessToken string) (*Principal, error) {
	intro, err := d.authClient.IntrospectToken(ctx, accessToken, &sdkauth.IntrospectOptions{
		Include: "identity_set_detail",
	})
	if err != nil {
		return nil, fmt.Errorf("introspect token: %w", err)
	}
	if !intro.Active {
		return nil, fmt.Errorf("globus reports token inactive")
	}

	p := &Principal{
		Primary: Identity{
			SubjectID:    intro.Sub,
			Username:     intro.Username,
			Email:        intro.Email,
			Name:         intro.Name,
			Organization: intro.Organization,
		},
	}
	for _, i := range intro.IdentitySetDetail {
		p.LinkedIDs = append(p.LinkedIDs, Identity{
			SubjectID:    i.ID,
			Username:     i.Username,
			Email:        i.Email,
			Name:         i.Name,
			Organization: i.Organization,
			IdentityType: i.IdentityType,
		})
	}
	return p, nil
}

// DependentTokens exchanges the user access token for downstream resource
// server tokens, keyed by resource server. RefreshTokens requests offline
// access so the portal can act as the user after the access token expires.
func (d *SDKDirectory) DependentTokens(ctx context.Context, accessToken string) (map[string]Token, error) {
	infos, err := d.authClient.GetDependentTokens(ctx, accessToken, &sdkauth.DependentTokensOptions{
		RefreshTokens: true,
	})
	if err != nil {
		return nil, fmt.Errorf("get dependent tokens: %w", err)
	}

	out := make(map[string]Token, len(infos))
	for _, t := range infos {
		out[t.ResourceServer] = Token{
			ResourceServer: t.ResourceServer,
			AccessToken:    t.AccessToken,
			RefreshToken:   t.RefreshToken,
			Scope:          t.Scope,
			ExpiresAt:      time.Now().UTC().Add(time.Duration(t.ExpiresIn) * time.Second),
		}
	}
	return out, nil
}

// Groups lists the caller's memberships using their dependent Groups token.
// include=my_memberships is what carries the caller's own role; without it only
// the coarse is_group_admin/is_member bools are available, which cannot tell
// manager from member.
func (d *SDKDirectory) Groups(ctx context.Context, accessToken string) ([]Group, error) {
	gc, err := d.newGroupsClient(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("build globus groups client: %w", err)
	}

	raw, err := gc.GetMyGroupsWithOptions(ctx, &sdkgroups.GetMyGroupsOptions{
		Statuses: []string{"active"},
		Include:  []string{"my_memberships"},
	})
	if err != nil {
		return nil, fmt.Errorf("list my groups: %w", err)
	}

	groups := make([]Group, 0, len(raw))
	for _, g := range raw {
		groups = append(groups, Group{
			ID:       g.ID,
			Name:     g.Name,
			Role:     callerRole(g),
			ParentID: g.ParentID,
		})
	}
	return groups, nil
}

// callerRole extracts the caller's role in a group. my_memberships carries the
// caller's own membership(s) with the role string; fall back to the coarse
// bools only if the include was somehow not honoured, so the result is never
// worse than the pre-SDK stand-in.
func callerRole(g sdkgroups.Group) string {
	for _, m := range g.MyMemberships {
		if m.Role != "" {
			return m.Role
		}
	}
	if g.IsGroupAdmin {
		return GroupRoleAdmin
	}
	return GroupRoleMember
}
