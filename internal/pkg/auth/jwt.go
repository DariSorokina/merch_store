// Package auth provides functionality for generating and parsing JSON Web Tokens (JWT)
// for user authentication. It defines custom claims, token generation, and validation logic.
package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// secretKey is the key used to sign the JWT. It should be kept secure.
var secretKey = []byte("supersecretkey")

// TOKENEXP defines the token expiration duration.
const TOKENEXP = time.Hour * 3

// SECRETKEY is a string constant representation of the secret key.
const SECRETKEY = "supersecretkey"

// Claims represents the custom JWT claims that include the user ID and standard claims.
// It embeds jwt.RegisteredClaims for standard fields like expiration time.
type Claims struct {
	UserID int32
	jwt.RegisteredClaims
}

// GenerateToken creates a new JWT token for a given userID.
// It sets the expiration time based on TOKENEXP and includes the userID in the claims.
func GenerateToken(userID int32) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TOKENEXP)),
		},
		UserID: userID,
	}
	// Create a new token with HS256 signing method and the specified claims.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Sign the token using the secret key and return the signed token string.
	return token.SignedString(secretKey)
}

// ParseToken validates the provided JWT token string and parses its claims.
// It returns the Claims if the token is valid, or an error otherwise.
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})
	if err != nil {
		return nil, err
	}
	// Validate and extract the claims.
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	// Return an error if the token signature is invalid.
	return nil, jwt.ErrSignatureInvalid
}
