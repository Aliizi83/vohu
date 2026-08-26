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
)

func main() {
	ctx := context.Background()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable is not set")
	}

	agent := models.NewGeminiAgent(
		apiKey,
		models.GeminiFlash,
	)

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

		response, err := agent.Chat(ctx, ai_model.ChatRequest{
			Messages: messages,
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
	}

	if err := scanner.Err(); err != nil {
		log.Printf("input error: %v", err)
	}
}
