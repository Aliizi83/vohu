package agenttool

// UpdateToolRequest only ever changes Visibility — see Tool's doc comment
// on why there's no Name/Description/Implemented to update: those are
// facts about the Go implementation a row is matched to by name, not
// data this API owns.
type UpdateToolRequest struct {
	Visibility Visibility `json:"visibility" binding:"required,oneof=public private"`
}

type Response struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Visibility  Visibility `json:"visibility"`
	Implemented bool       `json:"implemented"`
}

func toResponse(t Tool) Response {
	return Response{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Visibility:  t.Visibility,
		Implemented: t.Implemented,
	}
}
