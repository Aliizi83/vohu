package sshconn

type CreateSSHConnectionRequest struct {
	Name       string `json:"name" binding:"required,max=100"`
	Host       string `json:"host" binding:"required,max=255"`
	Port       int    `json:"port" binding:"omitempty,min=1,max=65535"`
	Username   string `json:"username" binding:"required,max=100"`
	PrivateKey string `json:"privateKey" binding:"required"`
}

type UpdateSSHConnectionRequest struct {
	Name     string `json:"name" binding:"omitempty,max=100"`
	Host     string `json:"host" binding:"omitempty,max=255"`
	Port     int    `json:"port" binding:"omitempty,min=1,max=65535"`
	Username string `json:"username" binding:"omitempty,max=100"`
	// PrivateKey is optional on update — omitted means "keep the existing
	// key," since there is no way to show it back to the client to
	// prefill a form.
	PrivateKey string `json:"privateKey"`
	// CommandPolicyMode toggles how this connection's command rules are
	// interpreted (see SSHConnection.CommandPolicyMode) — omitted means
	// "leave it as-is".
	CommandPolicyMode string `json:"commandPolicyMode" binding:"omitempty,oneof=accept prohibited"`
}

// Response deliberately never includes the private key, encrypted or not —
// the API is write-only for it, same as user.Response never includes the
// password hash.
type Response struct {
	ID                uint   `json:"id"`
	Name              string `json:"name"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Username          string `json:"username"`
	CommandPolicyMode string `json:"commandPolicyMode"`
	CreatedByUserID   uint   `json:"createdByUserId"`
}

func toResponse(conn SSHConnection) Response {
	return Response{
		ID:                conn.ID,
		Name:              conn.Name,
		Host:              conn.Host,
		Port:              conn.Port,
		Username:          conn.Username,
		CommandPolicyMode: conn.CommandPolicyMode,
		CreatedByUserID:   conn.CreatedByUserID,
	}
}
