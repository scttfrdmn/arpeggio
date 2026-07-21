package auth

import (
	"context"
	"testing"
	"time"

	"github.com/scttfrdmn/arpeggio/internal/config"
)

// memStore is a minimal SessionStore for tests, kept here so internal/auth has
// no test dependency on internal/store (which would be an import cycle risk and
// is heavier than these tests need).
type memStore struct {
	m map[string]*Session
}

func newMemStore() *memStore { return &memStore{m: make(map[string]*Session)} }

func (s *memStore) Put(_ context.Context, sess *Session) error {
	cp := *sess
	s.m[sess.ID] = &cp
	return nil
}

func (s *memStore) Get(_ context.Context, id string) (*Session, error) {
	sess, ok := s.m[id]
	if !ok {
		return nil, ErrNoSession
	}
	return sess, nil
}

func (s *memStore) Delete(_ context.Context, id string) error {
	delete(s.m, id)
	return nil
}

func TestFakeLogin(t *testing.T) {
	store := newMemStore()
	a := NewFakeAuthenticator(&config.Config{SessionTTL: time.Hour}, store)

	sess, err := a.FakeLogin(context.Background())
	if err != nil {
		t.Fatalf("FakeLogin: %v", err)
	}

	// The P0 gate: linked identities resolve to one subject. The primary
	// subject is the login identity, and the ORCID identity is present in the
	// linked set under the same human.
	if sess.SubjectID != FakeSubjectID {
		t.Errorf("SubjectID = %q, want %q", sess.SubjectID, FakeSubjectID)
	}
	if got := len(sess.Principal.LinkedIDs); got != 2 {
		t.Fatalf("linked identities = %d, want 2", got)
	}
	var sawORCID bool
	for _, id := range sess.Principal.LinkedIDs {
		if id.SubjectID == FakeORCIDSubjectID {
			sawORCID = true
		}
	}
	if !sawORCID {
		t.Errorf("linked identities missing the ORCID subject %q", FakeORCIDSubjectID)
	}

	// Three groups, one at each Globus role, so RBAC work has all three.
	roles := map[string]bool{}
	for _, g := range sess.Principal.Groups {
		roles[g.Role] = true
	}
	for _, want := range []string{GroupRoleAdmin, GroupRoleManager, GroupRoleMember} {
		if !roles[want] {
			t.Errorf("no group at role %q; got roles %v", want, roles)
		}
	}

	// The session persisted and is loadable — the SPA reads /api/me right after.
	loaded, err := store.Get(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("session not persisted: %v", err)
	}
	if loaded.SubjectID != FakeSubjectID {
		t.Errorf("persisted SubjectID = %q, want %q", loaded.SubjectID, FakeSubjectID)
	}

	// Golden rule 5: the portal holds the dependent Transfer token server-side.
	if _, ok := loaded.Transfer(); !ok {
		t.Error("no dependent Transfer token on the fake session")
	}
}
