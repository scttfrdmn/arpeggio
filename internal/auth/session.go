package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// Session is a portal session. It holds the principal snapshot taken at login
// and the dependent tokens the portal uses to act as the user.
//
// Golden rule 5: these tokens live here, server-side, and are never handed to
// ephemeral compute. That is what makes the endpoint allowlist enforced rather
// than advisory.
type Session struct {
	ID        string           `dynamodbav:"id"`
	SubjectID string           `dynamodbav:"subject_id"`
	Principal Principal        `dynamodbav:"principal"`
	Tokens    map[string]Token `dynamodbav:"tokens"`
	CreatedAt time.Time        `dynamodbav:"created_at"`
	ExpiresAt time.Time        `dynamodbav:"expires_at"`

	// TTL is the DynamoDB time-to-live attribute, in Unix seconds. Expired
	// sessions are reaped by DynamoDB at no cost — no sweeper, no clock.
	TTL int64 `dynamodbav:"ttl"`
}

// Valid reports whether the session is still usable.
func (s *Session) Valid() bool {
	return s != nil && time.Now().Before(s.ExpiresAt)
}

// Transfer returns the dependent token for the Globus Transfer API.
func (s *Session) Transfer() (Token, bool) {
	t, ok := s.Tokens[ResourceServerTransfer]
	return t, ok
}

// SessionStore persists sessions. Implemented by internal/store over DynamoDB.
type SessionStore interface {
	Put(ctx context.Context, s *Session) error
	Get(ctx context.Context, id string) (*Session, error)
	Delete(ctx context.Context, id string) error
}

// NewSession builds a session from a principal and its dependent tokens.
func NewSession(p *Principal, tokens map[string]Token, ttl time.Duration) (*Session, error) {
	id, err := randomID(32)
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	now := time.Now().UTC()
	exp := now.Add(ttl)
	return &Session{
		ID:        id,
		SubjectID: p.Primary.SubjectID,
		Principal: *p,
		Tokens:    tokens,
		CreatedAt: now,
		ExpiresAt: exp,
		TTL:       exp.Unix(),
	}, nil
}

// randomID returns a URL-safe random identifier of n bytes of entropy.
func randomID(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
