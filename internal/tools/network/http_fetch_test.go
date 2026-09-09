package network

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPFetchTool_ExtractsTextFromHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><head><title>ignored via style/script below</title>
			<style>body { color: red; }</style>
			<script>alert("nope")</script>
			</head><body><h1>Hello</h1><p>World</p></body></html>`))
	}))
	defer server.Close()

	tool := NewHTTPFetchTool()
	result, err := tool.Execute(context.Background(), map[string]any{"url": server.URL})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %+v", result.Data)
	}

	data := result.Data.(map[string]any)
	content := data["content"].(string)

	if !strings.Contains(content, "Hello") || !strings.Contains(content, "World") {
		t.Fatalf("expected extracted text to include the body content, got %q", content)
	}
	if strings.Contains(content, "color: red") || strings.Contains(content, "alert") {
		t.Fatalf("expected <style>/<script> content to be dropped, got %q", content)
	}
	if strings.Contains(content, "<h1>") || strings.Contains(content, "<p>") {
		t.Fatalf("expected tags to be stripped, got %q", content)
	}
}

func TestHTTPFetchTool_PlainTextPassesThroughUnmodified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("just some plain text"))
	}))
	defer server.Close()

	tool := NewHTTPFetchTool()
	result, err := tool.Execute(context.Background(), map[string]any{"url": server.URL})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["content"].(string) != "just some plain text" {
		t.Fatalf("expected plain text to pass through unmodified, got %q", data["content"])
	}
}

func TestHTTPFetchTool_ReportsStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	tool := NewHTTPFetchTool()
	result, err := tool.Execute(context.Background(), map[string]any{"url": server.URL})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	// A 404 is still a completed HTTP exchange, not a tool failure — the
	// model sees the status code and decides what it means, same
	// reasoning as ShellTool reporting a non-zero exit code as data.
	if !result.Success {
		t.Fatalf("expected Success=true for a completed request even with a 404 status, got: %+v", result.Data)
	}
	data := result.Data.(map[string]any)
	if data["statusCode"].(int) != http.StatusNotFound {
		t.Fatalf("expected statusCode=404, got %v", data["statusCode"])
	}
}

func TestHTTPFetchTool_UsesRequestedMethod(t *testing.T) {
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
	}))
	defer server.Close()

	tool := NewHTTPFetchTool()
	if _, err := tool.Execute(context.Background(), map[string]any{
		"url": server.URL, "method": "head",
	}); err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}

	if gotMethod != http.MethodHead {
		t.Fatalf("expected method HEAD (case-insensitive input), got %q", gotMethod)
	}
}

func TestHTTPFetchTool_MissingURL(t *testing.T) {
	tool := NewHTTPFetchTool()
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when url is missing")
	}
}

func TestHTTPFetchTool_ConnectionFailureIsReportedNotReturned(t *testing.T) {
	tool := NewHTTPFetchTool()
	result, err := tool.Execute(context.Background(), map[string]any{
		"url": "http://127.0.0.1:1", // nothing listens on port 1
	})
	if err != nil {
		t.Fatalf("expected nil Go error for a connection failure, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false for an unreachable URL")
	}
}

func TestHTTPFetchTool_TruncatesLargeBodies(t *testing.T) {
	huge := strings.Repeat("x", maxFetchChars*2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(huge))
	}))
	defer server.Close()

	tool := NewHTTPFetchTool()
	result, err := tool.Execute(context.Background(), map[string]any{"url": server.URL})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	data := result.Data.(map[string]any)
	content := data["content"].(string)
	if len(content) >= len(huge) {
		t.Fatalf("expected the body to be truncated, got length %d (original %d)", len(content), len(huge))
	}
}
