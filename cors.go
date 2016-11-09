package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

const (
	methods = "POST, GET, OPTIONS, PUT, DELETE"
	headers = "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization"
)

type WithCORS struct {
	r *mux.Router
}

func (s *WithCORS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", methods)
		w.Header().Set("Access-Control-Allow-Headers", headers)
	}

	if r.Method == "OPTIONS" {
		return
	}

	w.Header().Add("Content-Type", "application/json")
	s.r.ServeHTTP(w, r)
}
