package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"
)

// randomToken returns 32 random bytes, base64url-encoded without padding.
func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error on supported platforms.
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// pkceChallenge derives the S256 code_challenge from a verifier (RFC 7636).
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func equalTokens(a, b string) bool {
	return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// safeReturnTo keeps post-login redirects inside the admin area of this origin.
func safeReturnTo(v string) string {
	if v == "/admin" || (strings.HasPrefix(v, "/admin/") && !strings.ContainsAny(v, "\\\r\n") && !strings.Contains(v, "//")) {
		return v
	}
	return "/admin"
}
