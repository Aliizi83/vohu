package providerkey

type SetKeyRequest struct {
	Provider    Provider `json:"provider" binding:"required,oneof=gemini anthropic openai"`
	APIKey      string   `json:"apiKey" binding:"required"`
	BaseURL     string   `json:"baseUrl"`
	WorkspaceID string   `json:"workspaceId"`
}

// Response never includes the API key — write-only from the API's
// perspective, same as SSH connection secrets and user password hashes.
type Response struct {
	Provider    Provider `json:"provider"`
	BaseURL     string   `json:"baseUrl,omitempty"`
	WorkspaceID string   `json:"workspaceId,omitempty"`
}

func toResponse(key Key) Response {
	return Response{
		Provider:    key.Provider,
		BaseURL:     key.BaseURL,
		WorkspaceID: key.WorkspaceID,
	}
}
