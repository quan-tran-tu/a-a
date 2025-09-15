package supervisor

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"a-a/internal/executor"
	"a-a/internal/logger"
	"a-a/internal/parser"
)

var missionQueue = make(chan *Mission, 100) // Main work queue

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

func runMission(m *Mission) {
	var finalPlan string
	var finalError error

	logger.Log.Printf("Mission '%s' (ID: %s) executing", m.OriginalGoal, m.ID)

	// Save plan string for history/result
	if m.Plan != nil {
		if b, err := json.Marshal(m.Plan); err == nil {
			finalPlan = string(b)
		}
	}

	for m.CurrentAttempt < m.MaxRetries {
		m.CurrentAttempt++

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = ctx // (reserved)
		cancel()

		execErr := executor.ExecutePlan(m.Plan)
		finalError = execErr

		if execErr == nil {
			logger.Log.Printf("Mission '%s' SUCCEEDED (ID: %s).", m.OriginalGoal, m.ID)
			m.State = StatusSucceeded
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
