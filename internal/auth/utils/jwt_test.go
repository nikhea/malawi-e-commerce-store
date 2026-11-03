package utils_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/utils"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
)

func TestSignAndParse(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	token, expiresAt, err := utils.Sign("u1", "a@b.mw", userspublic.RoleCustomer, secret, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := utils.Parse(token, secret)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != "u1" || claims.Email != "a@b.mw" || claims.Role != userspublic.RoleCustomer {
		t.Fatalf("bad claims: %+v", claims)
	}
	if !claims.Expiry.Equal(expiresAt.Truncate(time.Second)) {
		t.Fatalf("expiry %v != %v", claims.Expiry, expiresAt)
	}
}

func TestParseRequiresExp(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	// jwt-go only validates exp when present, so a token without one would
	// be valid forever. The parser must reject it outright.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "u1",
		"email": "a@b.mw",
		"role":  string(userspublic.RoleCustomer),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := utils.Parse(signed, secret); err == nil {
		t.Fatal("token without exp was accepted")
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	token, _, err := utils.Sign("u1", "a@b.mw", userspublic.RoleCustomer, "0123456789abcdef0123456789abcdef", time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := utils.Parse(token, "ffffffffffffffffffffffffffffffff"); err == nil {
		t.Fatal("token verified with the wrong secret")
	}
}
