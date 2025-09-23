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

	"golang.org/x/sync/errgroup"
)

const defaultActionTimeout = 30 * time.Second
const stageConcurrencyDefault = 16

var resultsRef = regexp.MustCompile(`@results\.([A-Za-z0-9_\-]+)\.([A-Za-z0-9_]+)`)

func ExecutePlan(ctx context.Context, plan *parser.ExecutionPlan, sharedResults map[string]map[string]any, sharedMu *sync.Mutex) (*metrics.MissionMetrics, error) {
	mm := &metrics.MissionMetrics{Start: time.Now()}
	defer func() {
		mm.End = time.Now()
		mm.DurationMs = mm.End.Sub(mm.Start).Milliseconds()
	}()

	for _, stage := range plan.Plan { // stages sequential
		if err := ctx.Err(); err != nil {
			mm.Succeeded = false
			return mm, err
		}

		sm := metrics.StageMetrics{Stage: stage.Stage, Start: time.Now()}
		stageCtx, cancelStage := context.WithCancel(ctx)

		// errgroup to run actions in parallel with a concurrency cap
		g, gctx := errgroup.WithContext(stageCtx)
		g.SetLimit(stageConcurrencyDefault)

		var amu sync.Mutex // Protects sm.Actions

		for _, action := range stage.Actions {
			act := action
			g.Go(func() (rerr error) {
				// Panic safety -> convert to error so group cancels cleanly
				defer func() {
					if rec := recover(); rec != nil {
						rerr = fmt.Errorf("panic in action %s: %v", act.Action, rec)
					}
				}()

				// Resolve placeholders using mission-shared results (snapshot inside)
				act.Payload = resolvePayload(act.Payload, sharedResults, sharedMu)

				timeout := defaultActionTimeout
				if def, ok := parser.GetActionDefinition(act.Action); ok && def.DefaultTimeoutMs > 0 {
					timeout = time.Duration(def.DefaultTimeoutMs) * time.Millisecond
				}
				actionCtx, cancelAction := context.WithTimeout(gctx, timeout)
				defer cancelAction()

				am := metrics.ActionMetrics{ID: act.ID, Action: act.Action, Start: time.Now()}
				output, err := actions.Execute(actionCtx, &act)
				am.End = time.Now()
				am.DurationMs = am.End.Sub(am.Start).Milliseconds()
				am.Success = err == nil
				if err != nil {
					am.Err = err.Error()
				}

				amu.Lock()
				sm.Actions = append(sm.Actions, am)
				amu.Unlock()

				if err != nil {
					return fmt.Errorf("action '%s' (%s) failed: %w", act.Action, act.ID, err)
				}
				if output != nil {
					sharedMu.Lock()
					sharedResults[act.ID] = output
					sharedMu.Unlock()
				}
				return nil
			})
		}

		stageErr := g.Wait()
		cancelStage()

		sm.End = time.Now()
		sm.Finalize()
		mm.Stages = append(mm.Stages, sm)

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
	// Take a snapshot under lock
	m.Lock()
	snap := make(map[string]map[string]any, len(results))
	for k, v := range results {
		snap[k] = v
	}
	m.Unlock()
	return resolvePayloadWithSnapshot(payload, snap)
}

func resolvePayloadWithSnapshot(payload map[string]any, snap map[string]map[string]any) map[string]any {
	resolved := make(map[string]any, len(payload))

	for key, val := range payload {
		str, ok := val.(string)
		if !ok {
			resolved[key] = val
			continue
		}

		out := resultsRef.ReplaceAllStringFunc(str, func(match string) string {
			sub := resultsRef.FindStringSubmatch(match)
			if len(sub) != 3 {
				return ""
			}
			actionID, outKey := sub[1], sub[2]
			if m, ok := snap[actionID]; ok {
				if v, ok := m[outKey]; ok {
					return fmt.Sprintf("%v", v)
				}
			}
			return ""
		})
		resolved[key] = out
	}
	return resolved
}
