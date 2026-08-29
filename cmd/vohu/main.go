package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/ai_model/models"
	"github.com/Aliizi83/vohu/internal/tools"
)

func main() {
	ctx := context.Background()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable is not set")
	}

	agent := models.NewGeminiAgent(
		apiKey,
	)

	messages := make([]ai_model.Message, 0)

	toolDefinitions := []ai_model.ToolDefinition{
		{
			Name:        "get_current_system_time",
			Description: "Returns the current system date and time.",
		},
	}

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Vohu Gemini Chat")
	fmt.Println("Type 'exit' to quit.")
	fmt.Println()

	for {
		fmt.Print("You: ")

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}

		if strings.EqualFold(input, "exit") {
			break
		}

		messages = append(messages, ai_model.Message{
			Role:    "user",
			Content: input,
		})

		response, err := agent.Chat(ctx, ai_model.ChatRequest{
			Messages: messages,
			Model:    string(models.GeminiFlash),
			Tools:    toolDefinitions,
		})
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		fmt.Printf("Gemini: %s\n\n", response.Content)

		messages = append(messages, ai_model.Message{
			Role:    "assistant",
			Content: response.Content,
		})

		for _, call := range response.ToolCalls {

			currentTime := &tools.CurrentSystemTime{}

			if call.Name != currentTime.Name() {
				fmt.Printf("Unknown tool: %s\n", call.Name)
				continue
			}

			result, err := currentTime.Execute(
				ctx,
				call.Arguments,
			)

			if err != nil {
				fmt.Printf("Tool error: %v\n", err)
				continue
			}

			fmt.Printf("Tool result: %v\n", result)

			// Add assistant tool call to conversation
			messages = append(messages, ai_model.Message{
				Role: "assistant",
				ToolCalls: &[]ai_model.ToolCall{
					call,
				},
			})

			// Add tool result to conversation
			messages = append(messages, ai_model.Message{
				Role: "tool",
				ToolResults: &[]ai_model.ToolResult{
					{ToolCallID: call.ID, Name: call.Name, Result: result},
				},
			})
		}

		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("input error: %v", err)
	}
}
