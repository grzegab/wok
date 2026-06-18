package router

import (
	"net/http"

	sentryhttp "github.com/getsentry/sentry-go/http"
)

func SentryHandler() func(next http.Handler) http.Handler {
	sh := sentryhttp.New(sentryhttp.Options{
		Repanic: true,
	})

	return func(next http.Handler) http.Handler {
		return sh.HandleFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}
