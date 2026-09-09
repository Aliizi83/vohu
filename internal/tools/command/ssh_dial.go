package command

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// TestDial verifies an SSH private key can actually authenticate against a
// host — dialing and completing the auth handshake, then closing
// immediately without running any command. Used by sshconn.Service.Create
// to confirm a connection actually works before it's ever written to the
// database, rather than discovering a typo'd host or a mismatched key only
// the first time the agent tries to use it.
func TestDial(ctx context.Context, host string, port int, username string, privateKeyPEM string) error {
	signer, err := ssh.ParsePrivateKey([]byte(privateKeyPEM))
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		// Same gap SSHExecutor has — no known_hosts store yet for
		// user-managed connections; tracked there, not silently
		// duplicated as a fresh TODO here.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", host, port)

	dialer := net.Dialer{Timeout: config.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		conn.Close()
		return fmt.Errorf("handshake: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	client.Close()

	return nil
}
