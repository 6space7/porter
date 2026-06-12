package remote_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/6space7/porter/internal/remote"
	"golang.org/x/crypto/ssh"
)

func TestSSHValidatorRequiresConnectionFields(t *testing.T) {
	validator := remote.SSHValidator{}
	if _, err := validator.Check(context.Background(), remote.CheckRequest{}); err == nil {
		t.Fatal("expected missing fields to fail")
	}
}

func TestSSHValidatorReturnsOSAndDockerVersion(t *testing.T) {
	client := &fakeSSHClient{responses: map[string]fakeSSHResponse{
		"uname -a":         {output: "Linux porter-test\n"},
		"docker --version": {output: "Docker version 27.0.0\n"},
	}}
	validator := remote.SSHValidator{
		Dial: func(_ context.Context, addr string, config *ssh.ClientConfig) (remote.SSHClient, error) {
			if addr != "example.com:22" {
				t.Fatalf("addr = %q, want example.com:22", addr)
			}
			if config.User != "root" {
				t.Fatalf("user = %q, want root", config.User)
			}
			return client, nil
		},
	}

	result, err := validator.Check(context.Background(), remote.CheckRequest{
		Host:          "example.com",
		User:          "root",
		PrivateKeyPEM: testPrivateKeyPEM(t),
	})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if result.OS != "Linux porter-test" || result.DockerVersion != "Docker version 27.0.0" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSSHValidatorReportsDockerMissing(t *testing.T) {
	client := &fakeSSHClient{responses: map[string]fakeSSHResponse{
		"uname -a":         {output: "Linux porter-test\n"},
		"docker --version": {err: errors.New("not found")},
	}}
	validator := remote.SSHValidator{
		Dial: func(context.Context, string, *ssh.ClientConfig) (remote.SSHClient, error) {
			return client, nil
		},
	}

	_, err := validator.Check(context.Background(), remote.CheckRequest{
		Host:          "example.com:2222",
		User:          "root",
		PrivateKeyPEM: testPrivateKeyPEM(t),
	})
	if !errors.Is(err, remote.ErrDockerMissing) {
		t.Fatalf("error = %v, want docker missing", err)
	}
}

func testPrivateKeyPEM(t *testing.T) []byte {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

type fakeSSHClient struct {
	responses map[string]fakeSSHResponse
}

func (client *fakeSSHClient) NewSession() (remote.SSHSession, error) {
	return &fakeSSHSession{responses: client.responses}, nil
}

func (client *fakeSSHClient) Close() error {
	return nil
}

type fakeSSHResponse struct {
	output string
	err    error
}

type fakeSSHSession struct {
	responses map[string]fakeSSHResponse
}

func (session *fakeSSHSession) CombinedOutput(cmd string) ([]byte, error) {
	response := session.responses[cmd]
	return []byte(response.output), response.err
}

func (session *fakeSSHSession) Close() error {
	return nil
}
