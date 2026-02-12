package project

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	//JWT Library
	"github.com/golang-jwt/jwt/v5"
)

// Login handler: authenticates admin user and issues JWT tokens
func Login(w http.ResponseWriter, r *http.Request) {

	//Decode JSON body into LoginRequest struct
	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	//Fetch Admin credentials from Environment variables
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPass := os.Getenv("ADMIN_PASS")
	//Validate Email
	if req.Email != adminEmail {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	//Validate Password
	if req.Password != adminPass {
		http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
		return
	}

	//Generate access and refresh tokens
	accessToken, refreshToken, err := GenerateJWT(1, adminEmail)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	//Set access token cookie (short-lived)
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		Expires:  time.Now().Add(15 * time.Minute),
	})

	//Set refresh token cookie (long-lived)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})
	//Respond with success message
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("Login successful"))

}

// RefreshHandler: issues new access token using refresh token
func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	//Get refresh token from cookie
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "No refresh token", http.StatusUnauthorized)
		return
	}
	refreshToken := cookie.Value

	//Lookup user ID from redis using refresh token
	userIDStr, err := rdb.Get(ctx, "refresh:"+refreshToken).Result()
	if err != nil {
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}
	//Validate refresh token signature
	token, err := jwt.Parse(refreshToken, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "Invalid Refresh token", http.StatusUnauthorized)
		return
	}
	//Convert userID string to integer
	userID, _ := strconv.Atoi(userIDStr)
	//Generate new access token
	newAccess, _, err := GenerateJWT(userID, "")
	if err != nil {
		http.Error(w, "Could not generate token", http.StatusInternalServerError)
		return
	}
	//Generate new access token in JSON response
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": newAccess,
	})
}

// Logout Handler: invalidate token and clears cookies
func Logout(w http.ResponseWriter, r *http.Request) {
	//Extract access token from Authorization header
	parts := strings.Split(r.Header.Get("Authorization"), " ")
	if len(parts) != 2 {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	accessToken := parts[1]

	//Blacklist access token in Redis for 15 minutes
	rdb.Set(ctx, "bl:"+accessToken, 1, time.Minute*15)

	//Delete refresh token from Redis if present
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		rdb.Del(ctx, "refresh:"+cookie.Value)

		//Expire access token cookie immediately
		http.SetCookie(w, &http.Cookie{
			Name:     "access_token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Now().Add(-time.Hour),
		})

		//Expire refresh token cookie immediately
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Now().Add(-time.Hour),
		})
	}
	//Respond with logout confirmation
	w.Write([]byte("Logged out"))
}
