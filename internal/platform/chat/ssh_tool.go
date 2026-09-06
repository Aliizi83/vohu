package chat

import (
	"context"
	"fmt"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tools"
	"github.com/Aliizi83/vohu/internal/tools/command"
	"golang.org/x/crypto/ssh"
)

// HasAccessLevel is rbac.Service.HasAccessLevel's shape, injected as a
// function value like every cross-module dependency in this codebase —
// this package never imports rbac just for one permission check. level is
// a plain string (rbac.AccessLevel's underlying type) for the same reason
// sshconn's GrantCreatorAccess takes one instead of an rbac type.
type HasAccessLevel func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error)

// accessLevelWrite is what executing a command over a connection
// requires — running anything, even something read-only in intent,
// changes state on the remote system (a process runs, output is
// produced), so it's gated at Write, not the bare Read that
// ListSSHConnectionsTool's discovery-only listing requires.
const accessLevelWrite = "write"

// SSHTool is the platform's own tool — distinct from command.Tool, which
// runs locally on whatever process it's in. It never constructs a
// command.SSHExecutor until HasAccessLevel has cleared the caller at
// Write level for the specific connection they asked for; the
// connection's own AuthMethod/secret never leaves this method (never
// returned to the model, never logged).
type SSHTool struct {
	userID        uint
	sshconns      sshconn.Service
	canAccess     HasAccessLevel
	commandPolicy command.Policy
}

func NewSSHTool(
	userID uint,
	sshconns sshconn.Service,
	canAccess HasAccessLevel,
	commandPolicy command.Policy,
) *SSHTool {
	return &SSHTool{
		userID:        userID,
		sshconns:      sshconns,
		canAccess:     canAccess,
		commandPolicy: commandPolicy,
	}
}

func (t *SSHTool) Name() string { return "ssh_execute" }

func (t *SSHTool) Description() string {
	return "Execute a command on a remote host over SSH, using an SSH connection already stored in the system."
}

func (t *SSHTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{
		Properties: map[string]ai_model.ToolProperty{
			"connectionId": {
				Type:        "integer",
				Description: "The ID of the SSH connection to use.",
			},
			"program": {
				Type:        "string",
				Description: "The program or executable to run, e.g. \"git\" or \"ls\".",
			},
			"args": {
				Type:        "array",
				Description: "Arguments to pass to the program, e.g. [\"status\"] for \"git status\".",
				Items:       &ai_model.ToolProperty{Type: "string"},
			},
		},
		Required: []string{"connectionId", "program"},
	}
}

func (t *SSHTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	connectionID, ok := parseUintArg(args["connectionId"])
	if !ok {
		return tools.ToolResult{Success: false, Data: "connectionId is required and must be a number"}, nil
	}

	program, ok := args["program"].(string)
	if !ok || program == "" {
		return tools.ToolResult{Success: false, Data: "program is required"}, nil
	}

	commandArgs, err := parseStringArrayArg(args["args"])
	if err != nil {
		return tools.ToolResult{Success: false, Data: err.Error()}, nil
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

	secret, err := t.sshconns.DecryptSecret(conn)
	if err != nil {
		return tools.ToolResult{Success: false, Data: "failed to decrypt connection secret"}, nil
	}

	var auth ssh.AuthMethod
	switch conn.AuthMethod {
	case sshconn.AuthPassword:
		auth = ssh.Password(secret)
	case sshconn.AuthPrivateKey:
		signer, err := ssh.ParsePrivateKey([]byte(secret))
		if err != nil {
			return tools.ToolResult{Success: false, Data: "failed to parse private key"}, nil
		}
		auth = ssh.PublicKeys(signer)
	default:
		return tools.ToolResult{Success: false, Data: "unknown auth method on this connection"}, nil
	}

	executor := command.NewSSHExecutor(conn.Host, conn.Port, conn.Username, auth, t.commandPolicy)

	output, err := executor.Execute(ctx, command.Command{Program: program, Args: commandArgs})
	if err != nil {
		return tools.ToolResult{
			Success: false,
			Data: map[string]any{
				"output": output,
				"error":  err.Error(),
			},
		}, nil
	}

	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"output": output,
		},
	}, nil
}

// parseUintArg handles the float64 shape encoding/json produces for
// numbers decoded into map[string]any — which is exactly how a tool
// call's arguments arrive from every provider in this codebase.
func parseUintArg(v any) (uint, bool) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint(n), true
	default:
		return 0, false
	}
}

func parseStringArrayArg(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}

	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("args must be an array")
	}

	result := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("all command arguments must be strings")
		}
		result = append(result, s)
	}

	return result, nil
}
