package project

import (
	"context"
	"fmt"
	"net/http"
	"time"

	//JWT Library
	"github.com/golang-jwt/jwt/v5"
)

// GenerateJWT creates and signs both access and refresh token for user
func GenerateJWT(userID int, email string) (string, string, error) {

	//Define Claims for the access token(short-lived)
	accessClaims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	//Create a new JWT with HS256 signing method
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)

	//Sign the access token with secret key
	accessSigned, err := accessToken.SignedString(jwtSecret)
	if err != nil {
		return "", "", err
	}

	//Define Claims for the refresh token(long-lived)
	refreshClaims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   fmt.Sprintf("%d", userID),
	}

	//Create a new refresh token
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	//Sign the refresh token
	refreshSigned, err := refreshToken.SignedString(jwtSecret)
	if err != nil {
		return "", "", err
	}
	//Store the refresh token in Redis with expiry(for validation later)
	err = rdb.Set(ctx, "refresh:"+refreshSigned, userID, 7*24*time.Hour).Err()
	if err != nil {
		return "", "", err
	}
	//Return both signed tokens
	return accessSigned, refreshSigned, nil
}

// JwtMiddleware validates JWT tokens before allowing access to protected routes
func JwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		//Retrieve access token from cookie
		cookie, err := r.Cookie("access_token")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		tokenStr := cookie.Value

		//Check if token is blacklisted in Redis
		if rdb.Exists(ctx, "bl:"+tokenStr).Val() == 1 {
			http.Error(w, "Token revoked", http.StatusUnauthorized)
			return
		}

		//Parse token into Claims struct
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			//Ensure token using HMAC signing method
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return jwtSecret, nil
		})

		//Reject if token is invalid or parsing failed
		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		//Add user info to request context for downstream handlers
		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "email", claims.Email)
		//Pass request along with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
