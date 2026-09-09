package custommodel

type CreateCustomModelRequest struct {
	Name      string `json:"name" binding:"required,max=100"`
	BaseURL   string `json:"baseUrl" binding:"required,max=255"`
	ModelName string `json:"modelName" binding:"required,max=255"`
	APIKey    string `json:"apiKey" binding:"required"`
}

// Response never includes the API key — write-only from the API's
// perspective, same as provider keys and SSH connection secrets.
type Response struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"baseUrl"`
	ModelName string `json:"modelName"`
}

func toResponse(m CustomModel) Response {
	return Response{
		ID:        m.ID,
		Name:      m.Name,
		BaseURL:   m.BaseURL,
		ModelName: m.ModelName,
	}
}

func toResponses(models []CustomModel) []Response {
	responses := make([]Response, 0, len(models))
	for _, m := range models {
		responses = append(responses, toResponse(m))
	}
	return responses
}
