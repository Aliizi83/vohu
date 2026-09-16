package customtool

// CreateToolRequest defines a brand-new tool. Visibility defaults to
// private when omitted — same "safe by default" reasoning as everywhere
// else access-gated visibility shows up in this codebase.
type CreateToolRequest struct {
	Name         string     `json:"name" binding:"required,max=100"`
	Description  string     `json:"description" binding:"required"`
	ParamsSchema string     `json:"paramsSchema" binding:"required"`
	Visibility   Visibility `json:"visibility" binding:"omitempty,oneof=public private"`
}

// UpdateToolRequest changes a tool's metadata — never Name (the stable key
// the agent calls it by; renaming would break every existing call site and
// any binary manifest already deployed under the old name) and never
// CreatedByUserID.
type UpdateToolRequest struct {
	Description  string     `json:"description" binding:"omitempty"`
	ParamsSchema string     `json:"paramsSchema" binding:"omitempty"`
	Visibility   Visibility `json:"visibility" binding:"omitempty,oneof=public private"`
}

// CreateVersionRequest adds a new immutable version to an existing tool.
type CreateVersionRequest struct {
	Version    string `json:"version" binding:"required,max=50"`
	SourceCode string `json:"sourceCode" binding:"required"`
}

type Response struct {
	ID              uint       `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	ParamsSchema    string     `json:"paramsSchema"`
	Visibility      Visibility `json:"visibility"`
	CreatedByUserID uint       `json:"createdByUserId"`
}

func toResponse(t Tool) Response {
	return Response{
		ID:              t.ID,
		Name:            t.Name,
		Description:     t.Description,
		ParamsSchema:    t.ParamsSchema,
		Visibility:      t.Visibility,
		CreatedByUserID: t.CreatedByUserID,
	}
}

type VersionResponse struct {
	ID              uint   `json:"id"`
	ToolID          uint   `json:"toolId"`
	Version         string `json:"version"`
	SourceCode      string `json:"sourceCode"`
	CreatedByUserID uint   `json:"createdByUserId"`
}

func toVersionResponse(v ToolVersion) VersionResponse {
	return VersionResponse{
		ID:              v.ID,
		ToolID:          v.ToolID,
		Version:         v.Version,
		SourceCode:      v.SourceCode,
		CreatedByUserID: v.CreatedByUserID,
	}
}
