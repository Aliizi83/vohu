package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/customtool"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tooldeploy"
	"github.com/Aliizi83/vohu/internal/tools"
	"golang.org/x/crypto/ssh"
)

// CustomTool runs a customtool.Tool's latest version against a caller-named
// SSH connection, via tooldeploy (build-if-missing, deploy, execute — see
// its package doc comment for why this never goes through
// internal/tools/command.Policy the way ssh_execute does).
type CustomTool struct {
	row       customtool.Tool
	userID    uint
	sshconns  sshconn.Service
	canAccess shared.AccessLevelCheck
	tools     customtool.Service
	deployer  *tooldeploy.Deployer
}

func NewCustomTool(
	row customtool.Tool,
	userID uint,
	sshconns sshconn.Service,
	canAccess shared.AccessLevelCheck,
	toolsService customtool.Service,
	deployer *tooldeploy.Deployer,
) *CustomTool {
	return &CustomTool{
		row: row, userID: userID, sshconns: sshconns,
		canAccess: canAccess, tools: toolsService, deployer: deployer,
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

	conn, err := t.sshconns.GetByID(ctx, connectionID)
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("connection not found: %v", err)}, nil
	}

	privateKey, err := t.sshconns.DecryptPrivateKey(conn)
	if err != nil {
		return tools.ToolResult{Success: false, Data: "failed to decrypt connection private key"}, nil
	}
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return tools.ToolResult{Success: false, Data: "failed to parse private key"}, nil
	}

	version, err := t.tools.LatestVersionForTool(ctx, t.row.ID)
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("no version available for %s: %v", t.row.Name, err)}, nil
	}

	payload := make(map[string]any, len(args))
	for k, v := range args {
		if k != "connectionId" {
			payload[k] = v
		}
	}
	stdin, err := json.Marshal(payload)
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("failed to encode arguments: %v", err)}, nil
	}

	output, err := t.deployer.Run(ctx,
		tooldeploy.Connection{Host: conn.Host, Port: conn.Port, Username: conn.Username, Auth: ssh.PublicKeys(signer)},
		tooldeploy.Tool{Name: t.row.Name, Description: t.row.Description, Version: version.Version, SourceCode: version.SourceCode},
		stdin,
	)
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("failed to run %s: %v (output: %s)", t.row.Name, err, output)}, nil
	}

	var result struct {
		Success bool `json:"success"`
		Data    any  `json:"data"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("invalid output from %s: %s", t.row.Name, output)}, nil
	}
	return tools.ToolResult{Success: result.Success, Data: result.Data}, nil
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
