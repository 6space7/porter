package remote

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var ErrDockerMissing = errors.New("docker is missing")

type CheckRequest struct {
	Host          string
	User          string
	PrivateKeyPEM []byte
}

type CheckResult struct {
	DockerVersion string `json:"docker_version"`
	OS            string `json:"os"`
}

type Validator interface {
	Check(ctx context.Context, req CheckRequest) (CheckResult, error)
}

type SSHValidator struct {
	Timeout time.Duration
	Dial    SSHDialFunc
}

type SSHDialFunc func(ctx context.Context, addr string, config *ssh.ClientConfig) (SSHClient, error)

type SSHClient interface {
	NewSession() (SSHSession, error)
	Close() error
}

type SSHSession interface {
	CombinedOutput(cmd string) ([]byte, error)
	Close() error
}

func (validator SSHValidator) Check(ctx context.Context, req CheckRequest) (CheckResult, error) {
	host := strings.TrimSpace(req.Host)
	user := strings.TrimSpace(req.User)
	if host == "" {
		return CheckResult{}, fmt.Errorf("host is required")
	}
	if user == "" {
		return CheckResult{}, fmt.Errorf("user is required")
	}
	if len(req.PrivateKeyPEM) == 0 {
		return CheckResult{}, fmt.Errorf("private key is required")
	}

	signer, err := ssh.ParsePrivateKey(req.PrivateKeyPEM)
	if err != nil {
		return CheckResult{}, fmt.Errorf("parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         validator.timeout(),
	}
	dial := validator.Dial
	if dial == nil {
		dial = dialSSH
	}
	client, err := dial(ctx, sshAddress(host), config)
	if err != nil {
		return CheckResult{}, fmt.Errorf("dial ssh: %w", err)
	}
	defer client.Close()

	osInfo, err := runSSHCommand(client, "uname -a")
	if err != nil {
		return CheckResult{}, fmt.Errorf("check os: %w", err)
	}
	dockerVersion, err := runSSHCommand(client, "docker --version")
	if err != nil || strings.TrimSpace(dockerVersion) == "" {
		return CheckResult{}, ErrDockerMissing
	}
	return CheckResult{
		DockerVersion: strings.TrimSpace(dockerVersion),
		OS:            strings.TrimSpace(osInfo),
	}, nil
}

func (validator SSHValidator) timeout() time.Duration {
	if validator.Timeout > 0 {
		return validator.Timeout
	}
	return 10 * time.Second
}

func dialSSH(ctx context.Context, addr string, config *ssh.ClientConfig) (SSHClient, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return realSSHClient{Client: ssh.NewClient(sshConn, chans, reqs)}, nil
}

func runSSHCommand(client SSHClient, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	output, err := session.CombinedOutput(command)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func sshAddress(host string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(host, "22")
}

type realSSHClient struct {
	*ssh.Client
}

func (client realSSHClient) NewSession() (SSHSession, error) {
	return client.Client.NewSession()
}
