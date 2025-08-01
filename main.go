package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/mlabeeb03/web-server/internal/database"
)

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Could not open database connection: ", err)
	}

	mux := http.NewServeMux()

	jwtSecret := os.Getenv("JWT_SECRET")

	apiCfg := &apiConfig{fileserverHits: atomic.Int32{}, db: database.New(db), jwtSecret: jwtSecret}

	mux.HandleFunc("GET /admin/metrics", apiCfg.metrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.reset)

	mux.Handle("/app/", apiCfg.middlewareMetricInc(http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))

	mux.HandleFunc("GET /api/healthz", healthz)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getChirp)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.deleteChirp)
	mux.HandleFunc("GET /api/chirps", apiCfg.getAllChirps)
	mux.HandleFunc("POST /api/chirps", apiCfg.chirps)
	mux.HandleFunc("POST /api/users", apiCfg.createUser)
	mux.HandleFunc("PUT /api/users", apiCfg.modifyUser)
	mux.HandleFunc("POST /api/login", apiCfg.login)
	mux.HandleFunc("POST /api/refresh", apiCfg.handlerRefresh)
	mux.HandleFunc("POST /api/revoke", apiCfg.handlerRevoke)

	port := "8080"
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Could not ListenAndServe: ", err)
	}
}
