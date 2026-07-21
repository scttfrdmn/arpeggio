package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/scttfrdmn/arpeggio/internal/config"
)

// Authenticator runs the Globus Auth authorization-code flow.
//
// Globus is an InCommon service provider, so every campus IdP federated through
// InCommon works through this one integration. That is the reason Globus is the
// portal's IdP rather than per-institution Cognito federation.
type Authenticator struct {
	cfg      *config.Config
	provider *oidc.Provider
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
	dir      Directory
	sessions SessionStore
}

// NewAuthenticator discovers the Globus OIDC metadata and builds the flow.
func NewAuthenticator(ctx context.Context, cfg *config.Config, dir Directory, sessions SessionStore) (*Authenticator, error) {
	provider, err := oidc.NewProvider(ctx, cfg.GlobusIssuer)
	if err != nil {
		return nil, fmt.Errorf("discover globus oidc metadata at %s: %w", cfg.GlobusIssuer, err)
	}

	return &Authenticator{
		cfg:      cfg,
		provider: provider,
		dir:      dir,
		sessions: sessions,
		oauth: &oauth2.Config{
			ClientID:     cfg.GlobusClientID,
			ClientSecret: cfg.GlobusClientSecret,
			RedirectURL:  cfg.GlobusRedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       cfg.GlobusScopes,
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.GlobusClientID}),
	}, nil
}

// AuthCodeURL returns the Globus authorization URL and the PKCE verifier that
// must be carried through to Exchange.
func (a *Authenticator) AuthCodeURL(state string) (url, verifier string) {
	verifier = oauth2.GenerateVerifier()
	url = a.oauth.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier),
	)
	return url, verifier
}

// Exchange completes the authorization-code flow and establishes a session.
//
// The sequence is deliberate: verify the ID token, resolve the full linked
// identity set, then exchange for dependent tokens. Groups are read with the
// dependent Groups token, not the primary access token.
func (a *Authenticator) Exchange(ctx context.Context, code, verifier string) (*Session, error) {
	tok, err := a.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}

	rawID, ok := tok.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("globus response carried no id_token")
	}
	if _, err := a.verifier.Verify(ctx, rawID); err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}

	return a.establish(ctx, tok.AccessToken)
}

// establish resolves a session from an access token: introspect for the linked
// identity set, exchange for dependent tokens, then read groups with the
// dependent Groups token. Split out of Exchange so the fake login seam can
// reuse it without an OAuth round trip.
func (a *Authenticator) establish(ctx context.Context, accessToken string) (*Session, error) {
	principal, err := a.dir.Introspect(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("introspect access token: %w", err)
	}

	dependents, err := a.dir.DependentTokens(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("obtain dependent tokens: %w", err)
	}

	if gt, ok := dependents[ResourceServerGroups]; ok {
		groups, err := a.dir.Groups(ctx, gt.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("list globus groups: %w", err)
		}
		principal.Groups = groups
	}
	principal.IssuedAt = time.Now().UTC()

	sess, err := NewSession(principal, dependents, a.cfg.SessionTTL)
	if err != nil {
		return nil, err
	}
	if err := a.sessions.Put(ctx, sess); err != nil {
		return nil, fmt.Errorf("persist session: %w", err)
	}
	return sess, nil
}

// FakeLogin mints a session directly from the configured Directory, skipping
// the Globus OAuth round trip. It exists only for the ARP_FAKE_GLOBUS
// development seam and must never run against real Globus. The token string is
// a sentinel the FakeDirectory ignores.
func (a *Authenticator) FakeLogin(ctx context.Context) (*Session, error) {
	return a.establish(ctx, "fake-access-token")
}

// Session loads and validates the session referenced by the request cookie.
func (a *Authenticator) Session(ctx context.Context, r *http.Request) (*Session, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return nil, ErrNoSession
	}
	sess, err := a.sessions.Get(ctx, c.Value)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	if !sess.Valid() {
		return nil, ErrNoSession
	}
	return sess, nil
}

// Logout destroys the server-side session.
func (a *Authenticator) Logout(ctx context.Context, id string) error {
	return a.sessions.Delete(ctx, id)
}

// StateHash derives a short, opaque state value from the PKCE verifier so the
// callback can be correlated without server-side state storage.
func StateHash(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}
