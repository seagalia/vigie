package app

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/seagalia/vigie/config"
)

// requireBasicAuth lets a request through when it carries the username and password of one of the users; /healthz
// and the gRPC health service stay open, since probes, such as the grpc probe of Kubernetes, send no credentials
func requireBasicAuth(users []config.BasicAuthUser, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isHealthCheck := r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/grpc.health.v1.Health/")
		if isHealthCheck {
			next.ServeHTTP(w, r)
			return
		}
		username, password, hasCredentials := r.BasicAuth()
		if !hasCredentials || !isKnownUser(users, username, password) {
			w.Header().Set("WWW-Authenticate", `Basic realm="vigie", charset="UTF-8"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isKnownUser compares hashes in constant time and checks every user, so the time of the answer tells nothing about
// how much of a username or a password was right
func isKnownUser(users []config.BasicAuthUser, username, password string) bool {
	usernameHash := sha256.Sum256([]byte(username))
	passwordHash := sha256.Sum256([]byte(password))
	known := 0
	for _, user := range users {
		expectedUsername := sha256.Sum256([]byte(user.Username))
		expectedPassword := sha256.Sum256([]byte(user.Password))
		sameUsername := subtle.ConstantTimeCompare(usernameHash[:], expectedUsername[:])
		samePassword := subtle.ConstantTimeCompare(passwordHash[:], expectedPassword[:])
		known |= sameUsername & samePassword
	}
	return known == 1
}
