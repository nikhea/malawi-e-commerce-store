package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// OTP returns a 6-digit numeric code for email verification. Short-lived
// (15 min) and attempt-counted in the repo — brute force is uneconomical.
func OTP() (string, error) {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	n := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	return fmt.Sprintf("%06d", n%1000000), nil
}

// Token returns 32 random bytes, base64url-encoded (password-reset links
// and refresh tokens). 256 bits: unguessable, no expiry shortcut needed.
func Token() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// HashSecret fingerprints OTP codes and tokens for storage. SHA256 (not
// bcrypt): these secrets have 256-bit entropy, so a fast hash suffices
// and lookups stay indexed equality checks. The raw secret travels only
// inside events and emails — never the DB, never logs.
func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// OTPExpiry and ResetExpiry bound secret lifetimes. Short windows shrink
// the replay surface if an inbox is compromised later.
const (
	OTPExpiry          = 15 * time.Minute
	ResetExpiry        = 1 * time.Hour
	RefreshExpiry      = 30 * 24 * time.Hour
	MinPasswordLength  = 8
)
