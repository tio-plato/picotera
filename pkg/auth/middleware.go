package auth

import (
	"net/http"
)

// Middleware returns a chi middleware that authenticates internal management
// API requests. It is mounted only on the /api/picotera sub-router (see
// server.go), so the gateway catch-all, /api/unified, and static assets never
// reach it — they authenticate via API key. On success the resolved user is
// stored on the request context; on failure a 401 JSON response is written and
// the handler chain is short-circuited.
func Middleware(resolver *Resolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := resolver.Resolve(r.Context(), r)
			if err != nil || user == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
				return
			}

			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
		})
	}
}
