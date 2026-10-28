package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"notes-api/models"
	"notes-api/service"
)

type contextKey string

const (
	UserContextKey   contextKey = "auth_user"
	ClaimsContextKey contextKey = "auth_claims"
	TokenContextKey  contextKey = "auth_token"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("[notes-api] %s %s %d %s", r.Method, r.URL.Path, rw.statusCode, time.Since(start))
	})
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func Auth(authService service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				writeJSONError(w, http.StatusUnauthorized, "invalid authorization header format, expected 'Bearer <token>'")
				return
			}

			tokenStr := parts[1]
			user, claims, err := authService.ValidateToken(r.Context(), tokenStr)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, err.Error())
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			ctx = context.WithValue(ctx, ClaimsContextKey, claims)
			ctx = context.WithValue(ctx, TokenContextKey, tokenStr)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetAuthUser(r *http.Request) *models.UserResponse {
	val := r.Context().Value(UserContextKey)
	if val == nil {
		return nil
	}
	user, ok := val.(*models.UserResponse)
	if !ok {
		return nil
	}
	return user
}

func GetAuthToken(r *http.Request) string {
	val := r.Context().Value(TokenContextKey)
	if val == nil {
		return ""
	}
	tok, ok := val.(string)
	if !ok {
		return ""
	}
	return tok
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

