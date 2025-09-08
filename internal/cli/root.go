package cli

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"a-a/internal/actions"
	"a-a/internal/listener"
	"a-a/internal/parser"

	"github.com/spf13/cobra"
)

var highPriorityQueue = make(chan *parser.ExecutionPlan, 100)
var normalPriorityQueue = make(chan *parser.ExecutionPlan, 100)

func worker() {
	for {
		select {
		case plan := <-highPriorityQueue:
			executePlan(plan)
		case plan := <-normalPriorityQueue:
			executePlan(plan)
		}
	}
}

func resolvePayload(payload map[string]any, results map[string]map[string]any, m *sync.Mutex) map[string]any {
	m.Lock()
	defer m.Unlock()

	resolvedPayload := make(map[string]any)
	// Regex to find placeholders like @results.action_id.output_key
	re := regexp.MustCompile(`@results\.(\w+)\.(\w+)`)

	for key, val := range payload {
		strVal, ok := val.(string)
		if !ok {
			resolvedPayload[key] = val
			continue
		}

		// Replace all occurrences of the placeholder.
		resolvedVal := re.ReplaceAllStringFunc(strVal, func(match string) string {
			parts := re.FindStringSubmatch(match)
			actionID := parts[1]
			outputKey := parts[2]

			if resultData, ok := results[actionID]; ok {
				if resultVal, ok := resultData[outputKey]; ok {
					return fmt.Sprintf("%v", resultVal)
				}
			}
			return ""
		})
		resolvedPayload[key] = resolvedVal
	}
	return resolvedPayload
}

func executePlan(plan *parser.ExecutionPlan) {
	results := make(map[string]map[string]any)
	var resultsMutex sync.Mutex

	allActionNames := []string{}

	// Iterate through each stage sequentially.
	for _, stage := range plan.Plan {
		var wg sync.WaitGroup

		// Launch all actions within the current stage in parallel.
		for _, action := range stage.Actions {
			allActionNames = append(allActionNames, fmt.Sprintf("<%s>", action.Action))
			wg.Add(1)
			go func(act parser.Action) {
				defer wg.Done()

				// Before executing, resolve any placeholders in the payload.
				act.Payload = resolvePayload(act.Payload, results, &resultsMutex)

				output, err := actions.Execute(&act)
				if err != nil {
					fmt.Printf("\nError during '%s' (%s): %v\n> ", act.Action, act.ID, err)
					return
				}

				// Safely store the result for future stages to use.
				if output != nil {
					resultsMutex.Lock()
					results[act.ID] = output
					resultsMutex.Unlock()
				}
			}(action)
		}
		// Wait for all goroutines in the current stage to finish before proceeding.
		wg.Wait()
	}

	summary := strings.Join(allActionNames, " ")
	fmt.Println("\n---")
	fmt.Printf("Finished plan: %s\n", summary)
	fmt.Println("---")
	fmt.Print("> ")
}

var rootCmd = &cobra.Command{
	Use:   "assistant",
	Short: "A smart assistant CLI powered by Gemini",
	Long:  `An intelligent assistant that understands your text input, determines your intent, and performs actions asynchronously.`,
	Run: func(cmd *cobra.Command, args []string) {
		go worker()
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

			// Give immediate feedback
			fmt.Println("Generating plan...")

			plan, err := parser.GeneratePlan(inputText)
			if err != nil {
				fmt.Printf("Error generating plan: %v\n", err)
				continue
			}

			PrettyPrintPlan(plan)

			for {
				answer := listener.GetConfirmation("Do you want to execute this plan? [y/n] > ")
				if answer == "y" || answer == "yes" {
					fmt.Println("Plan approved. Executing now...")
					normalPriorityQueue <- plan
					break
				} else if answer == "n" || answer == "no" {
					fmt.Println("Plan cancelled.")
					break
				} else {
					fmt.Println("Invalid input. Please enter 'y' or 'n'.")
				}
			}
		}
	},
}
