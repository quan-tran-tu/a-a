package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"a-a/internal/actions"
	"a-a/internal/listener"
	"a-a/internal/parser"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "assistant",
	Short: "A smart assistant CLI powered by Gemini",
	Long:  `An intelligent assistant that understands your text input, determines your intent, and performs actions.`,
	Run: func(cmd *cobra.Command, args []string) {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)

		go func() {
			<-c
			fmt.Println("\nGoodbye!")
			os.Exit(0)
		}()

		fmt.Println("Hello! How can I help you today? (Type 'exit' or press Ctrl+C to quit)")

		for {
			inputText := listener.GetInput()

			if strings.TrimSpace(strings.ToLower(inputText)) == "exit" {
				fmt.Println("Goodbye!")
				break
			}

			if strings.TrimSpace(inputText) == "" {
				continue
			}

			action, err := parser.ParseIntent(inputText)
			if err != nil {
				fmt.Printf("Error parsing intent: %v\n", err)
				continue // Continue the loop instead of returning
			}

			if action.Action == "intent.unknown" {
				fmt.Println("I'm sorry, I'm not sure how to help with that.")
				continue // Continue the loop instead of returning
			}

			fmt.Printf("Understood. Performing action: '%s' with payload: '%s'\n", action.Action, action.Payload.Value)
			if err := actions.Execute(action); err != nil {
				fmt.Printf("Error executing action: %v\n", err)
				// Continue the loop even if there's an execution error
			}

			fmt.Println()
		}
	},
}

func init() {}
