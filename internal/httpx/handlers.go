package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/scttfrdmn/arpeggio/internal/auth"
	"github.com/scttfrdmn/arpeggio/internal/config"
)

// Server wires the P0 API surface.
type Server struct {
	cfg  *config.Config
	auth *auth.Authenticator
}

// NewServer builds the API server.
func NewServer(cfg *config.Config, a *auth.Authenticator) *Server {
	return &Server{cfg: cfg, auth: a}
}

// Routes returns the mux for the API.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/auth/login", s.login)
	mux.HandleFunc("GET /api/auth/callback", s.callback)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/me", s.me)
	return mux
}

func (s *Server) secure() bool {
	return strings.HasPrefix(s.cfg.PublicBaseURL, "https://")
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// login redirects to Globus, carrying a PKCE verifier in a short-lived cookie.
//
// Under the ARP_FAKE_GLOBUS seam there is no Globus to redirect to: mint a
// session directly and land on the home route, so the SPA sees a signed-in
// /api/me with no network.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Fake {
		sess, err := s.auth.FakeLogin(r.Context())
		if err != nil {
			Error(w, http.StatusInternalServerError, "fake_login_failed",
				"The development login seam could not create a session.")
			return
		}
		auth.SetSessionCookie(w, sess, s.secure())
		http.Redirect(w, r, s.cfg.PublicBaseURL+"/#/home", http.StatusFound)
		return
	}

	url, verifier := s.auth.AuthCodeURL("")
	state := auth.StateHash(verifier)
	url, _ = addState(url, state)

	auth.SetPKCECookie(w, verifier, s.secure())
	http.Redirect(w, r, url, http.StatusFound)
}

// callback completes the flow and establishes the portal session.
func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		Error(w, http.StatusBadRequest, "globus_error",
			"Globus declined the sign-in: "+e+". Try again, or check that the portal is registered with your identity provider.")
		return
	}

	code := q.Get("code")
	if code == "" {
		Error(w, http.StatusBadRequest, "missing_code",
			"The sign-in response carried no authorization code. Start again from the sign-in page.")
		return
	}

	c, err := r.Cookie(auth.PKCECookie)
	if err != nil {
		Error(w, http.StatusBadRequest, "expired_login",
			"The sign-in took too long and expired. Start again from the sign-in page.")
		return
	}
	if got := q.Get("state"); got != auth.StateHash(c.Value) {
		Error(w, http.StatusBadRequest, "state_mismatch",
			"The sign-in could not be verified. Start again from the sign-in page.")
		return
	}

	sess, err := s.auth.Exchange(r.Context(), code, c.Value)
	if err != nil {
		Error(w, http.StatusBadGateway, "exchange_failed",
			"Globus accepted the sign-in but the portal could not complete it. Try again.")
		return
	}

	auth.SetPKCECookie(w, "", s.secure())
	auth.SetSessionCookie(w, sess, s.secure())
	http.Redirect(w, r, s.cfg.PublicBaseURL+"/#/home", http.StatusFound)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if sess, err := s.auth.Session(r.Context(), r); err == nil {
		_ = s.auth.Logout(r.Context(), sess.ID)
	}
	auth.ClearSessionCookie(w, s.secure())
	JSON(w, http.StatusOK, map[string]string{"status": "signed out"})
}

// meResponse is the P0 payload: who you are, which identities are linked, and
// which Globus groups you belong to. This is the whole P0 acceptance surface.
type meResponse struct {
	Subject      string            `json:"subject"`
	Name         string            `json:"name"`
	Username     string            `json:"username"`
	Email        string            `json:"email"`
	Organization string            `json:"organization"`
	Linked       []linkedIdentity  `json:"linked_identities"`
	Groups       []groupMembership `json:"groups"`
	ExpiresAt    string            `json:"session_expires_at"`
}

type linkedIdentity struct {
	Subject      string `json:"subject"`
	Username     string `json:"username"`
	Organization string `json:"organization"`
	Type         string `json:"type"`
}

type groupMembership struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Parent string `json:"parent_id,omitempty"`
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	sess, err := s.auth.Session(r.Context(), r)
	if err != nil {
		if errors.Is(err, auth.ErrNoSession) {
			Error(w, http.StatusUnauthorized, "no_session", "Sign in to continue.")
			return
		}
		Error(w, http.StatusInternalServerError, "session_error",
			"The portal could not read your session. Sign in again.")
		return
	}

	p := sess.Principal
	out := meResponse{
		Subject:      p.Primary.SubjectID,
		Name:         p.Primary.Name,
		Username:     p.Primary.Username,
		Email:        p.Primary.Email,
		Organization: p.Primary.Organization,
		ExpiresAt:    sess.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	for _, i := range p.LinkedIDs {
		out.Linked = append(out.Linked, linkedIdentity{
			Subject: i.SubjectID, Username: i.Username,
			Organization: i.Organization, Type: i.IdentityType,
		})
	}
	for _, g := range p.Groups {
		out.Groups = append(out.Groups, groupMembership{
			ID: g.ID, Name: g.Name, Role: g.Role, Parent: g.ParentID,
		})
	}
	JSON(w, http.StatusOK, out)
}

// WithSession is middleware placing the session on the request context.
func (s *Server) WithSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := s.auth.Session(r.Context(), r)
		if err != nil {
			Error(w, http.StatusUnauthorized, "no_session", "Sign in to continue.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, sess)))
	})
}

type sessionKey struct{}

// SessionFrom retrieves the session placed by WithSession.
func SessionFrom(ctx context.Context) (*auth.Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(*auth.Session)
	return s, ok
}

func addState(rawURL, state string) (string, error) {
	if strings.Contains(rawURL, "state=") {
		return strings.Replace(rawURL, "state=", "state="+state, 1), nil
	}
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return rawURL + sep + "state=" + state, nil
}
