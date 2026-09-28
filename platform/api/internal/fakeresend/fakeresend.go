// Package fakeresend is a test double of the Resend send-email API (POST /emails) with its
// idempotency semantics. Test-only: never deployed, never talks to Resend.
package fakeresend

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Step scripts the next answer. Accept=true stores the email first (the provider did accept),
// which lets tests simulate "accepted, then the answer was lost" with Status >= 500 or Delay.
type Step struct {
	Status     int           `json:"status"`
	Name       string        `json:"name"`
	RetryAfter int           `json:"retry_after"`
	Delay      time.Duration `json:"delay"`
	Accept     bool          `json:"accept"`
	BadBody    bool          `json:"bad_body"`
}

type Email struct {
	ID             string          `json:"id"`
	IdempotencyKey string          `json:"idempotency_key"`
	Body           json.RawMessage `json:"body"`
	At             time.Time       `json:"at"`
}

type Server struct {
	apiKey string
	mu     sync.Mutex
	script []Step
	sent   []Email          // accepted emails, in order (one per key)
	byKey  map[string]entry // idempotency memory
	calls  int
}

type entry struct {
	body []byte
	id   string
}

func New(apiKey string) *Server { return &Server{apiKey: apiKey, byKey: map[string]entry{}} }

// Script appends scripted answers consumed by the next calls (default: accept).
func (s *Server) Script(steps ...Step) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.script = append(s.script, steps...)
}

func (s *Server) Sent() []Email {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Email(nil), s.sent...)
}

func (s *Server) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func writeErr(w http.ResponseWriter, status int, name string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"statusCode": status, "name": name, "message": name})
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /emails", s.send)
	// Test controls (the e2e drives them over HTTP).
	mux.HandleFunc("GET /_emails", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.Sent())
	})
	mux.HandleFunc("POST /_script", func(w http.ResponseWriter, r *http.Request) {
		var steps []Step
		if err := json.NewDecoder(r.Body).Decode(&steps); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Script(steps...)
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func (s *Server) send(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	s.mu.Lock()
	s.calls++
	var step Step
	if len(s.script) > 0 {
		step, s.script = s.script[0], s.script[1:]
	}
	s.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+s.apiKey {
		writeErr(w, http.StatusUnauthorized, "missing_api_key")
		return
	}
	var p struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}
	if json.Unmarshal(body, &p) != nil || p.From == "" || len(p.To) == 0 || p.Subject == "" || p.Text == "" {
		writeErr(w, http.StatusUnprocessableEntity, "missing_required_field")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) > 256 {
		writeErr(w, http.StatusBadRequest, "invalid_idempotency_key")
		return
	}
	accept := func() string {
		s.mu.Lock()
		defer s.mu.Unlock()
		if key != "" {
			if e, ok := s.byKey[key]; ok {
				return e.id
			}
		}
		id := newID()
		if key != "" {
			s.byKey[key] = entry{body: body, id: id}
		}
		s.sent = append(s.sent, Email{ID: id, IdempotencyKey: key, Body: json.RawMessage(body), At: time.Now().UTC()})
		return id
	}
	if key != "" {
		s.mu.Lock()
		e, seen := s.byKey[key]
		s.mu.Unlock()
		if seen && !bytes.Equal(e.body, body) {
			writeErr(w, http.StatusConflict, "invalid_idempotent_request")
			return
		}
	}
	if step.Accept {
		accept()
	}
	if step.Delay > 0 {
		select {
		case <-time.After(step.Delay):
		case <-r.Context().Done():
			return
		}
	}
	if step.Status != 0 && step.Status != http.StatusOK {
		if step.RetryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(step.RetryAfter))
		}
		writeErr(w, step.Status, step.Name)
		return
	}
	id := accept()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", "req_"+id[:8])
	if step.BadBody {
		_, _ = w.Write([]byte(`{"oops":`))
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
}
