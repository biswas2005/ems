package project

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func Register(w http.ResponseWriter, r *http.Request) {
	var u Users
	err := json.NewDecoder(r.Body).Decode(&u)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if !strings.Contains(u.Email, "@") || len(u.Password) < 6 {
		http.Error(w, "Invalid input", 404)
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	_, err = db.Exec("INSERT INTO users(email,password) VALUES (?,?)", u.Email, string(hash))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte("User registered"))
}

func Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var userID int
	var hash string

	err = db.QueryRow(
		"SELECT id,password FROM users WHERE email=?", req.Email,
	).Scan(&userID, &hash)

	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, ttl, err := GenerateJWT(userID, req.Email)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(map[string]string{
		"access_token": token,
		"expires_in":   ttl.String(),
	})
}

func Logout(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.Header.Get("Authorization"), " ")
	if len(parts) != 2 {
		http.Error(w, "Invalid token", 401)
		return
	}
	token := parts[1]

	rdb.Set(ctx, "bl:"+token, "revoked", time.Minute*15)
	w.Write([]byte("Logged out"))
}
