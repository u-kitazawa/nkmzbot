package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	AccessToken string `json:"access_token"`
	jwt.RegisteredClaims
}

// Auth handlers
func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	state := generateRandomString(32)
	url := a.oauthConfig.AuthCodeURL(state)

	json.NewEncoder(w).Encode(map[string]string{
		"auth_url": url,
		"state":    state,
	})
}

func (a *API) authenticateUser(code string) (string, string, string, error) {
	// Exchange code for token
	token, err := a.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		return "", "", "", fmt.Errorf("token exchange failed: %w", err)
	}

	// Get user info
	user, err := a.getDiscordUser(token.AccessToken)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get user: %w", err)
	}

	// Create JWT
	claims := &Claims{
		UserID:      user.ID,
		Username:    user.Username,
		AccessToken: token.AccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := jwtToken.SignedString(a.jwtSecret)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create token: %w", err)
	}

	return tokenString, user.ID, getUsername(user), nil
}

func (a *API) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	tokenString, userID, username, err := a.authenticateUser(code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":    tokenString,
		"user_id":  userID,
		"username": username,
	})
}

func (a *API) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	tokenString, _, _, err := a.authenticateUser(code)
	if err != nil {
		http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusBadGateway)
		return
	}

	// Set HTTP-only cookie with the JWT token
	// NOTE: Secure flag is set to false for local development
	// In production with HTTPS, this should be set to true
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    tokenString,
		Path:     "/",
		MaxAge:   86400, // 24 hours in seconds
		HttpOnly: true,
		Secure:   false, // TODO: Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	})

	// Return success message
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Authentication successful. Token has been set in cookie.",
	})
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Clear the auth cookie
	// NOTE: Secure flag should match the one set during login
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // Delete cookie
		HttpOnly: true,
		Secure:   false, // TODO: Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "logged out",
	})
}

// Middleware
func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenString string

		// First, try to get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				http.Error(w, "invalid authorization header", http.StatusUnauthorized)
				return
			}
		} else {
			// If no Authorization header, try to get token from cookie
			cookie, err := r.Cookie("auth_token")
			if err != nil {
				http.Error(w, "missing authentication", http.StatusUnauthorized)
				return
			}
			tokenString = cookie.Value
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return a.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "claims", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
