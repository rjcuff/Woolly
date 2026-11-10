package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const tokenCookie = "woolly_token"

// EnsureToken returns the API token stored in <home>/token, creating it the
// first time. The file is readable only by you, so other programs and other
// users on this computer can't use the API without it.
func EnsureToken(home string) (string, error) {
	path := filepath.Join(home, "token")
	if data, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(data)); len(t) >= 32 {
			return t, nil
		}
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	t := hex.EncodeToString(b[:])
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(t+"\n"), 0o600); err != nil {
		return "", err
	}
	return t, nil
}

func validToken(r *http.Request, token string) bool {
	got := ""
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		got = strings.TrimPrefix(h, "Bearer ")
	} else if c, err := r.Cookie(tokenCookie); err == nil {
		got = c.Value
	}
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

// requireToken protects /api/ routes. /api/version stays open so tools can
// check whether Woolly is running.
func requireToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/version" && !validToken(r, token) {
			writeError(w, http.StatusUnauthorized,
				"missing or wrong token. Open the page with `woolly ui`, or send the token from ~/.woolly/token as a Bearer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authHandler trades the token in the link `woolly ui` opens for a cookie,
// so the token doesn't stay in the address bar.
func authHandler(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.URL.Query().Get("token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			writeError(w, http.StatusForbidden, "wrong token. Open the page with `woolly ui`")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     tokenCookie,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
