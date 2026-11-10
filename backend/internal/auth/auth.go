// Package auth handles password hashing and session tokens.
//
// Passwords are bcrypt-hashed. Session tokens are random 256-bit values handed to
// the client in a cookie; only their SHA-256 hash is stored, so a database leak
// does not yield usable sessions.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// SessionTTL is how long a session stays valid without re-authentication.
const SessionTTL = 30 * 24 * time.Hour

// MinPasswordLength is deliberately a length floor rather than a composition rule:
// length dominates for offline-attack resistance, and composition rules mostly
// produce predictable substitutions.
const MinPasswordLength = 10

// ErrInvalidCredentials is returned for both unknown accounts and wrong
// passwords. Callers must not distinguish them — doing so enumerates accounts.
var ErrInvalidCredentials = errors.New("invalid credentials")

// BcryptCost is the work factor for new hashes. Production keeps the library
// default; test binaries lower it (bcrypt.MinCost) because every test that
// creates a user would otherwise pay ~60ms (~600ms under -race) for nothing.
// Stored hashes carry their own cost, so verification is unaffected.
var BcryptCost = bcrypt.DefaultCost

// HashPassword bcrypt-hashes a password after validating it.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

// CheckPassword verifies a password against a stored hash. It always returns
// ErrInvalidCredentials on mismatch so callers cannot leak which part failed.
func CheckPassword(hash, password string) error {
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// ValidatePassword enforces the length floor.
func ValidatePassword(password string) error {
	if len([]rune(password)) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if len(password) > 1024 {
		// bcrypt truncates at 72 bytes anyway; this just bounds the work.
		return errors.New("password is too long")
	}
	return nil
}

// NormalizeEmail trims and lowercases an address, and checks it parses.
func NormalizeEmail(email string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" {
		return "", errors.New("email is required")
	}
	if _, err := mail.ParseAddress(e); err != nil {
		return "", fmt.Errorf("invalid email address")
	}
	return e, nil
}

// NewSessionToken mints a random token and returns it with its storage hash.
// The plaintext goes to the client; only the hash is persisted.
func NewSessionToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken returns the storage hash for a session token. Plain SHA-256 is
// correct here: the token is already 256 bits of entropy, so there is nothing to
// brute-force and a slow KDF would only cost latency on every request.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SessionExpiry returns the expiry for a session created now.
func SessionExpiry() time.Time { return time.Now().UTC().Add(SessionTTL) }
