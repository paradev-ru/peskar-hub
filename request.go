package main

import (
	"net"
	"net/http"
)

func getIP(req *http.Request) string {
	realIPRaw := req.Header.Get("X-Real-Ip")

	if realIPRaw != "" {
		return realIPRaw
	}

	realIPRaw, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return "0.0.0.0"
	}

	return realIPRaw
}
