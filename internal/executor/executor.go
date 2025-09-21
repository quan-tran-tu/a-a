package executor

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"a-a/internal/actions"
	"a-a/internal/metrics"
	"a-a/internal/parser"
)

const defaultActionTimeout = 30 * time.Second

func computeActionTimeout(act parser.Action) time.Duration {
	def, ok := parser.GetActionDefinition(act.Action)
	if !ok {
		return defaultActionTimeout
	}
	t := def.DefaultTimeoutMs
	if t <= 0 {
		return defaultActionTimeout
	}
	return time.Duration(t) * time.Millisecond
}

func ExecutePlan(ctx context.Context, plan *parser.ExecutionPlan) (*metrics.MissionMetrics, error) {
	mm := &metrics.MissionMetrics{
		Start: time.Now(),
	}
	defer func() {
		mm.End = time.Now()
		mm.DurationMs = mm.End.Sub(mm.Start).Milliseconds()
	}()

	// Store the output of any actions that returns data, with key is the action's ID
	results := make(map[string]map[string]any)
	var resultsMutex sync.Mutex

	for _, stage := range plan.Plan { // Stages are executed sequentially
		// Check for cancellation before starting the stage
		if err := ctx.Err(); err != nil {
			mm.Succeeded = false
			return mm, err
		}

		sm := metrics.StageMetrics{
			Stage: stage.Stage,
			Start: time.Now(),
		}
		// Cancellation signal for the stage (fail fast mechanism)
		stageCtx, cancelStage := context.WithCancel(ctx)

		var wg sync.WaitGroup // A counter to notify when all goroutines are finished
		errChan := make(chan error, len(stage.Actions))
		actMetChan := make(chan metrics.ActionMetrics, len(stage.Actions))
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
				actionCtx, cancelAction := context.WithTimeout(stageCtx, computeActionTimeout(act))
				defer cancelAction()

				act.Payload = resolvePayload(act.Payload, results, &resultsMutex)

				am := metrics.ActionMetrics{
					ID:     act.ID,
					Action: act.Action,
					Start:  time.Now(),
				}

				output, err := actions.Execute(actionCtx, &act)
				am.End = time.Now()
				am.DurationMs = am.End.Sub(am.Start).Milliseconds()
				am.Success = err == nil
				if err != nil {
					am.Err = err.Error()
				}

				// Collect metrics
				actMetChan <- am
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

		var stageErr error

		select {
		case stageErr = <-errChan:
			cancelStage()
			<-waiter
		case <-waiter:
		}
		close(actMetChan)
		for am := range actMetChan {
			sm.Actions = append(sm.Actions, am)
		}
		sm.End = time.Now()
		sm.Finalize()
		mm.Stages = append(mm.Stages, sm)
		cancelStage()

		// If any action failed, stop the mission.
		if stageErr != nil {
			mm.Succeeded = false
			return mm, stageErr
		}

		// If mission was cancelled during/after stage, stop now.
		if err := ctx.Err(); err != nil {
			mm.Succeeded = false
			return mm, err
		}
	}
	mm.Succeeded = true
	return mm, nil
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
