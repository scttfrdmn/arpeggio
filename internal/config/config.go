// Package config loads runtime configuration from the environment.
//
// Configuration is read once at cold start. Secrets (the Globus client secret)
// come from SSM Parameter Store SecureString, not Secrets Manager — see golden
// rule 2 in CLAUDE.md.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config is the fully resolved runtime configuration.
type Config struct {
	GlobusIssuer       string
	GlobusClientID     string
	GlobusClientSecret string
	GlobusRedirectURL  string
	GlobusScopes       []string

	// TableName is the DynamoDB single-table name.
	TableName string

	// SessionTTL bounds a portal session independently of Globus token lifetime.
	SessionTTL time.Duration

	// PublicBaseURL is the origin the SPA is served from; used for CORS and
	// for post-login redirects.
	PublicBaseURL string

	// Fake, set by ARP_FAKE_GLOBUS=1, swaps the real Globus Directory and the
	// DynamoDB session store for in-memory stand-ins so the portal runs with no
	// network and no Globus client configured. A development seam only; it must
	// never be set in a deployed environment.
	Fake bool
}

// DefaultScopes are requested at first consent. All of them are asked for up
// front deliberately: incremental consent mid-workflow is a poor portal
// experience, and the Transfer scope is needed before the user reaches P6.
var DefaultScopes = []string{
	"openid",
	"profile",
	"email",
	"urn:globus:auth:scope:groups.api.globus.org:view_my_groups_and_memberships",
	"urn:globus:auth:scope:transfer.api.globus.org:all",
	"offline_access",
}

// Load reads configuration from the environment, naming every missing required
// key rather than failing on the first one.
func Load() (*Config, error) {
	c := &Config{
		GlobusIssuer:       envOr("ARP_GLOBUS_ISSUER", "https://auth.globus.org"),
		GlobusClientID:     os.Getenv("ARP_GLOBUS_CLIENT_ID"),
		GlobusClientSecret: os.Getenv("ARP_GLOBUS_CLIENT_SECRET"),
		GlobusRedirectURL:  os.Getenv("ARP_GLOBUS_REDIRECT_URL"),
		TableName:          os.Getenv("ARP_TABLE_NAME"),
		PublicBaseURL:      os.Getenv("ARP_PUBLIC_BASE_URL"),
		GlobusScopes:       DefaultScopes,
		Fake:               truthy(os.Getenv("ARP_FAKE_GLOBUS")),
	}

	if raw := os.Getenv("ARP_GLOBUS_SCOPES"); raw != "" {
		c.GlobusScopes = strings.Fields(raw)
	}

	ttl := 12 * time.Hour
	if raw := os.Getenv("ARP_SESSION_TTL"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("parse ARP_SESSION_TTL %q: %w", raw, err)
		}
		ttl = d
	}
	c.SessionTTL = ttl

	// The fake seam runs with no Globus client and no DynamoDB table, so none
	// of the Globus or table keys are required. PublicBaseURL still drives the
	// post-login redirect, so give it a local default rather than demanding it.
	if c.Fake {
		if c.PublicBaseURL == "" {
			c.PublicBaseURL = "http://localhost:8080"
		}
		return c, nil
	}

	var missing []string
	for _, kv := range [][2]string{
		{"ARP_GLOBUS_CLIENT_ID", c.GlobusClientID},
		{"ARP_GLOBUS_CLIENT_SECRET", c.GlobusClientSecret},
		{"ARP_GLOBUS_REDIRECT_URL", c.GlobusRedirectURL},
		{"ARP_TABLE_NAME", c.TableName},
		{"ARP_PUBLIC_BASE_URL", c.PublicBaseURL},
	} {
		if kv[1] == "" {
			missing = append(missing, kv[0])
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	return c, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// truthy reports whether an environment value reads as enabled. Anything other
// than the obvious off values counts as on, so ARP_FAKE_GLOBUS=1, =true, or
// =yes all work.
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
