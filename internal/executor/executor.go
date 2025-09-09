package executor

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"a-a/internal/actions"
	"a-a/internal/parser"
)

const actionTimeout = 30 * time.Second

func ExecutePlan(plan *parser.ExecutionPlan) error {
	// Store the output of any actions that returns data, with key is the action's ID
	results := make(map[string]map[string]any)
	var resultsMutex sync.Mutex

	for _, stage := range plan.Plan { // Stages are executed sequentially
		// Cancellation signal for the stage (fail fast mechanism)
		stageCtx, cancelStage := context.WithCancel(context.Background())

		var wg sync.WaitGroup // A counter to notify when all goroutines are finished
		errChan := make(chan error, len(stage.Actions))

		for _, action := range stage.Actions { // Every actions in the same stage starts in parallel
			wg.Add(1)
			go func(act parser.Action) {
				defer wg.Done() // Decrement counter when action finished

				// Prevent one action crash from crashing the entire application
				defer func() {
					if r := recover(); r != nil {
						errChan <- fmt.Errorf("panic in action %s: %v", act.Action, r)
					}
				}()

				// Add action timeout
				actionCtx, cancelAction := context.WithTimeout(stageCtx, actionTimeout)
				defer cancelAction()

				act.Payload = resolvePayload(act.Payload, results, &resultsMutex)

				output, err := actions.Execute(actionCtx, &act)
				if err != nil {
					errChan <- fmt.Errorf("action '%s' (%s) failed: %w", act.Action, act.ID, err)
					return
				}
				if output != nil {
					resultsMutex.Lock()
					results[act.ID] = output
					resultsMutex.Unlock()
				}
			}(action)
		}

		// Separate goroutine to wait for all other action goroutines to call Done()
		waiter := make(chan struct{})
		go func() {
			wg.Wait()
			close(waiter)
		}()

		select {
		case err := <-errChan:
			cancelStage()
			return err
		case <-waiter:
		}
		cancelStage()
	}
	return nil
}

// Update payload's placeholder with data extracted from results
func resolvePayload(payload map[string]any, results map[string]map[string]any, m *sync.Mutex) map[string]any {
	m.Lock()
	defer m.Unlock()

	resolvedPayload := make(map[string]any)
	re := regexp.MustCompile(`@results\.(\w+)\.(\w+)`) // @results.action_id.output_key

	for key, val := range payload {
		strVal, ok := val.(string)
		if !ok {
			resolvedPayload[key] = val
			continue
		}

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
