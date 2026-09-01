package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Aliizi83/vohu/internal/agent"
	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/ai_model/models"
	"github.com/Aliizi83/vohu/internal/tools"
	"github.com/Aliizi83/vohu/internal/tools/command"
	"github.com/Aliizi83/vohu/internal/tools/system_tools"
)

type modelOption struct {
	label    string
	provider string
	model    string
	baseURL  string
}

var modelOptions = []modelOption{
	{label: "Gemini Flash", provider: "gemini", model: string(models.GeminiFlash)},
	{label: "Gemini Flash Lite 3.5", provider: "gemini", model: string(models.GeminiFlashLight35)},
	{label: "Claude Opus 5", provider: "anthropic", model: string(models.ClaudeOpus5)},
	{label: "Claude Sonnet 5", provider: "anthropic", model: string(models.ClaudeSonnet5)},
	{label: "Claude Haiku 4.5", provider: "anthropic", model: string(models.ClaudeHaiku45)},
	{label: "OpenAI-compatible (custom URL + model)", provider: "openai-compatible"},
}

func chooseModel(scanner *bufio.Scanner) modelOption {
	fmt.Println("Which model would you like to use?")

	for i, option := range modelOptions {
		if option.model != "" {
			fmt.Printf("  %d. %s (%s)\n", i+1, option.label, option.model)
		} else {
			fmt.Printf("  %d. %s\n", i+1, option.label)
		}
	}

	var choice modelOption

	for {
		fmt.Print("Choice: ")

		if !scanner.Scan() {
			log.Fatal("no model selected")
		}

		input := strings.TrimSpace(scanner.Text())

		index, err := strconv.Atoi(input)
		if err != nil || index < 1 || index > len(modelOptions) {
			fmt.Printf("Please enter a number between 1 and %d.\n", len(modelOptions))
			continue
		}

		choice = modelOptions[index-1]
		break
	}

	// OpenAI-compatible providers (OpenAI itself, DeepSeek, Groq, a local
	// Ollama server, ...) all speak the same wire format — they only differ
	// by base URL and model name, so ask for those directly instead of
	// hardcoding an entry per provider.
	if choice.provider == "openai-compatible" {
		choice.baseURL = promptLine(scanner, "Base URL (leave empty for OpenAI itself): ")
		choice.model = promptLine(scanner, "Model name (e.g. gpt-4o, deepseek-chat): ")
	}

	return choice
}

func promptLine(scanner *bufio.Scanner, prompt string) string {
	fmt.Print(prompt)

	if !scanner.Scan() {
		log.Fatal("no input provided")
	}

	return strings.TrimSpace(scanner.Text())
}

func newLLM(choice modelOption) ai_model.LLM {
	switch choice.provider {

	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			log.Fatal("GEMINI_API_KEY environment variable is not set")
		}
		return models.NewGeminiAgent(apiKey)

	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			log.Fatal("ANTHROPIC_API_KEY environment variable is not set")
		}
		// Optional: only needed if ApiKey is an identity-linked key tied to
		// an organization with more than one workspace.
		workspaceID := os.Getenv("ANTHROPIC_WORKSPACE_ID")
		return models.NewAnthropicAgent(apiKey, workspaceID)

	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			log.Fatal("OPENAI_API_KEY environment variable is not set")
		}
		return models.NewOpenAIAgent(apiKey, choice.baseURL)

	case "deepseek":
		apiKey := os.Getenv("DEEPSEEK_API_KEY")
		if apiKey == "" {
			log.Fatal("DEEPSEEK_API_KEY environment variable is not set")
		}
		return models.NewOpenAIAgent(apiKey, choice.baseURL)

	default:
		log.Fatalf("unknown provider: %s", choice.provider)
		return nil
	}
}

func main() {
	ctx := context.Background()

	scanner := bufio.NewScanner(os.Stdin)

	choice := chooseModel(scanner)

	llm := newLLM(choice)

	// TESTING ONLY: prohibited-mode (deny-list) policy — everything is
	// allowed except what's explicitly blocked below. Switch back to
	// PolicyModeAccept (allow-list) before using this for anything beyond
	// local, throwaway testing.
	policy := command.NewCommandPolicy(
		command.PolicyModeProhibited,
		[]command.Rule{
			{Program: "rm", Allowed: false},
			{Program: "rmdir", Allowed: false},
			{Program: "dd", Allowed: false},
			{Program: "mkfs", Allowed: false},
			{Program: "fdisk", Allowed: false},
			{Program: "parted", Allowed: false},
			{Program: "shutdown", Allowed: false},
			{Program: "reboot", Allowed: false},
			{Program: "poweroff", Allowed: false},
			{Program: "halt", Allowed: false},
			{Program: "init", Allowed: false},
			{Program: "systemctl", Allowed: false},
			{Program: "service", Allowed: false},
			{Program: "kill", Allowed: false},
			{Program: "killall", Allowed: false},
			{Program: "pkill", Allowed: false},
			{Program: "passwd", Allowed: false},
			{Program: "useradd", Allowed: false},
			{Program: "userdel", Allowed: false},
			{Program: "usermod", Allowed: false},
			{Program: "groupadd", Allowed: false},
			{Program: "groupdel", Allowed: false},
			{Program: "chown", Allowed: false},
			{Program: "chmod", Allowed: false},
			{Program: "iptables", Allowed: false},
			{Program: "ufw", Allowed: false},
			{Program: "crontab", Allowed: false},
			{Program: "sudo", Allowed: false},
			{Program: "su", Allowed: false},
			{Program: "visudo", Allowed: false},
			{
				Program: "git",
				ArgsPrefixes: [][]string{
					{"push", "--force"},
					{"push", "-f"},
					{"reset", "--hard"},
					{"clean", "-f"},
				},
				Allowed: false,
			},
			{
				Program: "docker",
				ArgsPrefixes: [][]string{
					{"rm"},
					{"rmi"},
					{"system", "prune"},
				},
				Allowed: false,
			},
		},
	)

	executor := command.NewLocalExecutor(policy)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(system_tools.NewCurrentSystemTime())
	toolRegistry.Register(command.NewTool(executor))

	vohuAgent := agent.New(llm, toolRegistry, choice.model)

	messages := make([]ai_model.Message, 0)

	fmt.Println("Vohu Chat")
	fmt.Printf("Using %s (%s). Type 'exit' to quit.\n", choice.label, choice.model)
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
			Role:    ai_model.RoleUser,
			Content: input,
		})

		turnStart := len(messages)

		fmt.Print("Assistant: ")

		updated, err := vohuAgent.Run(ctx, messages, func(chunk string) {
			fmt.Print(chunk)
		})
		fmt.Println()

		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		messages = updated

		fmt.Println()

		for _, m := range messages[turnStart:] {
			if m.Role == ai_model.RoleTool && m.ToolResults != nil {
				for _, result := range *m.ToolResults {
					fmt.Printf("Tool result: %+v\n", result.Result)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("input error: %v", err)
	}

}
