package tools

import "github.com/Aliizi83/vohu/internal/ai_model"

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))

	for _, tool := range r.tools {
		result = append(result, tool)
	}

	return result
}

func (r *Registry) Definitions() []ai_model.ToolDefinition {
	result := make([]ai_model.ToolDefinition, 0, len(r.tools))

	for _, tool := range r.tools {
		result = append(result, ai_model.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
		})
	}

	return result
}
