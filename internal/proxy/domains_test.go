package proxy_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/6space7/porter/internal/proxy"
)

func TestGenerateSSLIPDomainUsesDashedIPv4(t *testing.T) {
	domain, err := proxy.GenerateSSLIPDomain("web", "203.0.113.42")
	if err != nil {
		t.Fatalf("generate sslip domain: %v", err)
	}

	if domain != "web.203-0-113-42.sslip.io" {
		t.Fatalf("domain = %q, want web.203-0-113-42.sslip.io", domain)
	}
}

func TestGenerateSSLIPDomainRejectsInvalidInput(t *testing.T) {
	if _, err := proxy.GenerateSSLIPDomain("Bad_App", "203.0.113.42"); err == nil {
		t.Fatal("expected invalid app label to fail")
	}
	if _, err := proxy.GenerateSSLIPDomain("web", "not-an-ip"); err == nil {
		t.Fatal("expected invalid IP to fail")
	}
}

func TestPreflightCustomDomainRequiresARecordToServerIP(t *testing.T) {
	resolver := fakeResolver{
		"app.example.com": []string{"198.51.100.10"},
	}

	err := proxy.PreflightCustomDomain(context.Background(), resolver, "app.example.com", "203.0.113.42")

	var preflightErr *proxy.DNSPreflightError
	if !errors.As(err, &preflightErr) {
		t.Fatalf("error = %v, want DNSPreflightError", err)
	}
	if preflightErr.Hostname != "app.example.com" {
		t.Fatalf("hostname = %q", preflightErr.Hostname)
	}
	if preflightErr.RequiredARecord != "203.0.113.42" {
		t.Fatalf("required A = %q", preflightErr.RequiredARecord)
	}
	if len(preflightErr.CurrentRecords) != 1 || preflightErr.CurrentRecords[0] != "198.51.100.10" {
		t.Fatalf("current records = %#v", preflightErr.CurrentRecords)
	}
}

func TestPreflightCustomDomainTreatsMissingDNSAsPreflightFailure(t *testing.T) {
	resolver := failingResolver{err: &net.DNSError{IsNotFound: true, Name: "app.example.com"}}

	err := proxy.PreflightCustomDomain(context.Background(), resolver, "app.example.com", "203.0.113.42")

	var preflightErr *proxy.DNSPreflightError
	if !errors.As(err, &preflightErr) {
		t.Fatalf("error = %v, want DNSPreflightError", err)
	}
	if preflightErr.Hostname != "app.example.com" || preflightErr.RequiredARecord != "203.0.113.42" || len(preflightErr.CurrentRecords) != 0 {
		t.Fatalf("preflight error = %#v", preflightErr)
	}
}

func TestPreflightCustomDomainAcceptsMatchingARecord(t *testing.T) {
	resolver := fakeResolver{
		"app.example.com": []string{"198.51.100.10", "203.0.113.42"},
	}

	if err := proxy.PreflightCustomDomain(context.Background(), resolver, "app.example.com", "203.0.113.42"); err != nil {
		t.Fatalf("preflight domain: %v", err)
	}
}

type fakeResolver map[string][]string

func (resolver fakeResolver) LookupHost(_ context.Context, hostname string) ([]string, error) {
	return resolver[hostname], nil
}

type failingResolver struct {
	err error
}

func (resolver failingResolver) LookupHost(context.Context, string) ([]string, error) {
	return nil, resolver.err
}
