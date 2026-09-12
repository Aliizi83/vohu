package chat

import (
	"context"
	"fmt"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/commandrule"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tools"
	"github.com/Aliizi83/vohu/internal/tools/command"
	"golang.org/x/crypto/ssh"
)

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
//
// The command policy itself is per-connection, not a single policy
// shared by every connection — commandRules.ListForConnection is read
// fresh on every call (see buildCommandPolicy) rather than built once at
// registry-construction time, since which connectionId a given call
// names isn't known until the model actually asks for one.
type SSHTool struct {
	userID       uint
	sshconns     sshconn.Service
	canAccess    shared.AccessLevelCheck
	commandRules commandrule.Service
}

func NewSSHTool(
	userID uint,
	sshconns sshconn.Service,
	canAccess shared.AccessLevelCheck,
	commandRules commandrule.Service,
) *SSHTool {
	return &SSHTool{
		userID:       userID,
		sshconns:     sshconns,
		canAccess:    canAccess,
		commandRules: commandRules,
	}
}

// buildCommandPolicy converts one connection's stored rules into the
// command package's own Policy shape — commandrule never imports
// internal/tools/command itself (same decoupling rule every platform
// module besides chat follows), so that conversion has to happen here.
// Always accept-mode (allow-list): a connection with no rules permits
// nothing, matching commandrule.Rule's own doc comment.
func buildCommandPolicy(rules []commandrule.Rule) command.Policy {
	converted := make([]command.Rule, 0, len(rules))
	for _, r := range rules {
		converted = append(converted, command.Rule{
			Program:      r.Program,
			ArgsPrefixes: [][]string(r.ArgsPrefixes),
			Allowed:      r.Allowed,
		})
	}
	return command.NewCommandPolicy(command.PolicyModeAccept, converted)
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

	privateKey, err := t.sshconns.DecryptPrivateKey(conn)
	if err != nil {
		return tools.ToolResult{Success: false, Data: "failed to decrypt connection private key"}, nil
	}

	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return tools.ToolResult{Success: false, Data: "failed to parse private key"}, nil
	}

	rules, _, err := t.commandRules.ListForConnection(ctx, connectionID, shared.Pagination{PageNumber: 1, PageSize: 1000})
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("failed to load command policy: %v", err)}, nil
	}
	policy := buildCommandPolicy(rules)

	executor := command.NewSSHExecutor(conn.Host, conn.Port, conn.Username, ssh.PublicKeys(signer), policy)

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
