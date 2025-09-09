package supervisor

type MissionResult struct {
	MissionID    string
	OriginalGoal string
	FinalPlan    string
	Error        string
}

// Global channel for all mission results.
var ResultChannel = make(chan MissionResult, 100)
