package command

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHExecutor runs commands on a remote host over SSH instead of the
// process command.Tool's Executor lives in — the platform's chat module
// uses this exclusively so nothing on the server host ever shells out on
// a user's behalf; the same command.Policy every Executor evaluates still
// applies here, unchanged.
type SSHExecutor struct {
	host     string
	port     int
	username string
	auth     ssh.AuthMethod
	policy   Policy
}

// NewSSHExecutor takes an already-built ssh.AuthMethod (ssh.Password(...)
// or ssh.PublicKeys(...)) rather than a raw secret — resolving a
// connection's AuthMethod/decrypted secret into one is the caller's job
// (the platform's sshconn/chat modules), keeping this package free of any
// import on internal/platform.
func NewSSHExecutor(host string, port int, username string, auth ssh.AuthMethod, policy Policy) *SSHExecutor {
	return &SSHExecutor{
		host:     host,
		port:     port,
		username: username,
		auth:     auth,
		policy:   policy,
	}
}

func (e *SSHExecutor) Execute(ctx context.Context, command Command) (string, error) {
	decision := e.policy.Evaluate(command)
	if !decision.Allowed {
		return decision.Reason, ErrCommandNotAllowed
	}

	config := &ssh.ClientConfig{
		User: e.username,
		Auth: []ssh.AuthMethod{e.auth},
		// No known_hosts store exists yet for user-managed connections —
		// pinning the host key on first connect (like most SSH clients
		// do) is real future work, not something to fake here. This is a
		// tracked gap, not a silent one.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", e.host, e.port)

	dialer := net.Dialer{Timeout: config.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return "", fmt.Errorf("ssh dial: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		conn.Close()
		return "", fmt.Errorf("ssh handshake: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	var output bytes.Buffer
	session.Stdout = &output
	session.Stderr = &output

	if err := session.Run(shellQuoteCommand(command)); err != nil {
		return output.String(), err
	}

	return output.String(), nil
}

// shellQuoteCommand joins Program and Args into a single string safe to
// hand to the remote shell — session.Run executes via the user's login
// shell, so each argument is single-quoted (with embedded single quotes
// escaped) rather than passed as an argv array like exec.CommandContext
// gets locally.
func shellQuoteCommand(command Command) string {
	parts := make([]string, 0, len(command.Args)+1)
	parts = append(parts, shellQuoteArg(command.Program))
	for _, arg := range command.Args {
		parts = append(parts, shellQuoteArg(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuoteArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
