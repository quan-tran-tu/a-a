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
	"a-a/internal/llm_client"
	"a-a/internal/logger"
	"a-a/internal/parser"
	"a-a/internal/supervisor"
	"a-a/internal/utils"
)

const maxCliHistory = 3

// Re-plan approval state (managed only by the main input loop)
var approvalMu sync.Mutex
var awaitingApproval bool
var awaitingMissionID string

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
			lbl := "FAILED"
			lower := strings.ToLower(result.Error)
			if strings.Contains(lower, "cancel") || strings.Contains(lower, "canceled") || strings.Contains(lower, "cancelled") {
				lbl = "CANCELLED"
			}
			listener.AsyncPrintln(fmt.Sprintf("[Mission %s %s]", result.MissionID, lbl))
		} else {
			listener.AsyncPrintln(fmt.Sprintf("[Mission %s SUCCEEDED]", result.MissionID))
		}

		if result.Metrics != nil {
			listener.AsyncPrintln(display.FormatMissionMetrics(result.Metrics))
		}
	}
}

var (
	flagLLM        string
	flagModelName  string
	flagOllamaHost string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&flagLLM, "llm", "gemini", "LLM backend: gemini | ollama")
	rootCmd.PersistentFlags().StringVar(&flagModelName, "model-name", "", "Model name, e.g. gemini-2.0-flash or llama3.2")
	rootCmd.PersistentFlags().StringVar(&flagOllamaHost, "ollama-host", "", "Ollama host URL")
}

// Try to make a file-based plan behave as an initial/seed plan for re-planning.
func ensureSeedPlanDefaults(p *parser.ExecutionPlan) {
	// 1) Force re-plan if user wants to continue after the seed
	if !p.Meta.Replan {
		p.Meta.Replan = true
	}

	// 2) If no handoff_path, try to infer it from a tmp/* write action
	if strings.TrimSpace(p.Meta.HandoffPath) == "" {
		if hp := inferHandoffFromWrites(p); hp != "" {
			p.Meta.HandoffPath = hp
		}
	}
}

// Walk actions in stage order; pick the last tmp/* path written by system.write_file(_atomic)
func inferHandoffFromWrites(p *parser.ExecutionPlan) string {
	var hp string
	for _, st := range p.Plan {
		for _, a := range st.Actions {
			if a.Action == "system.write_file" || a.Action == "system.write_file_atomic" {
				if v, ok := a.Payload["path"].(string); ok {
					s := strings.TrimSpace(v)
					if strings.HasPrefix(s, "tmp/") {
						hp = s
					}
				}
			}
		}
	}
	return hp
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

		if err := llm_client.Init(llm_client.Config{
			Backend:    flagLLM,
			Model:      flagModelName,
			OllamaHost: flagOllamaHost,
		}); err != nil {
			fmt.Println("Failed to init LLM client:", err)
			os.Exit(1)
		}

		supervisor.StartSupervisor()

		var cliConversationHistory []parser.ConversationTurn
		var historyMutex sync.Mutex
		go updateCliHistoryFromResults(&cliConversationHistory, &historyMutex)

		// Plan preview handler: print only; approval is captured by main loop
		go func() {
			for prev := range supervisor.PlanPreviewChannel {
				var plan parser.ExecutionPlan
				_ = json.Unmarshal([]byte(prev.PlanJSON), &plan)
				pretty := display.FormatPlan(&plan)

				listener.AsyncPrintln("\n[Re-plan proposed]\n" + pretty)

				approvalMu.Lock()
				awaitingApproval = true
				awaitingMissionID = prev.MissionID
				approvalMu.Unlock()
			}
		}()

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

			// If awaiting a re-plan approval, interpret this input as y/n and short-circuit.
			approvalMu.Lock()
			if awaitingApproval {
				ans := strings.TrimSpace(strings.ToLower(inputText))
				approved := (ans == "y" || ans == "yes")
				supervisor.PlanApprovalChannel <- supervisor.PlanApproval{
					MissionID: awaitingMissionID,
					Approved:  approved,
				}
				// Clear state
				awaitingApproval = false
				awaitingMissionID = ""
				approvalMu.Unlock()

				if approved {
					listener.AsyncPrintln("[Re-plan approved]")
				} else {
					listener.AsyncPrintln("[Re-plan rejected]")
				}
				continue
			}
			approvalMu.Unlock()

			// Copy LLM context safely
			historyMutex.Lock()
			missionHistory := make([]parser.ConversationTurn, len(cliConversationHistory))
			copy(missionHistory, cliConversationHistory)
			historyMutex.Unlock()

			// Intent analysis
			intentCtx, cancelIntent := context.WithTimeout(context.Background(), 20*time.Second)
			intent, err := parser.AnalyzeGoalIntent(intentCtx, inputText)
			cancelIntent()
			if err != nil {
				listener.AsyncPrintln(fmt.Sprintf("[Intent analysis FAILED] %v", err))
				continue
			}

			// Cancellation flow
			if intent.Cancel {
				if strings.TrimSpace(intent.TargetMissionID) != "" {
					ok, err := supervisor.CancelMission(intent.TargetMissionID)
					if err != nil {
						listener.AsyncPrintln(fmt.Sprintf("[Cancel] %v", err))
					} else if ok {
						listener.AsyncPrintln(fmt.Sprintf("[Cancel] Requested cancellation for mission %s", intent.TargetMissionID))
					} else {
						listener.AsyncPrintln(fmt.Sprintf("[Cancel] Mission %s is not running", intent.TargetMissionID))
					}
				} else {
					id, err := supervisor.CancelMostRecent()
					if err != nil {
						listener.AsyncPrintln(fmt.Sprintf("[Cancel] %v", err))
					} else {
						listener.AsyncPrintln(fmt.Sprintf("[Cancel] Requested cancellation for the current mission (%s)", id))
					}
				}
				continue
			}

			// Seed plan path
			if strings.TrimSpace(intent.SeedPlanPath) != "" {
				plans, err := parser.LoadExecutionPlansFromFile(intent.SeedPlanPath)
				if err != nil {
					listener.AsyncPrintln(fmt.Sprintf("[Seed] %v", err))
					continue
				}

				if len(plans) == 0 {
					listener.AsyncPrintln(fmt.Sprintf("[Seed] No missions found in %s", intent.SeedPlanPath))
					continue
				}
				if len(plans) > 1 {
					listener.AsyncPrintln(fmt.Sprintf("[Seed] Found more than one plan in %s, using the first one", intent.SeedPlanPath))
				}

				// Use the first as the seed plan
				seed := plans[0].Plan

				// Validate the seed plan structure
				if err := parser.ValidatePlan(seed); err != nil {
					listener.AsyncPrintln(fmt.Sprintf("[Seed] Invalid seed plan: %v", err))
					continue
				}

				// Ensure the plan will trigger the re-plan loop and has a handoff evidence path
				ensureSeedPlanDefaults(seed)

				needsConfirm := intent.RequiresConfirmation || utils.IsPlanRisky(seed)
				missionID := supervisor.SubmitMission(inputText, seed, missionHistory, needsConfirm)
				listener.AsyncPrintln(fmt.Sprintf("[Seed] Submitted mission %s using %s as initial plan", missionID, intent.SeedPlanPath))

				// Save to history
				if b, err := json.Marshal(seed); err == nil {
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
				continue
			}

			// Manual plans path
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
					manualNeedsConfirm := intent.RequiresConfirmation || utils.IsPlanRisky(p.Plan)
					missionID := supervisor.SubmitMission(p.Name, p.Plan, missionHistory, manualNeedsConfirm)
					listener.AsyncPrintln(fmt.Sprintf("[Manual] Submitted mission %s (%s)", missionID, p.Name))
				}
				continue
			}

			// Auto plan generation
			planID := uuid.New().String()[:8]
			listener.AsyncPrintln(fmt.Sprintf("Generating plan for the above query, plan's ID: %s ...", planID))

			planBudgetCtx, cancelPlanBudget := context.WithTimeout(context.Background(), 20*time.Second)
			plan, err := parser.GeneratePlan(planBudgetCtx, missionHistory, inputText)
			cancelPlanBudget()
			if err != nil {
				listener.AsyncPrintln(fmt.Sprintf("[Plan generation FAILED] %v", err))
				continue
			}

			// Log full plan
			logger.Log.Printf("Plan %s for goal %q (FULL):\n%s",
				planID, inputText, display.FormatPlanFull(plan))

			// Preview/confirm initial plan if needed
			needsConfirm := intent.RequiresConfirmation || utils.IsPlanRisky(plan)
			if needsConfirm {
				pretty := display.FormatPlan(plan)
				listener.AsyncPrintln(pretty)

				if listener.AskYesNo("Do you want to execute this plan?") {

				} else {
					listener.AsyncPrintln(fmt.Sprintf("[Plan %s REJECTED]", planID))
					continue
				}
			}

			// Start mission in the background (carry the confirmation policy forward)
			missionID := supervisor.SubmitMission(inputText, plan, missionHistory, needsConfirm)

			// Update history
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
