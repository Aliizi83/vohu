package tooldeploy

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

func detectPlatform(client *ssh.Client) (goos, goarch string, err error) {
	session, err := client.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()

	out, err := session.Output("uname -s && uname -m")
	if err != nil {
		return "", "", fmt.Errorf("uname: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return "", "", fmt.Errorf("unexpected uname output: %q", out)
	}

	return mapGOOS(lines[0]), mapGOARCH(lines[1]), nil
}

func mapGOOS(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "linux":
		return "linux"
	case "darwin":
		return "darwin"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

func mapGOARCH(s string) string {
	switch strings.TrimSpace(s) {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	case "i386", "i686":
		return "386"
	case "armv7l", "armv6l":
		return "arm"
	default:
		return strings.TrimSpace(s)
	}
}
