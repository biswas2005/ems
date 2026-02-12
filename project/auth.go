package project

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func Login(w http.ResponseWriter, r *http.Request) {

	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPass := os.Getenv("ADMIN_PASS")
	if req.Email != adminEmail {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	if req.Password != adminPass {
		http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
		return
	}

	accessToken, refreshToken, err := GenerateJWT(1, adminEmail)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		Expires:  time.Now().Add(15 * time.Minute),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("Login successful"))

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
			Name:     "access_token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Now().Add(-time.Hour),
		})

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
