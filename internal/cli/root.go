package cli

import (
	"fmt"

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
		fmt.Println("Hello! How can I help you today?")
		inputText := listener.GetInput()

		action, err := parser.ParseIntent(inputText)
		if err != nil {
			fmt.Printf("Error parsing intent: %v\n", err)
			return
		}

		if action.Action == "intent.unknown" {
			fmt.Println("I'm sorry, I'm not sure how to help with that.")
			return
		}

		fmt.Printf("Understood. Performing action: '%s' with payload: '%s'\n", action.Action, action.Payload.Value)
		if err := actions.Execute(action); err != nil {
			fmt.Printf("Error executing action: %v\n", err)
		}
	},
}

func init() {}
