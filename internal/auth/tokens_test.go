package auth_test

import (
	"strings"
	"testing"

	"github.com/6space7/porter/internal/auth"
)

func TestNewTokenStoresOnlyHashAndVerifiesPlaintextOnce(t *testing.T) {
	plaintext, record, err := auth.NewToken("reader", []string{"apps:read"})
	if err != nil {
		t.Fatalf("new token: %v", err)
	}

	if plaintext == "" {
		t.Fatal("plaintext token is empty")
	}
	if record.ID == "" {
		t.Fatal("record ID is empty")
	}
	if record.Name != "reader" {
		t.Fatalf("record name = %q, want reader", record.Name)
	}
	if record.Hash == "" {
		t.Fatal("record hash is empty")
	}
	if strings.Contains(record.Hash, plaintext) {
		t.Fatalf("stored hash %q contains plaintext token", record.Hash)
	}
	if len(record.Hash) != 64 {
		t.Fatalf("sha256 hex hash length = %d, want 64", len(record.Hash))
	}
	if !auth.VerifyToken(plaintext, record.Hash) {
		t.Fatal("expected plaintext token to verify against stored hash")
	}
	if auth.VerifyToken(plaintext+"x", record.Hash) {
		t.Fatal("expected modified token to fail verification")
	}
}

func TestHasScopeRequiresExplicitScope(t *testing.T) {
	granted := []string{"apps:read", "deployments:read"}

	if !auth.HasScope(granted, "apps:read") {
		t.Fatal("expected apps:read to be granted")
	}
	if auth.HasScope(granted, "apps:deploy") {
		t.Fatal("expected apps:deploy to be denied")
	}
}
