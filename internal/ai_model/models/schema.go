package models

import "github.com/Aliizi83/vohu/internal/ai_model"

// jsonSchemaProperties converts a tool's parameter definitions into a
// JSON-Schema "properties" object, keyed by property name. Returns nil if
// the tool takes no parameters.
func jsonSchemaProperties(params ai_model.ToolParameters) map[string]any {
	if len(params.Properties) == 0 {
		return nil
	}

	properties := make(map[string]any, len(params.Properties))

	for name, prop := range params.Properties {
		properties[name] = propertySchema(prop)
	}

	return properties
}

func propertySchema(prop ai_model.ToolProperty) map[string]any {
	schema := map[string]any{
		"type": prop.Type,
	}

	if prop.Description != "" {
		schema["description"] = prop.Description
	}

	if prop.Items != nil {
		schema["items"] = propertySchema(*prop.Items)
	}

	return schema
}
