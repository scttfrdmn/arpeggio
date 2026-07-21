package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPDirectory is a stand-in implementation of Directory over the Globus REST
// API. It exists only until the Go Globus SDK is wired in.
//
// DO NOT add Globus HTTP calls anywhere else in this repo (CLAUDE.md, golden
// rules). When the SDK adapter lands, this file is deleted and nothing outside
// this package changes.
type HTTPDirectory struct {
	AuthBase     string
	GroupsBase   string
	ClientID     string
	ClientSecret string
	HTTP         *http.Client
}

// NewHTTPDirectory builds the stand-in client.
func NewHTTPDirectory(clientID, clientSecret string) *HTTPDirectory {
	return &HTTPDirectory{
		AuthBase:     "https://auth.globus.org/v2",
		GroupsBase:   "https://groups.api.globus.org/v2",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTP:         &http.Client{Timeout: 15 * time.Second},
	}
}

type introspectResponse struct {
	Active       bool     `json:"active"`
	Sub          string   `json:"sub"`
	Username     string   `json:"username"`
	Email        string   `json:"email"`
	Name         string   `json:"name"`
	Organization string   `json:"organization"`
	IdentitySet  []idInfo `json:"identity_set_detail"`
}

type idInfo struct {
	Sub          string `json:"sub"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Organization string `json:"organization"`
	IdentityType string `json:"identity_type"`
}

// Introspect resolves a token to its principal. The identity_set_detail
// include is what yields linked identities; without it, a user who logs in
// with a different linked identity looks like a different person.
func (d *HTTPDirectory) Introspect(ctx context.Context, accessToken string) (*Principal, error) {
	form := url.Values{
		"token":   {accessToken},
		"include": {"identity_set_detail"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.AuthBase+"/oauth2/token/introspect", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(d.ClientID, d.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var out introspectResponse
	if err := d.do(req, &out); err != nil {
		return nil, err
	}
	if !out.Active {
		return nil, fmt.Errorf("globus reports token inactive")
	}

	p := &Principal{
		Primary: Identity{
			SubjectID:    out.Sub,
			Username:     out.Username,
			Email:        out.Email,
			Name:         out.Name,
			Organization: out.Organization,
		},
	}
	for _, i := range out.IdentitySet {
		p.LinkedIDs = append(p.LinkedIDs, Identity{
			SubjectID:    i.Sub,
			Username:     i.Username,
			Email:        i.Email,
			Name:         i.Name,
			Organization: i.Organization,
			IdentityType: i.IdentityType,
		})
	}
	return p, nil
}

// DependentTokens exchanges a user token for downstream resource server tokens.
func (d *HTTPDirectory) DependentTokens(ctx context.Context, accessToken string) (map[string]Token, error) {
	form := url.Values{
		"grant_type":  {"urn:globus:auth:grant_type:dependent_token"},
		"token":       {accessToken},
		"access_type": {"offline"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.AuthBase+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(d.ClientID, d.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var raw []struct {
		ResourceServer string `json:"resource_server"`
		AccessToken    string `json:"access_token"`
		RefreshToken   string `json:"refresh_token"`
		Scope          string `json:"scope"`
		ExpiresIn      int    `json:"expires_in"`
	}
	if err := d.do(req, &raw); err != nil {
		return nil, err
	}

	out := make(map[string]Token, len(raw))
	for _, t := range raw {
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

// Groups lists the caller's memberships using a dependent Groups token.
func (d *HTTPDirectory) Groups(ctx context.Context, accessToken string) ([]Group, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		d.GroupsBase+"/groups/my_groups", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	var raw []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		ParentID      string `json:"parent_id"`
		MyMemberships []struct {
			Role string `json:"role"`
		} `json:"my_memberships"`
	}
	if err := d.do(req, &raw); err != nil {
		return nil, err
	}

	groups := make([]Group, 0, len(raw))
	for _, g := range raw {
		role := GroupRoleMember
		if len(g.MyMemberships) > 0 {
			role = g.MyMemberships[0].Role
		}
		groups = append(groups, Group{ID: g.ID, Name: g.Name, Role: role, ParentID: g.ParentID})
	}
	return groups, nil
}

func (d *HTTPDirectory) do(req *http.Request, out any) error {
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", req.URL.Path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("globus %s returned %s", req.URL.Path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", req.URL.Path, err)
	}
	return nil
}
