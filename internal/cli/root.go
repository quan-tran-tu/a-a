package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"a-a/internal/display"
	"a-a/internal/listener"
	"a-a/internal/logger"
	"a-a/internal/parser"
	"a-a/internal/supervisor"
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

		// Print mission completion without breaking current input
		if result.Error != "" {
			listener.AsyncPrintln(fmt.Sprintf("[Mission %s FAILED]", result.MissionID))
		} else {
			listener.AsyncPrintln(fmt.Sprintf("[Mission %s SUCCEEDED]", result.MissionID))
		}

		if result.Metrics != nil {
			listener.AsyncPrintln(display.FormatMissionMetrics(result.Metrics))
		}
	}
}

var rootCmd = &cobra.Command{
	Use:   "assistant",
	Short: "A smart assistant CLI powered by Gemini",
	Long:  `An intelligent assistant that understands your text input and performs actions autonomously in the background.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := listener.Init(); err != nil {
			fmt.Println("Failed to init terminal input:", err)
			os.Exit(1)
		}
		defer listener.Close()

		supervisor.StartSupervisor()

		var cliConversationHistory []parser.ConversationTurn
		var historyMutex sync.Mutex
		go updateCliHistoryFromResults(&cliConversationHistory, &historyMutex)

		// Graceful shutdown
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			fmt.Println("\nGoodbye!")
			os.Exit(0)
		}()

		listener.AsyncPrintln("Hello! How can I help you today? (type 'exit' or press Ctrl+C to quit)")

		for {
			inputText := listener.GetInput()
			if strings.TrimSpace(strings.ToLower(inputText)) == "exit" {
				fmt.Println("Goodbye!")
				break
			}
			if strings.TrimSpace(inputText) == "" {
				continue
			}

			// Copy LLM context safely
			historyMutex.Lock()
			missionHistory := make([]parser.ConversationTurn, len(cliConversationHistory))
			copy(missionHistory, cliConversationHistory)
			historyMutex.Unlock()

			intentCtx, cancelIntent := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancelIntent()
			intent, err := parser.AnalyzeGoalIntent(intentCtx, inputText)
			if err != nil {
				listener.AsyncPrintln(fmt.Sprintf("[Intent analysis FAILED] %v", err))
				continue
			}

			if intent.RunManualPlans && strings.TrimSpace(intent.ManualPlansPath) != "" {
				plans, err := parser.LoadExecutionPlansFromFile(intent.ManualPlansPath)
				if err != nil {
					listener.AsyncPrintln(fmt.Sprintf("[Manual] %v", err))
					continue
				}
				if len(plans) == 0 {
					listener.AsyncPrintln("[Manual] No missions found in file")
					continue
				}

				// Filter by names if provided (order preserved)
				if len(intent.ManualPlanNames) > 0 {
					selected, missing, err := parser.SelectPlansByNames(plans, intent.ManualPlanNames)
					if err != nil {
						listener.AsyncPrintln(fmt.Sprintf("[Manual] %v", err))
						continue
					}
					if len(missing) > 0 {
						listener.AsyncPrintln(fmt.Sprintf("[Manual] Missing missions: %v", missing))
					}
					plans = selected
				}

				// Show catalog if confirmation requested
				if intent.RequiresConfirmation {
					listener.AsyncPrintln(display.FormatPlansCatalog(intent.ManualPlansPath, plans))
					listener.AsyncPrintln(fmt.Sprintf("About to run %d mission(s) from %s.", len(plans), intent.ManualPlansPath))
					ans := listener.GetConfirmation("Proceed? [y/n] > ")
					if ans != "y" && ans != "yes" {
						listener.AsyncPrintln("[Manual] Cancelled.")
						continue
					}
				}

				// Validate and submit
				valid := make([]parser.NamedPlan, 0, len(plans))
				for _, p := range plans {
					if err := parser.ValidatePlan(p.Plan); err != nil {
						listener.AsyncPrintln(fmt.Sprintf("[Manual] Invalid mission %q: %v", p.Name, err))
						continue
					}
					valid = append(valid, p)
				}
				if len(valid) == 0 {
					listener.AsyncPrintln("[Manual] No valid missions to run.")
					continue
				}
				for _, p := range valid {
					missionID := supervisor.SubmitMission(p.Name, p.Plan, missionHistory)
					listener.AsyncPrintln(fmt.Sprintf("[Manual] Submitted mission %s (%s)", missionID, p.Name))
				}
				continue
			}

			planID := uuid.New().String()[:8]
			listener.AsyncPrintln(fmt.Sprintf("Generating plan for the above query, plan's ID: %s ...", planID))

			// Context for intent recognition + plan generation
			planBudgetCtx, cancelPlanBudget := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancelPlanBudget()
			plan, err := parser.GeneratePlan(planBudgetCtx, missionHistory, inputText)
			if err != nil {
				listener.AsyncPrintln(fmt.Sprintf("[Plan generation FAILED] %v", err))
				continue
			}

			// Log full plan
			logger.Log.Printf("Plan %s for goal %q (FULL):\n%s",
				planID, inputText, display.FormatPlanFull(plan))

			// Log/preview plan for user if confirmation is needed or if risky
			needsConfirm := intent.RequiresConfirmation || supervisor.IsPlanRisky(plan)
			if needsConfirm {
				pretty := display.FormatPlan(plan)
				listener.AsyncPrintln(pretty)

				var approved bool
				for {
					ans := listener.GetConfirmation("Do you want to execute this plan? [y/n] > ")
					if ans == "y" || ans == "yes" {
						approved = true
						break
					} else if ans == "n" || ans == "no" {
						approved = false
						break
					} else {
						listener.AsyncPrintln("Invalid input. Please enter 'y' or 'n'.")
					}
				}

				if !approved {
					listener.AsyncPrintln(fmt.Sprintf("[Plan %s REJECTED]", planID))
					continue
				}
			}

			// Start mission in the background
			missionID := supervisor.SubmitMission(inputText, plan, missionHistory)

			if b, err := json.Marshal(plan); err == nil {
				historyMutex.Lock()
				cliConversationHistory = append(cliConversationHistory, parser.ConversationTurn{
					UserGoal:      inputText,
					AssistantPlan: string(b),
				})
				if len(cliConversationHistory) > maxCliHistory {
					cliConversationHistory = cliConversationHistory[1:]
				}
				historyMutex.Unlock()
			}

			listener.AsyncPrintln(fmt.Sprintf("[Plan %s ACCEPTED] Mission %s started", planID, missionID))
		}
	},
}
