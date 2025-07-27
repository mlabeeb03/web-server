package main

import (
	"log"
	"net/http"
	"sync/atomic"
)

func main() {
	mux := http.NewServeMux()

	apiCfg := &apiConfig{fileserverHits: atomic.Int32{}}

	mux.Handle("/app/", apiCfg.middlewareMetricInc(http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /metrics", apiCfg.metrics)
	mux.HandleFunc("POST /reset", apiCfg.reset)

	port := "8080"
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Could not ListenAndServe: ", err)
	}
}
