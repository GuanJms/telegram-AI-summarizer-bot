package server

import (
	"net/http"
)

func NewHTTPMux(webhook http.HandlerFunc) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/telegram/webhook", webhook)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	return mux
}

func ListenAndServe(addr string, mux *http.ServeMux) error {
	return http.ListenAndServe(addr, mux)
}
