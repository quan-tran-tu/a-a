package main

import (
	"log"

	"github.com/joho/godotenv"

	"a-a/internal/cli"
	"a-a/internal/llm_client"
	"a-a/internal/logger"
	"a-a/internal/parser"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env not found (continuing)")
	}

	if err := logger.Init("assistant.log"); err != nil {
		log.Fatalf("Fatal Error: Could not initialize logger: %v", err)
	}

	if err := llm_client.InitGeminiClient(); err != nil {
		log.Fatalf("Fatal Error: Could not initialize LLM client: %v", err)
	}

	// Load the action definitions
	parser.LoadRegistry()

	cli.Execute()
}
