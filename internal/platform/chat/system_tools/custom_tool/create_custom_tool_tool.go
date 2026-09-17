package custom_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/jobqueue"
	"github.com/Aliizi83/vohu/internal/platform/customtool"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/toolbuild"
	"github.com/Aliizi83/vohu/internal/tools"
)

// initialToolVersion is what every tool created through this path starts
// at — CreateCustomToolTool never adds a second version to an existing
// tool (that would require picking a name collision policy), so there's
// no reason for this to ever be anything else.
const initialToolVersion = "1.0.0"

// CreateCustomToolTool lets the model author a brand-new customtool.Tool
// from Go source, the same way a human does through POST /custom-tools +
// POST /custom-tools/{id}/versions, but as a single tool call. Source is
// compiled first (same check POST /custom-tools/check-source runs) so a
// syntax/type error never reaches the DB — it comes back as diagnostics
// the model can read and retry with, in the same turn. On success the
// resulting tool is registered into the same registry this tool itself
// lives in, so it's callable immediately, without waiting for the next
// turn's buildRegistry to run.
type CreateCustomToolTool struct {
	userID         uint
	canAccess      shared.AccessLevelCheck
	customTools    customtool.Service
	builder        toolbuild.Builder
	sshconns       sshconn.Service
	jobs           jobqueue.Store
	defaultRetries int
	registry       *tools.Registry
}

func NewCreateCustomToolTool(
	userID uint,
	canAccess shared.AccessLevelCheck,
	customTools customtool.Service,
	builder toolbuild.Builder,
	sshconns sshconn.Service,
	jobs jobqueue.Store,
	defaultRetries int,
	registry *tools.Registry,
) *CreateCustomToolTool {
	return &CreateCustomToolTool{
		userID: userID, canAccess: canAccess, customTools: customTools, builder: builder,
		sshconns: sshconns, jobs: jobs, defaultRetries: defaultRetries, registry: registry,
	}
}

func (t *CreateCustomToolTool) Name() string { return "create_custom_tool" }

func (t *CreateCustomToolTool) Description() string {
	return "Author and register a brand-new tool from Go source code, for a job no existing tool covers. The new tool becomes callable immediately, including later in this same turn. See the \"Authoring a custom tool\" section of your instructions for the exact source-code contract before calling this."
}

func (t *CreateCustomToolTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{
		Properties: map[string]ai_model.ToolProperty{
			"name": {
				Type:        "string",
				Description: "Unique machine-readable name the model will call this tool by later, e.g. \"list_docker_containers\". Lowercase with underscores, no spaces.",
			},
			"description": {
				Type:        "string",
				Description: "One sentence describing what the tool does and when to call it — this is exactly what a future turn's tool list will show.",
			},
			"paramsSchema": {
				Type:        "string",
				Description: `A JSON string describing this tool's own input parameters, besides connectionId (added automatically): {"type":"object","properties":{"path":{"type":"string","description":"..."}},"required":["path"]}. Only "string", "integer", and "boolean" property types are supported — no nested objects or arrays.`,
			},
			"sourceCode": {
				Type:        "string",
				Description: "Complete Go source for a single package main file, following the stdin/stdout JSON contract described in your instructions.",
			},
			"visibility": {
				Type:        "string",
				Description: "\"private\" (only you can use it — default) or \"public\" (every user can use it).",
			},
		},
		Required: []string{"name", "description", "paramsSchema", "sourceCode"},
	}
}

func (t *CreateCustomToolTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	paramsSchema, _ := args["paramsSchema"].(string)
	sourceCode, _ := args["sourceCode"].(string)
	visibilityArg, _ := args["visibility"].(string)

	if name == "" || description == "" || paramsSchema == "" || sourceCode == "" {
		return tools.ToolResult{Success: false, Data: "name, description, paramsSchema, and sourceCode are all required"}, nil
	}
	if len(name) > 100 {
		return tools.ToolResult{Success: false, Data: "name must be 100 characters or fewer"}, nil
	}
	// A custom tool's name is only unique among custom tools — nothing
	// stops it from colliding with a builtin's name (ssh_execute, say),
	// and buildRegistry would refuse to register it over the real one
	// rather than let it silently bypass that tool's own access
	// controls. Reject it here too, so the model gets a clear reason
	// instead of a tool that gets created but never actually takes effect.
	if _, exists := t.registry.Get(name); exists {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("%q is already a registered tool name — choose a different name", name)}, nil
	}

	visibility := customtool.Visibility(visibilityArg)
	if visibility == "" {
		visibility = customtool.VisibilityPrivate
	}
	if visibility != customtool.VisibilityPublic && visibility != customtool.VisibilityPrivate {
		return tools.ToolResult{Success: false, Data: "visibility must be \"public\" or \"private\""}, nil
	}

	var schemaCheck struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal([]byte(paramsSchema), &schemaCheck); err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("paramsSchema is not valid JSON: %v", err)}, nil
	}

	allowed, err := t.canAccess(ctx, t.userID, customtool.ResourceTypeCustomTool, shared.WildcardResourceID, "write")
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("permission check failed: %v", err)}, nil
	}
	if !allowed {
		return tools.ToolResult{Success: false, Data: "you don't have permission to create custom tools"}, nil
	}

	if _, err := t.builder.Build(ctx, toolbuild.Request{SourceCode: sourceCode, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}); err != nil {
		return tools.ToolResult{Success: false, Data: map[string]any{
			"compileError": true,
			"diagnostics":  customtool.ParseGoErrors(err.Error()),
		}}, nil
	}

	tool, err := t.customTools.CreateTool(ctx, t.userID, customtool.CreateToolRequest{
		Name: name, Description: description, ParamsSchema: paramsSchema, Visibility: visibility,
	})
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("failed to create tool: %v", err)}, nil
	}
	if _, err := t.customTools.CreateVersion(ctx, t.userID, tool.ID, customtool.CreateVersionRequest{
		Version: initialToolVersion, SourceCode: sourceCode,
	}); err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("tool was created but its source failed to save: %v", err)}, nil
	}

	t.registry.Register(NewCustomTool(*tool, t.userID, t.sshconns, t.canAccess, t.jobs, t.defaultRetries))

	return tools.ToolResult{Success: true, Data: map[string]any{
		"toolId":  tool.ID,
		"name":    tool.Name,
		"message": fmt.Sprintf("%q created and is callable now — call it the same way as any other tool, with a connectionId.", tool.Name),
	}}, nil
}
