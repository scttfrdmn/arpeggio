package config

import (
	"strings"
	"testing"
	"time"
)

// allRequired is the minimal set of env vars a non-fake Load needs. Cases start
// from this and mutate one thing, so each case names exactly what it exercises.
func allRequired() map[string]string {
	return map[string]string{
		"ARP_GLOBUS_CLIENT_ID":     "client-abc",
		"ARP_GLOBUS_CLIENT_SECRET": "secret-xyz",
		"ARP_GLOBUS_REDIRECT_URL":  "https://portal.example.edu/api/auth/callback",
		"ARP_TABLE_NAME":           "arpeggio",
		"ARP_PUBLIC_BASE_URL":      "https://portal.example.edu",
	}
}

func TestLoad(t *testing.T) {
	// Every ARP_* key the loader reads, so a case starting from a subset of
	// allRequired() is not polluted by the ambient environment.
	allKeys := []string{
		"ARP_GLOBUS_ISSUER", "ARP_GLOBUS_CLIENT_ID", "ARP_GLOBUS_CLIENT_SECRET",
		"ARP_GLOBUS_REDIRECT_URL", "ARP_TABLE_NAME", "ARP_PUBLIC_BASE_URL",
		"ARP_GLOBUS_SCOPES", "ARP_SESSION_TTL", "ARP_FAKE_GLOBUS",
	}

	tests := []struct {
		name    string
		env     map[string]string
		wantErr string // substring; "" means expect success
		check   func(t *testing.T, c *Config)
	}{
		{
			name: "all required present",
			env:  allRequired(),
			check: func(t *testing.T, c *Config) {
				if c.TableName != "arpeggio" {
					t.Errorf("TableName = %q, want arpeggio", c.TableName)
				}
				if c.SessionTTL != 12*time.Hour {
					t.Errorf("SessionTTL = %v, want default 12h", c.SessionTTL)
				}
				if c.GlobusIssuer != "https://auth.globus.org" {
					t.Errorf("GlobusIssuer = %q, want default", c.GlobusIssuer)
				}
				if len(c.GlobusScopes) != len(DefaultScopes) {
					t.Errorf("GlobusScopes = %v, want defaults", c.GlobusScopes)
				}
				if c.Fake {
					t.Error("Fake = true, want false")
				}
			},
		},
		{
			name:    "one missing key is named",
			env:     without(allRequired(), "ARP_TABLE_NAME"),
			wantErr: "ARP_TABLE_NAME",
		},
		{
			name:    "every missing key is named at once",
			env:     map[string]string{},
			wantErr: "ARP_GLOBUS_CLIENT_ID, ARP_GLOBUS_CLIENT_SECRET, ARP_GLOBUS_REDIRECT_URL, ARP_TABLE_NAME, ARP_PUBLIC_BASE_URL",
		},
		{
			name: "fake mode needs no globus or table",
			env:  map[string]string{"ARP_FAKE_GLOBUS": "1"},
			check: func(t *testing.T, c *Config) {
				if !c.Fake {
					t.Error("Fake = false, want true")
				}
				if c.PublicBaseURL != "http://localhost:8080" {
					t.Errorf("PublicBaseURL = %q, want local default", c.PublicBaseURL)
				}
			},
		},
		{
			name: "fake mode keeps an explicit public base url",
			env: map[string]string{
				"ARP_FAKE_GLOBUS":     "true",
				"ARP_PUBLIC_BASE_URL": "http://localhost:3000",
			},
			check: func(t *testing.T, c *Config) {
				if c.PublicBaseURL != "http://localhost:3000" {
					t.Errorf("PublicBaseURL = %q, want the explicit value", c.PublicBaseURL)
				}
			},
		},
		{
			name: "ARP_FAKE_GLOBUS=0 is off, and then keys are required",
			env:  map[string]string{"ARP_FAKE_GLOBUS": "0"},
			// off + no required keys => the required-key error, proving 0 is falsey
			wantErr: "missing required configuration",
		},
		{
			name: "custom scopes split on whitespace",
			env:  with(allRequired(), "ARP_GLOBUS_SCOPES", "openid  profile   email"),
			check: func(t *testing.T, c *Config) {
				want := []string{"openid", "profile", "email"}
				if strings.Join(c.GlobusScopes, ",") != strings.Join(want, ",") {
					t.Errorf("GlobusScopes = %v, want %v", c.GlobusScopes, want)
				}
			},
		},
		{
			name: "custom session ttl parses",
			env:  with(allRequired(), "ARP_SESSION_TTL", "90m"),
			check: func(t *testing.T, c *Config) {
				if c.SessionTTL != 90*time.Minute {
					t.Errorf("SessionTTL = %v, want 90m", c.SessionTTL)
				}
			},
		},
		{
			name:    "bad session ttl is a named error",
			env:     with(allRequired(), "ARP_SESSION_TTL", "not-a-duration"),
			wantErr: "ARP_SESSION_TTL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear every key the loader reads, then set only this case's env.
			// t.Setenv restores prior values and forbids t.Parallel, so cases
			// cannot leak into each other.
			for _, k := range allKeys {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			c, err := Load()

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() succeeded, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, c)
			}
		})
	}
}

func with(m map[string]string, k, v string) map[string]string {
	out := make(map[string]string, len(m)+1)
	for kk, vv := range m {
		out[kk] = vv
	}
	out[k] = v
	return out
}

func without(m map[string]string, k string) map[string]string {
	out := make(map[string]string, len(m))
	for kk, vv := range m {
		if kk != k {
			out[kk] = vv
		}
	}
	return out
}
