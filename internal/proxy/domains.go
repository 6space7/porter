package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

var sslipAppLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

type Resolver interface {
	LookupHost(ctx context.Context, hostname string) ([]string, error)
}

type DNSPreflightError struct {
	Hostname        string
	RequiredARecord string
	CurrentRecords  []string
}

func (err *DNSPreflightError) Error() string {
	return fmt.Sprintf("%s must point to %s", err.Hostname, err.RequiredARecord)
}

func GenerateSSLIPDomain(appName, publicIP string) (string, error) {
	if !sslipAppLabelPattern.MatchString(appName) {
		return "", fmt.Errorf("app name is not a valid DNS label")
	}

	ip := net.ParseIP(publicIP)
	if ip == nil || ip.To4() == nil {
		return "", fmt.Errorf("public IP must be an IPv4 address")
	}

	dashed := strings.ReplaceAll(ip.To4().String(), ".", "-")
	return appName + "." + dashed + ".sslip.io", nil
}

func PreflightCustomDomain(ctx context.Context, resolver Resolver, hostname, serverIP string) error {
	records, err := resolver.LookupHost(ctx, hostname)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return &DNSPreflightError{
				Hostname:        hostname,
				RequiredARecord: serverIP,
				CurrentRecords:  nil,
			}
		}
		return fmt.Errorf("resolve domain: %w", err)
	}

	for _, record := range records {
		if record == serverIP {
			return nil
		}
	}

	return &DNSPreflightError{
		Hostname:        hostname,
		RequiredARecord: serverIP,
		CurrentRecords:  append([]string(nil), records...),
	}
}
