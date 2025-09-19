package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"a-a/internal/executor"
	"a-a/internal/logger"
	"a-a/internal/metrics"
	"a-a/internal/parser"
)

var missionQueue = make(chan *Mission, 100) // Main work queue

var curMu sync.Mutex
var curMission *Mission
var curCancel context.CancelFunc

func StartSupervisor() {
	go func() {
		for mission := range missionQueue {
			logger.Log.Printf("[Supervisor] Starting mission '%s' (ID: %s)", mission.OriginalGoal, mission.ID)
			mission.State = StatusRunning
			runMission(mission)
		}
	}()
}

// Submit only after plan is known & confirmed
func SubmitMission(goal string, plan *parser.ExecutionPlan, history []parser.ConversationTurn) string {
	id := uuid.New().String()[:8]
	newMission := &Mission{
		ID:                  id,
		OriginalGoal:        goal,
		State:               "PENDING",
		CurrentAttempt:      0,
		MaxRetries:          3,
		ConversationHistory: history,
		Plan:                plan,
	}
	missionQueue <- newMission
	return id
}

// Cancel a specific mission by ID (works if it's the current running one).
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

func runMission(m *Mission) {
	var finalPlan string
	var finalError error
	var mm *metrics.MissionMetrics

	logger.Log.Printf("Mission '%s' (ID: %s) executing", m.OriginalGoal, m.ID)

	// Save plan string for history/result
	if m.Plan != nil {
		if b, err := json.Marshal(m.Plan); err == nil {
			finalPlan = string(b)
		}
	}

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

	for m.CurrentAttempt < m.MaxRetries {
		m.CurrentAttempt++

		var execErr error
		mm, execErr = executor.ExecutePlan(missionCtx, m.Plan)
		finalError = execErr

		if execErr == nil {
			logger.Log.Printf("Mission '%s' SUCCEEDED (ID: %s).", m.OriginalGoal, m.ID)
			m.State = StatusSucceeded
			break
		}

		// No retry if cancelled
		if errors.Is(execErr, context.Canceled) || strings.Contains(strings.ToLower(execErr.Error()), "cancel") {
			logger.Log.Printf("Mission '%s' CANCELLED (ID: %s).", m.OriginalGoal, m.ID)
			m.State = StatusCancelled
			break
		}

		logger.Log.Printf("Mission '%s' FAILED on attempt %d/%d (ID: %s): %v",
			m.OriginalGoal, m.CurrentAttempt, m.MaxRetries, m.ID, execErr)

		failureTurn := parser.ConversationTurn{
			UserGoal:       m.OriginalGoal,
			AssistantPlan:  finalPlan,
			ExecutionError: execErr.Error(),
		}
		m.ConversationHistory = append(m.ConversationHistory, failureTurn)

		if m.CurrentAttempt >= m.MaxRetries {
			m.State = StatusFailed
			break
		}

		time.Sleep(1 * time.Second) // naive backoff
	}

	result := MissionResult{
		MissionID:    m.ID,
		OriginalGoal: m.OriginalGoal,
		FinalPlan:    finalPlan,
		Metrics:      mm,
	}
	if finalError != nil {
		result.Error = finalError.Error()
	}
	ResultChannel <- result
}

// A list of risky actions that requires user confirmation
func IsPlanRisky(plan *parser.ExecutionPlan) bool {
	for _, stage := range plan.Plan {
		for _, action := range stage.Actions {
			if action.Action == "system.execute_shell" || action.Action == "system.delete_folder" {
				return true
			}
		}
	}
	return false
}
