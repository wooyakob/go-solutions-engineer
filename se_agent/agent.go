package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

type timeArgs struct {
	Timezone string `json:"timezone" jsonschema:"IANA timezone name for the city, e.g. Europe/London or America/New_York"`
}

type timeResult struct {
	Timezone string `json:"timezone"`
	Time     string `json:"time"`
}

func getCurrentTime(_ agent.Context, args timeArgs) (timeResult, error) {
	loc, err := time.LoadLocation(args.Timezone)
	if err != nil {
		return timeResult{}, fmt.Errorf("unknown timezone %q: %w", args.Timezone, err)
	}
	return timeResult{Timezone: args.Timezone, Time: time.Now().In(loc).Format(time.RFC1123)}, nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is not set (add it to .env or export it)", key)
	}
	return v
}

func main() {
	ctx := context.Background()

	// Load .env if present; real environment variables take precedence.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Failed to load .env: %v", err)
	}

	// Ollama exposes an OpenAI-compatible Chat Completions API at /v1.
	model, err := openaimodel.NewModel(ctx, mustEnv("OLLAMA_MODEL"), &openaimodel.ClientConfig{
		BaseURL: mustEnv("OLLAMA_BASE_URL"),
		APIKey:  "ollama", // ignored by Ollama, but the client requires a value
		API:     openaimodel.APIChatCompletions,
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	timeTool, err := functiontool.New(functiontool.Config{
		Name:        "get_current_time",
		Description: "Returns the current time in the given IANA timezone.",
	}, getCurrentTime)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	timeAgent, err := llmagent.New(llmagent.Config{
		Name:        "hello_time_agent",
		Model:       model,
		Description: "Tells the current time in a specified city.",
		Instruction: "You are a helpful assistant that tells the current time in a city. " +
			"Map the city to its IANA timezone and call get_current_time.",
		Tools: []tool.Tool{timeTool},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(timeAgent),
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
