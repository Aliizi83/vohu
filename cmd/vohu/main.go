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
	"github.com/Aliizi83/vohu/internal/tools/command"
	"github.com/Aliizi83/vohu/internal/tools/system_tools"
)

func main() {
	ctx := context.Background()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable is not set")
	}

	agent := models.NewGeminiAgent(apiKey)

	policy := command.NewCommandPolicy(
		command.PolicyModeAccept,
		[]command.Rule{
			{
				Program: "pwd",
				Allowed: true,
			},
			{
				Program: "ls",
				Allowed: true,
			},
			{
				Program: "whoami",
				Allowed: true,
			},
			{
				Program: "git",
				ArgsPrefixes: [][]string{
					{"status"},
					{"log"},
				},
				Allowed: true,
			},
			{
				Program: "docker",
				ArgsPrefixes: [][]string{
					{"ps"},
					{"logs"},
				},
				Allowed: true,
			},
		},
	)

	executor := command.NewLocalExecutor(policy)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(system_tools.NewCurrentSystemTime())
	toolRegistry.Register(command.NewTool(executor))

	messages := make([]ai_model.Message, 0)

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

		// Agent loop
		for {
			response, err := agent.Chat(ctx, ai_model.ChatRequest{
				Messages: messages,
				Model:    string(models.GeminiFlashLight35),
				Tools:    toolRegistry.Definitions(),
			})
			if err != nil {
				fmt.Printf("Error: %v\n\n", err)
				break
			}

			if len(response.ToolCalls) == 0 {
				fmt.Printf("Gemini: %s\n\n", response.Content)

				messages = append(messages, ai_model.Message{
					Role:    "assistant",
					Content: response.Content,
				})

				break
			}

			messages = append(messages, ai_model.Message{
				Role:      "assistant",
				ToolCalls: &response.ToolCalls,
			})

			for _, call := range response.ToolCalls {

				tool, ok := toolRegistry.Get(call.Name)

				if !ok {
					fmt.Printf("Unknown tool: %s\n", call.Name)

					messages = append(messages, ai_model.Message{
						Role: "tool",
						ToolResults: &[]ai_model.ToolResult{
							{
								ToolCallID: call.ID,
								Name:       call.Name,
								Result: fmt.Sprintf(
									"Unknown tool: %s",
									call.Name,
								),
							},
						},
					})

					continue
				}

				result, err := tool.Execute(
					ctx,
					call.Arguments,
				)

				if err != nil {
					fmt.Printf("Tool error: %v\n", err)

					messages = append(messages, ai_model.Message{
						Role: "tool",
						ToolResults: &[]ai_model.ToolResult{
							{
								ToolCallID: call.ID,
								Name:       call.Name,
								Result: fmt.Sprintf(
									"Tool execution failed: %v",
									err,
								),
							},
						},
					})

					continue
				}

				fmt.Printf("Tool result: %+v\n", result)

				messages = append(messages, ai_model.Message{
					Role: "tool",
					ToolResults: &[]ai_model.ToolResult{
						{
							ToolCallID: call.ID,
							Name:       call.Name,
							Result:     result,
						},
					},
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("input error: %v", err)
	}

}
