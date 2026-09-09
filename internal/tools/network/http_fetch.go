// Package network implements the agent's tools for reaching outside the
// local filesystem/shell — currently just http_fetch.
package network

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/tools"
)

const (
	fetchTimeout      = 30 * time.Second
	maxFetchBodyBytes = 2 * 1024 * 1024 // 2MB — plenty for a page, cheap to cap regardless
	maxFetchChars     = 30_000
)

type HTTPFetchTool struct {
	client *http.Client
}

func NewHTTPFetchTool() *HTTPFetchTool {
	return &HTTPFetchTool{client: &http.Client{Timeout: fetchTimeout}}
}

func (t *HTTPFetchTool) Name() string { return "http_fetch" }

func (t *HTTPFetchTool) Description() string {
	return "Fetch the content of a URL. HTML pages come back as extracted text, not raw markup, to save context."
}

func (t *HTTPFetchTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{
		Properties: map[string]ai_model.ToolProperty{
			"url": {
				Type:        "string",
				Description: "The URL to fetch, including scheme (e.g. \"https://example.com\").",
			},
			"method": {
				Type:        "string",
				Description: "HTTP method. Defaults to GET.",
			},
		},
		Required: []string{"url"},
	}
}

func (t *HTTPFetchTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	url, ok := args["url"].(string)
	if !ok || url == "" {
		return tools.ToolResult{Success: false, Data: `"url" is required`}, nil
	}

	method, _ := args["method"].(string)
	if method == "" {
		method = http.MethodGet
	}
	method = strings.ToUpper(method)

	runCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(runCtx, method, url, nil)
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("invalid request: %v", err)}, nil
	}
	// A default UA — some servers reject requests with no User-Agent at
	// all, and Go's zero-value default ("Go-http-client/1.1") reads as
	// an obvious bot to plenty of others too.
	req.Header.Set("User-Agent", "Vohu-Agent/1.0 (+https://github.com/Aliizi83/vohu)")

	resp, err := t.client.Do(req)
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("request failed: %v", err)}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBodyBytes))
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("failed to read response body: %v", err)}, nil
	}

	contentType := resp.Header.Get("Content-Type")
	content := string(body)
	if strings.Contains(contentType, "text/html") {
		content = extractText(body)
	}
	content = tools.Truncate(content, maxFetchChars)

	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"url":         url,
			"statusCode":  resp.StatusCode,
			"contentType": contentType,
			"content":     content,
		},
	}, nil
}

// extractText walks the HTML token stream and keeps only text nodes —
// <script>/<style> content is dropped entirely rather than dumped as
// meaningless text, everything else is just tag-stripped.
func extractText(body []byte) string {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	var b strings.Builder
	skipDepth := 0

	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return strings.TrimSpace(b.String())

		case html.StartTagToken:
			name, _ := tokenizer.TagName()
			if isSkippedTag(string(name)) {
				skipDepth++
			}

		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if isSkippedTag(string(name)) && skipDepth > 0 {
				skipDepth--
			}

		case html.TextToken:
			if skipDepth == 0 {
				text := strings.TrimSpace(string(tokenizer.Text()))
				if text != "" {
					b.WriteString(text)
					b.WriteString("\n")
				}
			}
		}
	}
}

func isSkippedTag(tag string) bool {
	return tag == "script" || tag == "style"
}
