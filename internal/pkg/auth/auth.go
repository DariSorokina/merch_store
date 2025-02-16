// Package auth provides functionality for handling JSON Web Token (JWT) based authentication.
// It includes middleware for validating JWT tokens in HTTP requests and parsing tokens to extract claims.
package auth

import (
	"context"
	"encoding/json"
	"merch_store/internal/models"
	"net/http"
	"strings"
)

// contextKey is a custom type used for storing values in a context without risking collisions.
type contextKey string

// ContextUserID is the key used to store and retrieve the user ID from the request context.
const ContextUserID contextKey = "сontextUserID"

// CheckJWTMiddleware is an HTTP middleware function that validates the Authorization header of incoming requests.
// It checks for the presence of a Bearer token, parses the token to extract the user ID, and stores it in the request context.
// If validation fails at any point, it returns an error response with the appropriate HTTP status code.
func CheckJWTMiddleware() func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				writeErrorResponse(w, "missing auth header", http.StatusUnauthorized)
				return
			}
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeErrorResponse(w, "invalid auth header", http.StatusUnauthorized)
				return
			}

			claims, err := ParseToken(parts[1])
			if err != nil {
				writeErrorResponse(w, "invalid token", http.StatusUnauthorized)
				return
			}

			// Store the user ID from the token claims into the request context.
			ctx := context.WithValue(r.Context(), ContextUserID, claims.UserID)
			h.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}

// writeErrorResponse writes a JSON-formatted error response to the HTTP response writer.
// It sets the Content-Type header, writes the appropriate HTTP status code, and encodes an ErrorResponse payload.
func writeErrorResponse(res http.ResponseWriter, errorInfo string, statusCode int) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(statusCode)
	json.NewEncoder(res).Encode(models.ErrorResponse{Errors: errorInfo})
}
