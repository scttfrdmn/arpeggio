package auth

import (
	"net/http"
	"time"
)

// SessionCookie is the portal session cookie name.
const SessionCookie = "arp_session"

// SetSessionCookie writes the session cookie. SameSite=Lax rather than Strict:
// the Globus callback is a top-level cross-site navigation and Strict would
// drop the cookie on the redirect back.
func SetSessionCookie(w http.ResponseWriter, s *Session, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    s.ID,
		Path:     "/",
		Expires:  s.ExpiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie expires the session cookie.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// PKCECookie carries the PKCE verifier across the redirect to Globus. It is
// short-lived and deleted on callback.
const PKCECookie = "arp_pkce"

// SetPKCECookie stores the PKCE verifier for the duration of the login round trip.
func SetPKCECookie(w http.ResponseWriter, verifier string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     PKCECookie,
		Value:    verifier,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
