package customtool

// CreateToolRequest defines a brand-new tool. Visibility defaults to
// private when omitted.
type CreateToolRequest struct {
	Name         string     `json:"name" binding:"required,max=100"`
	Description  string     `json:"description" binding:"required"`
	ParamsSchema string     `json:"paramsSchema" binding:"required"`
	Visibility   Visibility `json:"visibility" binding:"omitempty,oneof=public private"`
}

// UpdateToolRequest never changes Name — the stable key the agent calls
// it by.
type UpdateToolRequest struct {
	Description  string     `json:"description" binding:"omitempty"`
	ParamsSchema string     `json:"paramsSchema" binding:"omitempty"`
	Visibility   Visibility `json:"visibility" binding:"omitempty,oneof=public private"`
}

type CreateVersionRequest struct {
	Version    string `json:"version" binding:"required,max=50"`
	SourceCode string `json:"sourceCode" binding:"required"`
}

type CheckSourceRequest struct {
	SourceCode string `json:"sourceCode" binding:"required"`
}

type SourceDiagnostic struct {
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

type CheckSourceResponse struct {
	Success     bool               `json:"success"`
	Diagnostics []SourceDiagnostic `json:"diagnostics"`
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
