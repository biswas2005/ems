package project

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	var user Users
	err = db.QueryRow("SELECT id,email,password FROM users WHERE email=?", req.Email).Scan(&user.ID, &user.Email, &user.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
		return
	}

	accessToken, refreshToken, err := GenerateJWT(user.ID, user.Email)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": accessToken,
	})

}

func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "No refresh token", http.StatusUnauthorized)
		return
	}
	refreshToken := cookie.Value

	userIDStr, err := rdb.Get(ctx, "refresh:"+refreshToken).Result()
	if err != nil {
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	token, err := jwt.Parse(refreshToken, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "Invalid Refresh token", http.StatusUnauthorized)
		return
	}
	userID, _ := strconv.Atoi(userIDStr)
	newAccess, _, err := GenerateJWT(userID, "")
	if err != nil {
		http.Error(w, "Could not generate token", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": newAccess,
	})
}

func Logout(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.Header.Get("Authorization"), " ")
	if len(parts) != 2 {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	accessToken := parts[1]

	rdb.Set(ctx, "bl:"+accessToken, 1, time.Minute*15)

	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		rdb.Del(ctx, "refresh:"+cookie.Value)

		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Now().Add(-time.Hour),
		})
	}
	w.Write([]byte("Logged out"))
}
