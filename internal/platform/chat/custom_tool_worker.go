package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Aliizi83/vohu/internal/jobqueue"
	"github.com/Aliizi83/vohu/internal/platform/customtool"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tooldeploy"
	"golang.org/x/crypto/ssh"
)

const CustomToolQueue = "custom_tools"

const customToolJobType = "custom_tool.execute"

type customToolJobPayload struct {
	ToolID       uint           `json:"toolId"`
	ConnectionID uint           `json:"connectionId"`
	Args         map[string]any `json:"args"`
}

// RegisterCustomToolWorker wires the given worker to actually build (if
// needed), deploy, and run a custom tool's latest version — the work
// CustomTool.Execute used to do inline before handing it off to the job
// queue. Looks the tool's version and the connection's credentials up
// fresh on every job (never trusts anything beyond the IDs in the
// payload), so "latest version always wins" and a rotated SSH key is
// picked up immediately.
func RegisterCustomToolWorker(worker *jobqueue.Worker, sshconns sshconn.Service, customTools customtool.Service, deployer *tooldeploy.Deployer) {
	worker.Register(customToolJobType, func(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var payload customToolJobPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, fmt.Errorf("invalid custom tool job payload: %w", err)
		}

		row, err := customTools.GetToolByID(ctx, payload.ToolID)
		if err != nil {
			return nil, fmt.Errorf("tool not found: %w", err)
		}
		version, err := customTools.LatestVersionForTool(ctx, payload.ToolID)
		if err != nil {
			return nil, fmt.Errorf("no version available for %s: %w", row.Name, err)
		}

		conn, err := sshconns.GetByID(ctx, payload.ConnectionID)
		if err != nil {
			return nil, fmt.Errorf("connection not found: %w", err)
		}
		privateKey, err := sshconns.DecryptPrivateKey(conn)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt connection private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey([]byte(privateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}

		stdin, err := json.Marshal(payload.Args)
		if err != nil {
			return nil, fmt.Errorf("failed to encode arguments: %w", err)
		}

		output, err := deployer.Run(ctx,
			tooldeploy.Connection{Host: conn.Host, Port: conn.Port, Username: conn.Username, Auth: ssh.PublicKeys(signer)},
			tooldeploy.Tool{Name: row.Name, Description: row.Description, Version: version.Version, SourceCode: version.SourceCode},
			stdin,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to run %s: %w (output: %s)", row.Name, err, output)
		}
		return output, nil
	})
}
