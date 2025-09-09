package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"a-a/internal/display"
	"a-a/internal/listener"
	"a-a/internal/parser"
	"a-a/internal/supervisor"

	"github.com/spf13/cobra"
)

const maxCliHistory = 3

func updateCliHistoryFromResults(cliHistory *[]parser.ConversationTurn, mu *sync.Mutex) {
	for result := range supervisor.ResultChannel {
		mu.Lock()

		newTurn := parser.ConversationTurn{
			UserGoal:      result.OriginalGoal,
			AssistantPlan: result.FinalPlan,
		}
		if result.Error != "" {
			newTurn.ExecutionError = result.Error
		}

		*cliHistory = append(*cliHistory, newTurn)
		if len(*cliHistory) > maxCliHistory {
			*cliHistory = (*cliHistory)[1:]
		}

		mu.Unlock()

		if result.Error != "" {
			fmt.Printf("\n[Mission %s FAILED]\n> ", result.MissionID)
		} else {
			fmt.Printf("\n[Mission %s SUCCEEDED]\n> ", result.MissionID)
		}
	}
}

func handleConfirmations() {
	for req := range supervisor.ConfirmationChannel {
		fmt.Printf("\n\n----------------- USER ACTION REQUIRED -----------------\n")
		fmt.Printf("Mission '%s' requires your approval for a risky plan.\n", req.MissionID)
		fmt.Println(display.FormatPlan(req.Plan))

		var approved bool
		for {
			answer := listener.GetConfirmation("Do you want to execute this plan? [y/n] > ")
			if answer == "y" || answer == "yes" {
				approved = true
				break
			} else if answer == "n" || answer == "no" {
				approved = false
				break
			} else {
				fmt.Println("Invalid input. Please enter 'y' or 'n'.")
			}
		}
		fmt.Printf("------------------------------------------------------\n> ")
		req.Response <- approved
	}
}

var rootCmd = &cobra.Command{
	Use:   "assistant",
	Short: "A smart assistant CLI powered by Gemini",
	Long:  `An intelligent assistant that understands your text input and performs actions autonomously in the background.`,
	Run: func(cmd *cobra.Command, args []string) {
		supervisor.StartSupervisor()
		go handleConfirmations()

		var cliConversationHistory []parser.ConversationTurn
		var historyMutex sync.Mutex
		go updateCliHistoryFromResults(&cliConversationHistory, &historyMutex)

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			fmt.Println("\nGoodbye!")
			os.Exit(0)
		}()

		fmt.Println("Hello! How can I help you today? (type 'exit' or press Ctrl+C to quit)")

		for {
			inputText := listener.GetInput()

			if strings.TrimSpace(strings.ToLower(inputText)) == "exit" {
				fmt.Println("Goodbye!")
				break
			}
			if strings.TrimSpace(inputText) == "" {
				continue
			}

			historyMutex.Lock()
			missionHistory := make([]parser.ConversationTurn, len(cliConversationHistory))
			copy(missionHistory, cliConversationHistory)
			historyMutex.Unlock()

			missionID := supervisor.SubmitMission(inputText, missionHistory)

			fmt.Printf("[Mission %s started]\n", missionID)
		}
	},
}
