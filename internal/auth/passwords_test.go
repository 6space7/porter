package auth_test

import (
	"testing"

	"github.com/6space7/porter/internal/auth"
)

func TestPasswordHashVerifiesOnlyOriginalPassword(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if hash == "correct horse battery staple" {
		t.Fatal("password hash must not store plaintext")
	}
	if !auth.VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("expected original password to verify")
	}
	if auth.VerifyPassword(hash, "wrong password") {
		t.Fatal("expected wrong password to fail verification")
	}
}
