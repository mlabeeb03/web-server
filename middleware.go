package main

import "net/http"

func (config *apiConfig) middlewareMetricInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}
