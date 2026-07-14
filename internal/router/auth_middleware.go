package router

import (
	"context"
	"log"
	"net/http"

	"github.com/grzegab/wok/internal/app"
)

type sessionValidator interface {
	Validate(ctx context.Context, token string) (bool, error)
}

func RequireLogin(sessionRepo sessionValidator) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(app.SessionCookieName)
			if err != nil {
				http.NotFound(w, r)
				return
			}

			valid, err := sessionRepo.Validate(r.Context(), cookie.Value)
			if err != nil {
				log.Printf("could not validate session: %v", err)
				valid = false
			}

			if !valid {
				http.NotFound(w, r)
				return
			}

			// TODO: add cookie session to ctx

			next.ServeHTTP(w, r)
		})
	}
}
