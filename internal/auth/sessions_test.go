package auth_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/6space7/porter/internal/auth"
)

func TestNewSessionCookieUsesSecureDefaults(t *testing.T) {
	cookie := auth.NewSessionCookie("porter_session", "opaque", time.Hour, true)

	if cookie.Name != "porter_session" || cookie.Value != "opaque" {
		t.Fatalf("cookie identity = %#v", cookie)
	}
	if !cookie.HttpOnly {
		t.Fatal("session cookie must be http-only")
	}
	if !cookie.Secure {
		t.Fatal("session cookie must be secure when requested")
	}
	if cookie.Path != "/" {
		t.Fatalf("path = %q, want /", cookie.Path)
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("same site = %v, want strict", cookie.SameSite)
	}
	if cookie.MaxAge != int(time.Hour.Seconds()) {
		t.Fatalf("max age = %d", cookie.MaxAge)
	}
}

func TestExpiredSessionCookieClearsCookie(t *testing.T) {
	cookie := auth.ExpiredSessionCookie("porter_session", true)

	if cookie.Name != "porter_session" {
		t.Fatalf("name = %q", cookie.Name)
	}
	if cookie.Value != "" || cookie.MaxAge != -1 {
		t.Fatalf("expired cookie = %#v", cookie)
	}
	if !cookie.HttpOnly || !cookie.Secure {
		t.Fatalf("expired cookie lost secure flags: %#v", cookie)
	}
}
