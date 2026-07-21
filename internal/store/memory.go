package store

import (
	"context"
	"sync"

	"github.com/scttfrdmn/arpeggio/internal/auth"
)

// Memory is an in-memory SessionStore for the ARP_FAKE_GLOBUS development seam.
// It exists so the portal runs with no DynamoDB and no network. It is never
// used in a deployed environment — the control plane persists to DynamoDB so
// sessions survive Lambda cold starts and so TTL reaps them for free.
//
// It does not honour the session TTL by reaping on a timer; Valid() already
// gates on ExpiresAt at read time, and a dev process is short-lived.
type Memory struct {
	mu       sync.RWMutex
	sessions map[string]*auth.Session
}

// NewMemory returns an empty in-memory session store.
func NewMemory() *Memory {
	return &Memory{sessions: make(map[string]*auth.Session)}
}

// Put stores a copy of the session so later mutations by the caller do not
// alias the stored record.
func (m *Memory) Put(ctx context.Context, s *auth.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored := *s
	m.sessions[s.ID] = &stored
	return nil
}

// Get returns the session, or auth.ErrNoSession if absent — matching the
// DynamoDB store, whose Get returns the same sentinel on a missing item.
func (m *Memory) Get(ctx context.Context, id string) (*auth.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, auth.ErrNoSession
	}
	out := *s
	return &out, nil
}

// Delete removes the session. Deleting an absent session is a no-op, matching
// the DynamoDB store.
func (m *Memory) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	return nil
}
