// Package auth hashes passwords (PBKDF2-SHA256, standard library) and
// issues opaque session tokens (only their SHA-256 is stored).
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Iterations follows the OWASP recommendation for PBKDF2-HMAC-SHA256.
const Iterations = 600_000

// MinPasswordLength is enforced on every new password.
const MinPasswordLength = 10

// ErrWeakPassword is returned for passwords that are too short.
var ErrWeakPassword = fmt.Errorf("le mot de passe doit faire au moins %d caractères", MinPasswordLength)

// HashPassword returns "pbkdf2-sha256$iterations$salt$hash" (base64).
func HashPassword(password string) (string, error) {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return "", ErrWeakPassword
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, Iterations, 32)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", Iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// CheckPassword verifies a password against a stored hash in constant time.
func CheckPassword(stored, password string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1000 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[2])
	want, err2 := enc.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash is checked when a login does not exist, so that response time
// does not reveal which logins exist.
var dummyHash, _ = HashPassword("pairing-easy-dummy-password")

// CheckDummy burns the same time as a real password check.
func CheckDummy(password string) { CheckPassword(dummyHash, password) }

// NewToken returns a random session token and the hash to store.
func NewToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken returns the stored form of a token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RandomPIN returns a numeric PIN of n digits.
func RandomPIN(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	for i := range b {
		b[i] = '0' + b[i]%10
	}
	return string(b)
}

// RandomID returns a URL-safe random identifier of n bytes of entropy.
func RandomID(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return strings.ToLower(base64.RawURLEncoding.EncodeToString(b))
}

// ErrInvalidCredentials hides whether the login or the password was wrong.
var ErrInvalidCredentials = errors.New("identifiant ou mot de passe incorrect")
