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

// Accept mission-shared results + mutex
func ExecutePlan(ctx context.Context, plan *parser.ExecutionPlan, sharedResults map[string]map[string]any, sharedMu *sync.Mutex) (*metrics.MissionMetrics, error) {
	mm := &metrics.MissionMetrics{Start: time.Now()}
	defer func() {
		mm.End = time.Now()
		mm.DurationMs = mm.End.Sub(mm.Start).Milliseconds()
	}()

	for _, stage := range plan.Plan { // Stages sequential
		if err := ctx.Err(); err != nil {
			mm.Succeeded = false
			return mm, err
		}

		sm := metrics.StageMetrics{Stage: stage.Stage, Start: time.Now()}
		stageCtx, cancelStage := context.WithCancel(ctx)
		defer cancelStage()

		var wg sync.WaitGroup
		errChan := make(chan error, len(stage.Actions))
		actMetChan := make(chan metrics.ActionMetrics, len(stage.Actions))

		for _, action := range stage.Actions { // Actions parallel
			wg.Add(1)
			go func(act parser.Action) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						errChan <- fmt.Errorf("panic in action %s: %v", act.Action, r)
					}
				}()

				actionCtx, cancelAction := context.WithTimeout(stageCtx, defaultActionTimeout)
				defer cancelAction()

				// Resolve placeholders using mission-shared results
				act.Payload = resolvePayload(act.Payload, sharedResults, sharedMu)

				am := metrics.ActionMetrics{ID: act.ID, Action: act.Action, Start: time.Now()}
				output, err := actions.Execute(actionCtx, &act)
				am.End = time.Now()
				am.DurationMs = am.End.Sub(am.Start).Milliseconds()
				am.Success = err == nil
				if err != nil {
					am.Err = err.Error()
				}

				actMetChan <- am
				if err != nil {
					errChan <- fmt.Errorf("action '%s' (%s) failed: %w", act.Action, act.ID, err)
					return
				}
				if output != nil {
					sharedMu.Lock()
					sharedResults[act.ID] = output // Append into mission map
					sharedMu.Unlock()
				}
			}(action)
		}

		waiter := make(chan struct{})
		go func() { wg.Wait(); close(waiter) }()

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

		if stageErr != nil {
			mm.Succeeded = false
			return mm, stageErr
		}
		if err := ctx.Err(); err != nil {
			mm.Succeeded = false
			return mm, err
		}
	}
	mm.Succeeded = true
	return mm, nil
}

func resolvePayload(payload map[string]any, results map[string]map[string]any, m *sync.Mutex) map[string]any {
	m.Lock()
	defer m.Unlock()

	resolved := make(map[string]any)
	re := regexp.MustCompile(`@results\.([A-Za-z0-9_\-]+)\.([A-Za-z0-9_]+)`)

	for key, val := range payload {
		strVal, ok := val.(string)
		if !ok {
			resolved[key] = val
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
		resolved[key] = resolvedVal
	}
	return resolved
}
