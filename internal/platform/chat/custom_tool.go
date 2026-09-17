package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/jobqueue"
	"github.com/Aliizi83/vohu/internal/platform/customtool"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tools"
)

// CustomTool checks the caller's access to a named SSH connection, then
// hands the actual build/deploy/execute work to a jobqueue worker (see
// custom_tool_worker.go) and blocks on the result — that work can take
// seconds (a fresh build) and shouldn't run inline in the HTTP request's
// goroutine. The access check itself stays here, in the request path,
// since it's specific to the calling user; the worker trusts whatever
// connectionId/toolId it's handed.
type CustomTool struct {
	row            customtool.Tool
	userID         uint
	sshconns       sshconn.Service
	canAccess      shared.AccessLevelCheck
	jobs           jobqueue.Store
	defaultRetries int
}

func NewCustomTool(
	row customtool.Tool,
	userID uint,
	sshconns sshconn.Service,
	canAccess shared.AccessLevelCheck,
	jobs jobqueue.Store,
	defaultRetries int,
) *CustomTool {
	return &CustomTool{
		row: row, userID: userID, sshconns: sshconns,
		canAccess: canAccess, jobs: jobs, defaultRetries: defaultRetries,
	}
}

func (t *CustomTool) Name() string { return t.row.Name }

func (t *CustomTool) Description() string { return t.row.Description }

func (t *CustomTool) Parameters() ai_model.ToolParameters {
	params := parseParamsSchema(t.row.ParamsSchema)
	params.Properties["connectionId"] = ai_model.ToolProperty{
		Type: "integer", Description: "The ID of the SSH connection to run this against.",
	}
	params.Required = append(params.Required, "connectionId")
	return params
}

func (t *CustomTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	connectionID, ok := parseUintArg(args["connectionId"])
	if !ok {
		return tools.ToolResult{Success: false, Data: "connectionId is required and must be a number"}, nil
	}

	allowed, err := t.canAccess(ctx, t.userID, sshconn.ResourceTypeSSHConnection, connectionID, accessLevelWrite)
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("permission check failed: %v", err)}, nil
	}
	if !allowed {
		return tools.ToolResult{Success: false, Data: "access to this SSH connection is not permitted"}, nil
	}

	toolArgs := make(map[string]any, len(args))
	for k, v := range args {
		if k != "connectionId" {
			toolArgs[k] = v
		}
	}
	payload, err := json.Marshal(customToolJobPayload{ToolID: t.row.ID, ConnectionID: connectionID, Args: toolArgs})
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("failed to encode arguments: %v", err)}, nil
	}

	job := jobqueue.Job{
		ID:            jobqueue.NewJobID(),
		Queue:         CustomToolQueue,
		Type:          customToolJobType,
		Payload:       payload,
		RetryIfFailed: t.defaultRetries,
	}
	if err := t.jobs.Enqueue(ctx, job); err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("failed to queue %s: %v", t.row.Name, err)}, nil
	}

	result, err := t.jobs.AwaitResult(ctx, job.ID)
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("failed to run %s: %v", t.row.Name, err)}, nil
	}
	if result.Err != "" {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("failed to run %s: %s", t.row.Name, result.Err)}, nil
	}

	var parsed struct {
		Success bool `json:"success"`
		Data    any  `json:"data"`
	}
	if err := json.Unmarshal(result.Output, &parsed); err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("invalid output from %s: %s", t.row.Name, result.Output)}, nil
	}
	return tools.ToolResult{Success: parsed.Success, Data: parsed.Data}, nil
}

// parseParamsSchema reads the flat {"type":"object","properties":{...},
// "required":[...]} shape every seeded customtool.Tool.ParamsSchema uses —
// not a general JSON Schema parser (no nesting, no arrays). Malformed or
// unrecognized schema text yields empty parameters rather than an error,
// same "best-effort, don't fail the whole turn" reasoning as buildRegistry
// skipping an unknown tool name.
func parseParamsSchema(raw string) ai_model.ToolParameters {
	var schema struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		return ai_model.ToolParameters{Properties: map[string]ai_model.ToolProperty{}}
	}

	props := make(map[string]ai_model.ToolProperty, len(schema.Properties))
	for name, p := range schema.Properties {
		props[name] = ai_model.ToolProperty{Type: p.Type, Description: p.Description}
	}
	return ai_model.ToolParameters{Properties: props, Required: schema.Required}
}
