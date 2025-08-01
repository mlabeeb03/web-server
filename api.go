package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/mlabeeb03/web-server/internal/auth"
	"github.com/mlabeeb03/web-server/internal/database"
)

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func (config *apiConfig) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprintf(`<html>
									<body>
										<h1>Welcome, Chirpy Admin</h1>
										<p>Chirpy has been visited %d times!</p>
									</body>
								</html>`, config.fileserverHits.Load())))
}

func (config *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	type parameter struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameter{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}
	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't hash password", err)
		return
	}
	dbUser, err := config.db.CreateUser(r.Context(), database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create user", err)
		return
	}
	user := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}
	respondWithJSON(w, http.StatusCreated, user)
}

func (config *apiConfig) modifyUser(w http.ResponseWriter, r *http.Request) {
	type parameter struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameter{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}
	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't hash password", err)
		return
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "User is not authorized", nil)
		return
	}
	UserID, err := auth.ValidateJWT(token, config.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "User is not authorized", nil)
		return
	}
	dbUser, err := config.db.UpdateUser(r.Context(), database.UpdateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
		ID:             UserID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update user", err)
		return
	}
	respondWithJSON(w, http.StatusOK, User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	})
}

func (config *apiConfig) login(w http.ResponseWriter, r *http.Request) {
	type parameter struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type AuthResponse struct {
		User         User   `json:"user"`
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameter{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}
	dbUser, err := config.db.GetUser(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find user", err)
		return
	}
	if auth.CheckPasswordHash(params.Password, dbUser.HashedPassword) != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password", err)
		return
	}
	token, err := auth.MakeJWT(dbUser.ID, config.jwtSecret, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not create JWT", err)
		return
	}
	refreshTokenString, err := auth.MakeRefreshToken()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not create refresh token", err)
		return
	}
	refreshToken, err := config.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:  refreshTokenString,
		UserID: dbUser.ID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not create refresh token", err)
		return
	}
	user := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}
	respondWithJSON(w, http.StatusOK, AuthResponse{
		User:         user,
		Token:        token,
		RefreshToken: refreshToken.Token,
	})
}

func (config *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	platform := os.Getenv("PLATFORM")
	if platform != "dev" {
		respondWithError(w, http.StatusForbidden, "Method only allowed on dev", nil)
		return
	}
	config.db.DeleteAllUsers(r.Context())
	w.WriteHeader(200)
}

func (config *apiConfig) chirps(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "User is not authorized", nil)
		return
	}
	UserID, err := auth.ValidateJWT(token, config.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "User is not authorized", nil)
		return
	}
	const maxChirpLen int = 140
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
	} else if utf8.RuneCountInString(params.Body) > maxChirpLen {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long", nil)
	} else {
		badWords := map[string]bool{
			"kerfuffle": true,
			"sharbert":  true,
			"fornax":    true,
		}
		words := strings.Fields(params.Body)
		for i, word := range words {
			lowered := strings.ToLower(word)
			if badWords[lowered] {
				words[i] = "****"
			}
		}
		output := strings.Join(words, " ")
		params.Body = output
		dbChirp, err := config.db.CreateChirp(r.Context(), database.CreateChirpParams{
			Body:   params.Body,
			UserID: UserID,
		})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Couldn't create chirp", err)
			return
		}
		chirp := Chirp{
			ID:        dbChirp.ID,
			CreatedAt: dbChirp.CreatedAt,
			UpdatedAt: dbChirp.UpdatedAt,
			Body:      dbChirp.Body,
			UserID:    dbChirp.UserID,
		}
		respondWithJSON(w, http.StatusCreated, chirp)
	}
}

func (config *apiConfig) getChirp(w http.ResponseWriter, r *http.Request) {
	uuid, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse uuid", err)
		return
	}
	dbChirp, err := config.db.GetChirp(r.Context(), uuid)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't fetch chirp", err)
		return
	}
	chirp := Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}
	respondWithJSON(w, http.StatusOK, chirp)
}

func (config *apiConfig) deleteChirp(w http.ResponseWriter, r *http.Request) {
	uuid, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse uuid", err)
		return
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "User is not authorized", nil)
		return
	}
	UserID, err := auth.ValidateJWT(token, config.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "User is not authorized", nil)
		return
	}
	chirp, err := config.db.GetChirp(r.Context(), uuid)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Chirp not found", nil)
		return
	}
	if UserID != chirp.UserID {
		respondWithError(w, http.StatusForbidden, "User is not authorized", err)
		return
	}
	err = config.db.DeleteChirp(r.Context(), uuid)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not delete chirp", err)
		return
	}
	respondWithJSON(w, http.StatusNoContent, nil)
}

func (config *apiConfig) getAllChirps(w http.ResponseWriter, r *http.Request) {
	dbChirps, err := config.db.GetAllChirps(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't fetch chirps", err)
		return
	}
	chirps := []Chirp{}
	for _, chirp := range dbChirps {
		chirps = append(chirps, Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		})
	}
	respondWithJSON(w, http.StatusOK, chirps)
}

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	type response struct {
		Token string `json:"token"`
	}

	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find token", err)
		return
	}

	user, err := cfg.db.GetUserFromRefreshToken(r.Context(), refreshToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't get user for refresh token", err)
		return
	}

	accessToken, err := auth.MakeJWT(
		user.ID,
		cfg.jwtSecret,
		time.Hour,
	)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate token", err)
		return
	}

	respondWithJSON(w, http.StatusOK, response{
		Token: accessToken,
	})
}

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find token", err)
		return
	}

	_, err = cfg.db.RevokeRefreshToken(r.Context(), refreshToken)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't revoke session", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
