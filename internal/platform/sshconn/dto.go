package sshconn

type CreateSSHConnectionRequest struct {
	Name       string     `json:"name" binding:"required,max=100"`
	Host       string     `json:"host" binding:"required,max=255"`
	Port       int        `json:"port" binding:"omitempty,min=1,max=65535"`
	Username   string     `json:"username" binding:"required,max=100"`
	AuthMethod AuthMethod `json:"authMethod" binding:"required,oneof=password private_key"`
	Secret     string     `json:"secret" binding:"required"`
}

type UpdateSSHConnectionRequest struct {
	Name       string     `json:"name" binding:"omitempty,max=100"`
	Host       string     `json:"host" binding:"omitempty,max=255"`
	Port       int        `json:"port" binding:"omitempty,min=1,max=65535"`
	Username   string     `json:"username" binding:"omitempty,max=100"`
	AuthMethod AuthMethod `json:"authMethod" binding:"omitempty,oneof=password private_key"`
	// Secret is optional on update — omitted means "keep the existing
	// secret," since there is no way to show it back to the client to
	// prefill a form.
	Secret string `json:"secret"`
}

// Response deliberately never includes the secret, encrypted or not — the
// API is write-only for it, same as user.Response never includes the
// password hash.
type Response struct {
	ID              uint       `json:"id"`
	Name            string     `json:"name"`
	Host            string     `json:"host"`
	Port            int        `json:"port"`
	Username        string     `json:"username"`
	AuthMethod      AuthMethod `json:"authMethod"`
	CreatedByUserID uint       `json:"createdByUserId"`
}

func toResponse(conn SSHConnection) Response {
	return Response{
		ID:              conn.ID,
		Name:            conn.Name,
		Host:            conn.Host,
		Port:            conn.Port,
		Username:        conn.Username,
		AuthMethod:      conn.AuthMethod,
		CreatedByUserID: conn.CreatedByUserID,
	}
}
