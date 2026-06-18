package router

import (
	"fmt"
	"net"
	"net/http"

	"github.com/grzegab/wok/internal/repository"
)

func ban(ipRepo repository.IpRepository, reason string, w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	if ip != "" {
		err := ipRepo.Add(r.Context(), ip, reason)
		if err != nil {
			fmt.Printf("Could not save IP %s: %v\n", ip, err)
		}
	}

	w.WriteHeader(http.StatusTeapot)
	w.Write([]byte("Banned"))
}
