// Package security provides functionality for handling password hashing and verification.
// It leverages the bcrypt algorithm to securely hash passwords and compare hashed values.
package security

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword takes a plaintext password and returns its bcrypt hash.
// If an error occurs during hashing, it logs the error and returns the resulting hash as a string.
func HashPassword(password string) string {
	passwordBytes := []byte(password)
	hash, err := bcrypt.GenerateFromPassword(passwordBytes, bcrypt.DefaultCost)
	if err != nil {
		log.Print(err.Error())
	}
	return string(hash)
}

// CheckPassword compares a bcrypt hashed password with its possible plaintext equivalent.
// It returns nil on success, or an error on failure indicating that the passwords do not match.
func CheckPassword(hashedPassword, userPassword string) error {
	hashedPasswordBytes := []byte(hashedPassword)
	userPasswordBytes := []byte(userPassword)

	err := bcrypt.CompareHashAndPassword(hashedPasswordBytes, userPasswordBytes)
	return err
}
