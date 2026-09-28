// Package fakegithub is a test double of GitHub's OAuth web flow and /user endpoint.
// It is used by Go tests and the Compose e2e check; it is never part of the production image.
package fakegithub

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type grant struct {
	challenge   string
	redirectURI string
	user        User
}

// Server auto-approves authorization for the current user (set with SetUser or POST /_fake/user).
// A user with ID 0 simulates the visitor denying access.
type Server struct {
	ClientID, ClientSecret string

	mu     sync.Mutex
	user   User
	codes  map[string]grant
	tokens map[string]User
}

func New(clientID, clientSecret string, user User) *Server {
	return &Server{ClientID: clientID, ClientSecret: clientSecret, user: user,
		codes: map[string]grant{}, tokens: map[string]User{}}
}

func (s *Server) SetUser(u User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.user = u
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login/oauth/authorize", s.authorize)
	mux.HandleFunc("POST /login/oauth/access_token", s.token)
	mux.HandleFunc("GET /user", s.me)
	mux.HandleFunc("POST /_fake/user", s.setUser)
	return mux
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	redirect, err := url.Parse(q.Get("redirect_uri"))
	if q.Get("client_id") != s.ClientID || err != nil || q.Get("state") == "" ||
		q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		http.Error(w, "invalid authorize request", http.StatusBadRequest)
		return
	}
	back := redirect.Query()
	back.Set("state", q.Get("state"))

	s.mu.Lock()
	user := s.user
	if user.ID == 0 {
		back.Set("error", "access_denied")
	} else {
		code := random()
		s.codes[code] = grant{challenge: q.Get("code_challenge"), redirectURI: q.Get("redirect_uri"), user: user}
		back.Set("code", code)
	}
	s.mu.Unlock()

	redirect.RawQuery = back.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	f := r.PostForm
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.codes[f.Get("code")]
	delete(s.codes, f.Get("code")) // codes are single-use, like GitHub's
	sum := sha256.Sum256([]byte(f.Get("code_verifier")))
	switch {
	case f.Get("client_id") != s.ClientID || f.Get("client_secret") != s.ClientSecret:
		writeJSON(w, map[string]string{"error": "incorrect_client_credentials"})
	case !ok || f.Get("redirect_uri") != g.redirectURI:
		writeJSON(w, map[string]string{"error": "bad_verification_code"})
	case base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge:
		writeJSON(w, map[string]string{"error": "invalid_grant"})
	default:
		tok := random()
		s.tokens[tok] = g.user
		writeJSON(w, map[string]string{"access_token": tok, "token_type": "bearer", "scope": ""})
	}
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	u, ok := s.tokens[stripBearer(r.Header.Get("Authorization"))]
	s.mu.Unlock()
	if !ok {
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
		return
	}
	writeJSON(w, u)
}

func (s *Server) setUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	s.SetUser(User{ID: id, Login: r.URL.Query().Get("login")})
	w.WriteHeader(http.StatusNoContent)
}

func stripBearer(h string) string {
	const p = "Bearer "
	if len(h) > len(p) && h[:len(p)] == p {
		return h[len(p):]
	}
	return ""
}

func random() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
