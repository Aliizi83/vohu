// Package toolbuild compiles a customtool.ToolVersion's Go source into a
// binary for a specific target OS/arch.
package toolbuild

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Request struct {
	SourceCode string
	GOOS       string
	GOARCH     string
}

type Builder interface {
	Build(ctx context.Context, req Request) ([]byte, error)
}

type GoBuilder struct {
	Timeout time.Duration
}

func NewGoBuilder() *GoBuilder {
	return &GoBuilder{Timeout: 2 * time.Minute}
}

func (b *GoBuilder) Build(ctx context.Context, req Request) ([]byte, error) {
	if req.GOOS == "" || req.GOARCH == "" {
		return nil, errors.New("toolbuild: GOOS and GOARCH are required")
	}

	ctx, cancel := context.WithTimeout(ctx, b.Timeout)
	defer cancel()

	dir, err := os.MkdirTemp("", "vohu-toolbuild-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(req.SourceCode), 0o600); err != nil {
		return nil, err
	}

	if err := b.run(ctx, dir, nil, "go", "mod", "init", "customtool"); err != nil {
		return nil, err
	}
	if err := b.run(ctx, dir, nil, "go", "mod", "tidy"); err != nil {
		return nil, fmt.Errorf("resolving dependencies: %w", err)
	}

	outputPath := filepath.Join(dir, "tool.bin")
	env := []string{"CGO_ENABLED=0", "GOOS=" + req.GOOS, "GOARCH=" + req.GOARCH}
	if err := b.run(ctx, dir, env, "go", "build", "-o", outputPath, "."); err != nil {
		return nil, fmt.Errorf("building: %w", err)
	}

	return os.ReadFile(outputPath)
}

func (b *GoBuilder) run(ctx context.Context, dir string, extraEnv []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%s %v: %w: %s", name, args, err, stderr.String())
		}
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}
