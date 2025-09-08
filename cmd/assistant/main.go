package main

import (
	"a-a/internal/cli"
	"a-a/internal/llm_client"
	"a-a/internal/parser"

	"log"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	if err := llm_client.InitGeminiClient(); err != nil {
		log.Fatalf("Fatal Error: Could not initialize LLM client: %v", err)
	}

	// Load the action definitions
	parser.LoadRegistry()

	cli.Execute()
}
