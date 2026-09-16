package toolbuild_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Aliizi83/vohu/internal/toolbuild"
)

const validSource = `package main

import "fmt"

func main() {
	fmt.Print("hello from a built tool")
}
`

const invalidSource = `package main

func main() {
	this is not valid go
}
`

func TestBuild_CompilesValidSource_ReturnsRunnableBinary(t *testing.T) {
	builder := toolbuild.NewGoBuilder()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	binary, err := builder.Build(ctx, toolbuild.Request{
		SourceCode: validSource, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if len(binary) == 0 {
		t.Fatal("expected a non-empty binary")
	}

	path := filepath.Join(t.TempDir(), "built-tool")
	if err := os.WriteFile(path, binary, 0o700); err != nil {
		t.Fatalf("failed to write binary: %v", err)
	}

	out, err := exec.Command(path).CombinedOutput()
	if err != nil {
		t.Fatalf("running the built binary failed: %v (output: %s)", err, out)
	}
	if string(out) != "hello from a built tool" {
		t.Fatalf("expected %q, got %q", "hello from a built tool", out)
	}
}

func TestBuild_InvalidSource_ReturnsError(t *testing.T) {
	builder := toolbuild.NewGoBuilder()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err := builder.Build(ctx, toolbuild.Request{
		SourceCode: invalidSource, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	})
	if err == nil {
		t.Fatal("expected an error for invalid source, got nil")
	}
}

func TestBuild_CrossCompilesForADifferentTarget(t *testing.T) {
	builder := toolbuild.NewGoBuilder()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	binary, err := builder.Build(ctx, toolbuild.Request{
		SourceCode: validSource, GOOS: "linux", GOARCH: "arm64",
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if len(binary) == 0 {
		t.Fatal("expected a non-empty binary")
	}
	// ELF magic bytes — confirms a real binary came out, not just any file.
	if len(binary) < 4 || string(binary[:4]) != "\x7fELF" {
		t.Fatalf("expected an ELF binary, got header bytes %v", binary[:min(4, len(binary))])
	}
}

func TestBuild_MissingTarget_ReturnsError(t *testing.T) {
	builder := toolbuild.NewGoBuilder()
	_, err := builder.Build(context.Background(), toolbuild.Request{SourceCode: validSource})
	if err == nil {
		t.Fatal("expected an error when GOOS/GOARCH are missing, got nil")
	}
}
