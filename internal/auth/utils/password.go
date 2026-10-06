package utils

import "golang.org/x/crypto/bcrypt"

// Hash turns a raw password into a storable bcrypt hash. DefaultCost
// (~100ms) is the point: slow for attackers, unnoticeable for logins.
func Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

// Verify checks a login attempt against the stored hash. Constant-time
// comparison inside — never use == on passwords or hashes.
func Verify(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
