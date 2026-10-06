package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
)

// Sign mints an HS256 token carrying identity + role, valid for ttl.
func Sign(userID, email string, role userspublic.Role, secret string, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	expiresAt = time.Now().Add(ttl)
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"role":  string(role),
		"exp":   expiresAt.Unix(),
	})
	token, err = t.SignedString([]byte(secret))
	return token, expiresAt, err
}

// Parse verifies signature + expiry and returns the claims. The role is
// validated against the users contract so the modules can't drift.
func Parse(tokenStr, secret string) (public.Claims, error) {
	t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		// SECURITY: accept only HS256 (alg-confusion attack otherwise).
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return public.Claims{}, errors.New("invalid token")
	}
	c, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return public.Claims{}, errors.New("invalid claims")
	}
	role, _ := c["role"].(string)
	if role != string(userspublic.RoleAdmin) && role != string(userspublic.RoleCustomer) {
		return public.Claims{}, errors.New("invalid role claim")
	}
	exp, _ := c["exp"].(float64)
	sub, _ := c["sub"].(string)
	email, _ := c["email"].(string)
	if sub == "" || email == "" {
		return public.Claims{}, errors.New("incomplete claims")
	}
	return public.Claims{
		UserID: sub,
		Email:  email,
		Role:   userspublic.Role(role),
		Expiry: time.Unix(int64(exp), 0),
	}, nil
}
