package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"a-a/internal/display"
	"a-a/internal/executor"
	"a-a/internal/logger"
	"a-a/internal/metrics"
	"a-a/internal/parser"
	"a-a/internal/utils"
)

var missionQueue = make(chan *Mission, 100) // Main work queue

var curMu sync.Mutex
var curMission *Mission
var curCancel context.CancelFunc

var workerWG sync.WaitGroup
var closeOnce sync.Once

const evidenceMaxBytes = 8000
const evidenceSep = "\n\n---\n"

// Append with a separator and keep only the last evidenceMaxBytes bytes.
func appendEvidenceBounded(m *Mission, chunk string) {
	if strings.TrimSpace(chunk) == "" {
		return
	}
	var sb strings.Builder
	if len(m.Evidence) > 0 {
		sb.WriteString(m.Evidence)
		sb.WriteString(evidenceSep)
	}
	sb.WriteString(chunk)
	full := sb.String()

	// Keep tail up to evidenceMaxBytes bytes.
	if len(full) > evidenceMaxBytes {
		full = full[len(full)-evidenceMaxBytes:]
	}
	m.Evidence = full
}

func StartSupervisor() {
	workerWG.Go(func() {
		for mission := range missionQueue {
			logger.Log.Printf("[Supervisor] Starting mission '%s' (ID: %s)", mission.OriginalGoal, mission.ID)
			mission.State = StatusRunning
			runMission(mission)
		}
	})
}

// StopSupervisor closes the queue and waits for the worker to finish (or ctx timeout).
func StopSupervisor(ctx context.Context) {
	// Cancel current mission if any
	_, _ = CancelMostRecent()

	// Close queue only once
	closeOnce.Do(func() { close(missionQueue) })

	done := make(chan struct{})
	go func() {
		workerWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		// timeout/ctx cancel -> give up waiting
	}
}

// Submit mission for execution
func SubmitMission(goal string, plan *parser.ExecutionPlan, history []parser.ConversationTurn, requireConfirm bool) string {
	id := uuid.New().String()[:8]
	newMission := &Mission{
		ID:                  id,
		OriginalGoal:        goal,
		State:               "PENDING",
		CurrentAttempt:      0,
		MaxRetries:          3,
		ConversationHistory: history,
		Plan:                plan,
		RequireConfirm:      requireConfirm,

		// Multi-plan mission state
		ScratchDir: "tmp/scratch/" + id,
		Results:    make(map[string]map[string]any),
		LastStage:  0,
	}
	_ = os.MkdirAll(newMission.ScratchDir, 0o755)
	// Note: if missionQueue is closed due to shutdown, this send will panic.
	missionQueue <- newMission
	return id
}

// Cancel a specific mission by ID.
func CancelMission(id string) (bool, error) {
	curMu.Lock()
	defer curMu.Unlock()

	if curMission == nil || curMission.State != StatusRunning {
		return false, fmt.Errorf("no mission is currently running")
	}
	if id != "" && !strings.EqualFold(curMission.ID, id) {
		return false, fmt.Errorf("mission %s is not running (current running: %s)", id, curMission.ID)
	}
	if curCancel == nil {
		return false, fmt.Errorf("internal error: cancel function not set")
	}
	curCancel()
	return true, nil
}

// Cancel the most recent / current mission.
func CancelMostRecent() (string, error) {
	curMu.Lock()
	defer curMu.Unlock()

	if curMission == nil || curMission.State != StatusRunning {
		return "", fmt.Errorf("no mission is currently running")
	}
	if curCancel == nil {
		return "", fmt.Errorf("internal error: cancel function not set")
	}
	id := curMission.ID
	curCancel()
	return id, nil
}

// Read evidence from the given path, persist a copy in the mission scratch dir.
func readAndPersistEvidence(m *Mission, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		logger.Log.Printf("Evidence read failed (%s): %v", path, err)
		return ""
	}
	content := string(b)
	base := filepath.Base(path)
	ts := time.Now().Format("20060102-150405")
	dst := filepath.Join(m.ScratchDir, ts+"-"+base)
	_ = os.WriteFile(dst, b, 0o644)
	return content
}

func confirmNextPlanIfNeeded(m *Mission, p *parser.ExecutionPlan) bool {
	// Require preview if user asked or plan is risky
	need := m.RequireConfirm || utils.IsPlanRisky(p)
	if !need {
		return true
	}
	b, _ := json.Marshal(p)
	PlanPreviewChannel <- PlanPreview{MissionID: m.ID, PlanJSON: string(b)}

	timer := time.NewTimer(1 * time.Minute)
	defer timer.Stop()

	// Wait for approval (serialized by mission for now)
	for {
		select {
		case ans := <-PlanApprovalChannel:
			if ans.MissionID != m.ID {
				continue
			}
			return ans.Approved
		case <-timer.C:
			return false
		}
	}
}

func runMission(m *Mission) {
	var finalPlan string
	var finalError error

	overall := &metrics.MissionMetrics{Start: time.Now()}
	defer func() {
		overall.End = time.Now()
		overall.DurationMs = overall.End.Sub(overall.Start).Milliseconds()
	}()

	logger.Log.Printf("Mission '%s' (ID: %s) executing", m.OriginalGoal, m.ID)

	planJSON := func(p *parser.ExecutionPlan) string {
		b, _ := json.Marshal(p)
		return string(b)
	}

	// Save initial plan JSON (initial preview handled by CLI)
	if m.Plan != nil {
		finalPlan = planJSON(m.Plan)
	}

	// Wire up cancel for the running mission
	missionCtx, cancel := context.WithCancel(context.Background())
	curMu.Lock()
	curMission = m
	curCancel = cancel
	curMu.Unlock()
	defer func() {
		cancel()
		curMu.Lock()
		if curMission != nil && curMission.ID == m.ID {
			curMission = nil
			curCancel = nil
		}
		curMu.Unlock()
	}()

	for {
		var mm *metrics.MissionMetrics
		var execErr error
		m.CurrentAttempt = 0

		for m.CurrentAttempt < m.MaxRetries {
			m.CurrentAttempt++

			// Continue stage numbering across multi-plan mission
			planForExec := renumberStages(m.Plan, m.LastStage)

			// Execute with mission-shared results map
			mm, execErr = executor.ExecutePlan(missionCtx, planForExec, m.Results, &m.ResultsMu)
			if mm != nil {
				overall.Stages = append(overall.Stages, mm.Stages...)
			}

			if execErr == nil {
				m.LastStage = maxStage(planForExec) // Advance stage cursor
				break                               // Plan succeeded
			}

			finalError = execErr

			// No retry if cancelled
			if errors.Is(execErr, context.Canceled) || strings.Contains(strings.ToLower(execErr.Error()), "cancel") {
				logger.Log.Printf("Mission '%s' CANCELLED (ID: %s).", m.OriginalGoal, m.ID)
				m.State = StatusCancelled
				ResultChannel <- MissionResult{
					MissionID:    m.ID,
					OriginalGoal: m.OriginalGoal,
					FinalPlan:    finalPlan,
					Metrics:      overall,
					Error:        execErr.Error(),
				}
				return
			}

			logger.Log.Printf("Mission '%s' FAILED on attempt %d/%d (ID: %s): %v",
				m.OriginalGoal, m.CurrentAttempt, m.MaxRetries, m.ID, execErr)

			// Remember failure in history
			m.ConversationHistory = append(m.ConversationHistory, parser.ConversationTurn{
				UserGoal:       m.OriginalGoal,
				AssistantPlan:  finalPlan,
				ExecutionError: execErr.Error(),
			})

			if m.CurrentAttempt >= m.MaxRetries {
				m.State = StatusFailed
				break
			}
			time.Sleep(1 * time.Second) // Naive backoff
		}

		// If the plan still failed after retries -> emit result & return
		if execErr != nil {
			ResultChannel <- MissionResult{
				MissionID:    m.ID,
				OriginalGoal: m.OriginalGoal,
				FinalPlan:    finalPlan,
				Metrics:      overall,
				Error:        finalError.Error(),
			}
			return
		}

		logger.Log.Printf("Plan completed (type=%s replan=%v).", m.Plan.Meta.PlanType, m.Plan.Meta.Replan)

		// If plan requests a replan -> accumulate evidence, generate next plan, confirm, loop
		if m.Plan.Meta.Replan {
			// Accumulate evidence
			ev := readAndPersistEvidence(m, m.Plan.Meta.HandoffPath)
			appendEvidenceBounded(m, ev)

			// Build new goal with bounded evidence and PREV_LAST_STAGE
			newGoal := fmt.Sprintf("%s\n\nPREV_LAST_STAGE: %d", m.OriginalGoal, m.LastStage)
			if strings.TrimSpace(m.Evidence) != "" {
				newGoal = fmt.Sprintf("%s\n\nEVIDENCE:\n%s", newGoal, m.Evidence)
			}

			// Generate next plan
			planCtx, cancelPlan := context.WithTimeout(context.Background(), 20*time.Second)
			newPlan, genErr := parser.GeneratePlan(planCtx, m.ConversationHistory, newGoal)
			cancelPlan()
			if genErr != nil {
				logger.Log.Printf("Re-plan generation FAILED (mission %s): %v", m.ID, genErr)
				finalError = fmt.Errorf("replan failed: %w", genErr)
				m.State = StatusFailed
				ResultChannel <- MissionResult{
					MissionID:    m.ID,
					OriginalGoal: m.OriginalGoal,
					FinalPlan:    finalPlan,
					Metrics:      overall,
					Error:        finalError.Error(),
				}
				return
			}

			// Disallow reusing any existing action IDs (must reference via @results)
			if err := checkDuplicateActionIDs(newPlan, m.Results); err != nil {
				logger.Log.Printf("Re-plan generation FAILED (mission %s): %v", m.ID, err)
				finalError = fmt.Errorf("replan failed: %w", err)
				m.State = StatusFailed
				ResultChannel <- MissionResult{
					MissionID:    m.ID,
					OriginalGoal: m.OriginalGoal,
					FinalPlan:    finalPlan,
					Metrics:      overall,
					Error:        finalError.Error(),
				}
				return
			}

			// Log full re-plan for audit
			logger.Log.Printf("Proposing re-plan for mission %s (type=%s replan=%v):\n%s",
				m.ID, newPlan.Meta.PlanType, newPlan.Meta.Replan, display.FormatPlanFull(newPlan))

			// Preview/confirm next plan (if required). Abort if user rejects.
			if !confirmNextPlanIfNeeded(m, newPlan) {
				m.State = StatusCancelled
				ResultChannel <- MissionResult{
					MissionID:    m.ID,
					OriginalGoal: m.OriginalGoal,
					FinalPlan:    planJSON(newPlan),
					Metrics:      overall,
					Error:        "replan rejected by user",
				}
				return
			}

			// Save new plan to history, switch, and loop again
			if b, err := json.Marshal(newPlan); err == nil {
				m.ConversationHistory = append(m.ConversationHistory, parser.ConversationTurn{
					UserGoal:      newGoal,
					AssistantPlan: string(b),
				})
			}
			m.Plan = newPlan
			finalPlan = planJSON(newPlan)
			continue
		}

		// No replan requested -> mission complete
		m.State = StatusSucceeded
		overall.Succeeded = true
		ResultChannel <- MissionResult{
			MissionID:    m.ID,
			OriginalGoal: m.OriginalGoal,
			FinalPlan:    finalPlan,
			Metrics:      overall,
		}
		return
	}
}

func renumberStages(p *parser.ExecutionPlan, offset int) *parser.ExecutionPlan {
	if p == nil || offset <= 0 {
		return p
	}
	for i := range p.Plan {
		p.Plan[i].Stage = p.Plan[i].Stage + offset
	}
	return p
}

func maxStage(p *parser.ExecutionPlan) int {
	max := 0
	for _, s := range p.Plan {
		if s.Stage > max {
			max = s.Stage
		}
	}
	return max
}

func checkDuplicateActionIDs(p *parser.ExecutionPlan, existing map[string]map[string]any) error {
	for _, s := range p.Plan {
		for _, a := range s.Actions {
			if _, exists := existing[a.ID]; exists {
				return fmt.Errorf("action id '%s' already exists from a previous plan; use a new id and reference the old output via @results.%s.<key>", a.ID, a.ID)
			}
		}
	}
	return nil
}
