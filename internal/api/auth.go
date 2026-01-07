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

func (a *API) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := a.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, fmt.Sprintf("token exchange failed: %v", err), http.StatusBadGateway)
		return
	}

	// Get user info
	user, err := a.getDiscordUser(token.AccessToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get user: %v", err), http.StatusBadGateway)
		return
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
		http.Error(w, "failed to create token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":    tokenString,
		"user_id":  user.ID,
		"username": getUsername(user),
	})
}

func (a *API) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := a.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Redirect(w, r, "/login?error=token_exchange_failed", http.StatusSeeOther)
		return
	}

	// Get user info
	user, err := a.getDiscordUser(token.AccessToken)
	if err != nil {
		http.Redirect(w, r, "/login?error=failed_to_get_user", http.StatusSeeOther)
		return
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
		http.Redirect(w, r, "/login?error=failed_to_create_token", http.StatusSeeOther)
		return
	}

	// Redirect to login page with token in URL fragment
	http.Redirect(w, r, "/login?success=true#token="+tokenString, http.StatusSeeOther)
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "logged out",
	})
}

// Middleware
func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
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
