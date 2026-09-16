package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/customtool"
)

func TestCustomTool_Execute_MissingConnectionId(t *testing.T) {
	tool := NewCustomTool(customtool.Tool{Name: "read_file"}, 1, &stubSSHConnService{}, allowAccess, &stubCustomToolService{}, nil)

	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false when connectionId is missing")
	}
}

func TestCustomTool_Execute_AccessDeniedNeverReachesConnection(t *testing.T) {
	svc := &stubSSHConnService{getErr: errors.New("GetByID should never be called")}
	tool := NewCustomTool(customtool.Tool{Name: "read_file"}, 1, svc, denyAccess, &stubCustomToolService{}, nil)

	result, err := tool.Execute(context.Background(), map[string]any{"connectionId": float64(1)})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false when access is denied")
	}
}

func TestParseParamsSchema_ExtractsPropertiesAndRequired(t *testing.T) {
	params := parseParamsSchema(`{"type":"object","properties":{"path":{"type":"string","description":"a path"}},"required":["path"]}`)
	prop, ok := params.Properties["path"]
	if !ok || prop.Type != "string" || prop.Description != "a path" {
		t.Fatalf("expected \"path\" property to round-trip, got %+v", params.Properties)
	}
	if len(params.Required) != 1 || params.Required[0] != "path" {
		t.Fatalf("expected required=[\"path\"], got %v", params.Required)
	}
}

func TestParseParamsSchema_MalformedSchemaYieldsEmptyParameters(t *testing.T) {
	params := parseParamsSchema("not json")
	if len(params.Properties) != 0 {
		t.Fatalf("expected empty properties for malformed schema, got %+v", params.Properties)
	}
}
