package main

import (
	"fmt"
	"net/http"
)

func (config *apiConfig) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprintf("Hits: %d", config.fileserverHits.Load())))
}

func (config *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	config.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprintf("Hits: %d", config.fileserverHits.Load())))
}
