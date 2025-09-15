package planner

import (
	"github.com/google/uuid"

	"a-a/internal/parser"
)

// Builds intent + plan using a provided planID
func BuildWithID(history []parser.ConversationTurn, userGoal, planID string) (*parser.ExecutionPlan, *parser.GoalIntent, string, error) {
	if planID == "" {
		planID = uuid.New().String()[:8]
	}
	intent, err := parser.AnalyzeGoalIntent(userGoal)
	if err != nil {
		return nil, nil, planID, err
	}
	plan, err := parser.GeneratePlan(history, userGoal)
	if err != nil {
		return nil, nil, planID, err
	}
	return plan, intent, planID, nil
}
