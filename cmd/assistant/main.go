package main

import (
	"a-a/internal/cli"
	"a-a/internal/llm_client"
	"a-a/internal/logger"
	"a-a/internal/parser"

	"log"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
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
