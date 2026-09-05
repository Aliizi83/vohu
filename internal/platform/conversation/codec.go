package conversation

import (
	"encoding/json"
	"errors"

	"github.com/Aliizi83/vohu/internal/ai_model"
)

// storedToolResult mirrors ai_model.ToolResult but with Error as a plain
// string — ai_model.ToolResult.Error is the error interface, whose
// concrete types (e.g. *errors.errorString) have no exported fields, so
// json.Marshal on it directly would silently drop the message.
type storedToolResult struct {
	ToolCallID string `json:"toolCallId"`
	Name       string `json:"name"`
	Result     any    `json:"result"`
	Error      string `json:"error,omitempty"`
}

// toEntityMessage converts one in-memory message into the DB row shape.
// Sequence/ordering is handled by insertion order (see entity.go), not
// here.
func toEntityMessage(conversationID uint, msg ai_model.Message) (Message, error) {
	row := Message{
		ConversationID: conversationID,
		Role:           string(msg.Role),
		Content:        msg.Content,
	}

	if msg.ToolCalls != nil {
		encoded, err := json.Marshal(*msg.ToolCalls)
		if err != nil {
			return Message{}, err
		}
		row.ToolCallsJSON = string(encoded)
	}

	if msg.ToolResults != nil {
		stored := make([]storedToolResult, 0, len(*msg.ToolResults))
		for _, result := range *msg.ToolResults {
			entry := storedToolResult{
				ToolCallID: result.ToolCallID,
				Name:       result.Name,
				Result:     result.Result,
			}
			if result.Error != nil {
				entry.Error = result.Error.Error()
			}
			stored = append(stored, entry)
		}

		encoded, err := json.Marshal(stored)
		if err != nil {
			return Message{}, err
		}
		row.ToolResultsJSON = string(encoded)
	}

	return row, nil
}

// toAgentMessage is toEntityMessage's inverse, used when loading history
// back out for agent.Agent.Run.
func toAgentMessage(row Message) (ai_model.Message, error) {
	msg := ai_model.Message{
		Role:    ai_model.Role(row.Role),
		Content: row.Content,
	}

	if row.ToolCallsJSON != "" {
		var calls []ai_model.ToolCall
		if err := json.Unmarshal([]byte(row.ToolCallsJSON), &calls); err != nil {
			return ai_model.Message{}, err
		}
		msg.ToolCalls = &calls
	}

	if row.ToolResultsJSON != "" {
		var stored []storedToolResult
		if err := json.Unmarshal([]byte(row.ToolResultsJSON), &stored); err != nil {
			return ai_model.Message{}, err
		}

		results := make([]ai_model.ToolResult, 0, len(stored))
		for _, entry := range stored {
			result := ai_model.ToolResult{
				ToolCallID: entry.ToolCallID,
				Name:       entry.Name,
				Result:     entry.Result,
			}
			if entry.Error != "" {
				result.Error = errors.New(entry.Error)
			}
			results = append(results, result)
		}
		msg.ToolResults = &results
	}

	return msg, nil
}
