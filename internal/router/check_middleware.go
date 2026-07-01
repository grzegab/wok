package router

import (
	"context"
	"log"
	"net"
	"net/http"
)

type ipExists interface {
	Exists(ctx context.Context, ip string) (bool, error)
}

func CheckIP(ipRepo ipExists) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var ip string

			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				log.Printf("could not split host/port for %q, using raw remote addr", r.RemoteAddr)
				ip = r.RemoteAddr
			}

			banned, err := ipRepo.Exists(r.Context(), ip)
			if err != nil {
				log.Printf("could not check IP %s: %v", ip, err)
			}

			if banned {
				w.WriteHeader(http.StatusTeapot)
				w.Write([]byte("Banned"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
