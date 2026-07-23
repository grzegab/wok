package handlers

import (
	"context"
	"log"
	"net"
	"net/http"
)

type BannedIps interface {
	Add(ctx context.Context, ip string, reason string) error
}

func Ban(ipRepo BannedIps, reason string, w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	if ip != "" {
		err := ipRepo.Add(r.Context(), ip, reason)
		if err != nil {
			log.Printf("could not save IP %s: %v", ip, err)
		}
	}

	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("404 page not found"))
}
