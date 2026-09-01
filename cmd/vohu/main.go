package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Aliizi83/vohu/internal/agent"
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

	llm := models.NewGeminiAgent(apiKey)

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

	vohuAgent := agent.New(llm, toolRegistry, string(models.GeminiFlashLight35))

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

		turnStart := len(messages)

		updated, err := vohuAgent.Run(ctx, messages)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		messages = updated

		for _, m := range messages[turnStart:] {
			switch {
			case m.Role == "tool" && m.ToolResults != nil:
				for _, result := range *m.ToolResults {
					fmt.Printf("Tool result: %+v\n", result.Result)
				}

			case m.Role == "assistant" && m.ToolCalls == nil:
				fmt.Printf("Gemini: %s\n\n", m.Content)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("input error: %v", err)
	}

}
