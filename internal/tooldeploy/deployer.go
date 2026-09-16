// Package tooldeploy gets a customtool.ToolVersion's compiled binary onto
// a target SSH host and runs it — building it first if the host's own
// ~/.vohu/tools.json manifest doesn't already list this exact version.
//
// This never goes through internal/tools/command.Policy — that gates
// agent-directed ssh_execute calls; a custom tool's access is decided
// separately, by rbac.ResourceAccess on the tool itself. Concurrent Run
// calls deploying the same tool+version to the same connection can race
// (both build and upload); harmless since both write identical content
// to the same deterministic path, just wasted work.
package tooldeploy

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"path"
	"time"

	"github.com/Aliizi83/vohu/internal/toolbuild"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Connection struct {
	Host     string
	Port     int
	Username string
	Auth     ssh.AuthMethod
}

type Tool struct {
	Name        string
	Description string
	Version     string
	SourceCode  string
}

type Deployer struct {
	builder toolbuild.Builder
}

func NewDeployer(builder toolbuild.Builder) *Deployer {
	return &Deployer{builder: builder}
}

// Run ensures tool is deployed at its current version on conn, then
// executes it with stdin piped in, returning its stdout.
func (d *Deployer) Run(ctx context.Context, conn Connection, tool Tool, stdin []byte) ([]byte, error) {
	client, err := dial(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer client.Close()

	remotePath, err := d.ensureDeployed(ctx, client, tool)
	if err != nil {
		return nil, fmt.Errorf("ensure deployed: %w", err)
	}

	return execute(remotePath, stdin, client)
}

func (d *Deployer) ensureDeployed(ctx context.Context, client *ssh.Client, tool Tool) (string, error) {
	goos, goarch, err := detectPlatform(client)
	if err != nil {
		return "", err
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return "", fmt.Errorf("sftp: %w", err)
	}
	defer sftpClient.Close()

	manifest, err := readManifest(sftpClient)
	if err != nil {
		return "", err
	}

	if entry, ok := manifest.find(tool.Name); ok && entry.Version == tool.Version {
		return entry.ExecutablePath, nil
	}

	binary, err := d.builder.Build(ctx, toolbuild.Request{SourceCode: tool.SourceCode, GOOS: goos, GOARCH: goarch})
	if err != nil {
		return "", fmt.Errorf("build: %w", err)
	}

	remotePath := path.Join(".vohu/tools", tool.Name, tool.Version, tool.Name)
	if err := uploadBinary(sftpClient, remotePath, binary); err != nil {
		return "", fmt.Errorf("upload: %w", err)
	}

	manifest.upsert(ManifestEntry{
		Name: tool.Name, Description: tool.Description,
		ExecutablePath: remotePath, Version: tool.Version,
	})
	if err := writeManifest(sftpClient, manifest); err != nil {
		return "", fmt.Errorf("write manifest: %w", err)
	}

	return remotePath, nil
}

func uploadBinary(client *sftp.Client, remotePath string, data []byte) error {
	if err := client.MkdirAll(path.Dir(remotePath)); err != nil {
		return err
	}

	f, err := client.Create(remotePath)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return client.Chmod(remotePath, 0o700)
}

func execute(remotePath string, stdin []byte, client *ssh.Client) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	session.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run("./" + remotePath); err != nil {
		return stdout.Bytes(), fmt.Errorf("run %s: %w (stderr: %s)", remotePath, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func dial(ctx context.Context, conn Connection) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            conn.Username,
		Auth:            []ssh.AuthMethod{conn.Auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", conn.Host, conn.Port)
	dialer := net.Dialer{Timeout: config.Timeout}

	tcpConn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(tcpConn, address, config)
	if err != nil {
		tcpConn.Close()
		return nil, err
	}
	return ssh.NewClient(sshConn, chans, reqs), nil
}
