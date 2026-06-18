package router

import (
	"fmt"
	"net"
	"net/http"

	"github.com/grzegab/wok/internal/repository"
)

func CheckIP(ipRepo repository.IpRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var ip string

			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				fmt.Println("Checking IP address: ", r.RemoteAddr)
				ip = r.RemoteAddr
			}

			banned, err := ipRepo.Exists(r.Context(), ip)
			if err != nil {
				fmt.Printf("Could not check IP %s: %v\n", ip, err)
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
