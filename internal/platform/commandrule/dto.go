package commandrule

type CreateRuleRequest struct {
	SSHConnectionID uint       `json:"sshConnectionId" binding:"required"`
	Program         string     `json:"program" binding:"required,max=255"`
	ArgsPrefixes    [][]string `json:"argsPrefixes"`
	// Allowed defaults to true (an allow rule) when omitted — the common
	// case, since this is an allow-list policy; a caller has to opt in
	// explicitly to write a deny rule (e.g. carving an exception out of a
	// broader allow rule that matches first).
	Allowed *bool `json:"allowed"`
}

type UpdateRuleRequest struct {
	Program string `json:"program" binding:"omitempty,max=255"`
	// ArgsPrefixes: omitted (nil) means "don't change"; an explicit empty
	// array clears it (the rule then matches the program regardless of
	// args). encoding/json already distinguishes the two on its own — an
	// omitted key decodes to nil, "[]" decodes to a non-nil empty slice.
	ArgsPrefixes [][]string `json:"argsPrefixes"`
	Allowed      *bool      `json:"allowed"`
}

type Response struct {
	ID              uint       `json:"id"`
	SSHConnectionID uint       `json:"sshConnectionId"`
	Program         string     `json:"program"`
	ArgsPrefixes    [][]string `json:"argsPrefixes"`
	Allowed         bool       `json:"allowed"`
}

func toResponse(rule Rule) Response {
	return Response{
		ID:              rule.ID,
		SSHConnectionID: rule.SSHConnectionID,
		Program:         rule.Program,
		ArgsPrefixes:    [][]string(rule.ArgsPrefixes),
		Allowed:         rule.Allowed,
	}
}
