package httpapi

import (
	"net/http"
	"net/url"
)

func IsSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// sameOrigin rejects unsafe requests whose Origin (or, if absent, Referer) is not the public
// origin. Combined with SameSite=Lax cookies and CSRF tokens, it blocks cross-site mutations.
// CORS stays closed: no Access-Control-Allow-* headers are ever sent.
func sameOrigin(publicOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsSafeMethod(r.Method) && requestOrigin(r) != publicOrigin {
			WriteError(w, r, http.StatusForbidden, "origin_rejected", "Cross-origin request rejected")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestOrigin(r *http.Request) string {
	if o := r.Header.Get("Origin"); o != "" {
		return o
	}
	u, err := url.Parse(r.Header.Get("Referer"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
