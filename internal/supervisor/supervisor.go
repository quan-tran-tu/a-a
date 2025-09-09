package supervisor

import (
	"a-a/internal/display"
	"a-a/internal/executor"
	"a-a/internal/logger"
	"a-a/internal/parser"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var missionQueue = make(chan *Mission, 100)

func StartSupervisor() {
	go func() {
		for mission := range missionQueue {
			fmt.Printf("\n[Supervisor] Starting new mission '%s' (ID: %s)\n> ", mission.OriginalGoal, mission.ID)
			mission.State = StatusRunning
			runMission(mission)
		}
	}()
}

func SubmitMission(goal string, history []parser.ConversationTurn) string {
	newMission := &Mission{
		ID:                  uuid.New().String()[:8],
		OriginalGoal:        goal,
		State:               "PENDING",
		CurrentAttempt:      0,
		MaxRetries:          3,
		ConversationHistory: history,
	}
	missionQueue <- newMission
	return newMission.ID
}

func runMission(m *Mission) {
	var finalPlan string
	var finalError error

	logger.Log.Printf("Starting new mission '%s' (ID: %s)", m.OriginalGoal, m.ID)

	for m.CurrentAttempt < m.MaxRetries {
		m.CurrentAttempt++
		logger.Log.Printf("Mission '%s' - Attempt %d/%d", m.OriginalGoal, m.CurrentAttempt, m.MaxRetries)

		plan, err := parser.GeneratePlan(m.ConversationHistory, m.OriginalGoal)
		if err != nil {
			logger.Log.Printf("Error generating plan for mission '%s': %v", m.OriginalGoal, err)
			finalError = err
			break
		}

		planBytes, _ := json.Marshal(plan)
		finalPlan = string(planBytes)

		planString := display.FormatPlan(plan)
		logger.Log.Printf("Mission '%s' generated plan:\n%s", m.OriginalGoal, planString)

		if isPlanRisky(plan) {
			logger.Log.Printf("Mission '%s' requires confirmation for a risky plan.", m.OriginalGoal)

			responseChan := make(chan bool)
			ConfirmationChannel <- ConfirmationRequest{
				MissionID: m.ID,
				Plan:      plan,
				Response:  responseChan,
			}

			approved := <-responseChan
			if !approved {
				logger.Log.Printf("User REJECTED the plan for mission '%s'. Aborting...", m.OriginalGoal)
				m.State = StatusFailed
				finalError = fmt.Errorf("user rejected the plan")
				break
			}
			logger.Log.Printf("User APPROVED the plan for mission '%s'. Executing... ", m.OriginalGoal)
		} else {
			logger.Log.Printf("Auto-approving plan for mission '%s'. Executing now...", m.OriginalGoal)
		}

		execErr := executor.ExecutePlan(plan)
		finalError = execErr

		if execErr == nil {
			logger.Log.Printf("Mission '%s' SUCCEEDED.", m.OriginalGoal)
			m.State = StatusSucceeded
			break
		}

		logger.Log.Printf("Mission '%s' FAILED on attempt %d. Error: %v ", m.OriginalGoal, m.CurrentAttempt, execErr)
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

		logger.Log.Printf("Attempting self-correction for mission '%s'... ", m.OriginalGoal)
		time.Sleep(1 * time.Second)
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

func isPlanRisky(plan *parser.ExecutionPlan) bool {
	for _, stage := range plan.Plan {
		for _, action := range stage.Actions {
			if action.Action == "system.execute_shell" || action.Action == "system.delete_folder" {
				return true
			}
		}
	}
	return false
}
