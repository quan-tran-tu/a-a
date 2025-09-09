package supervisor

import (
	"a-a/internal/parser"
	"testing"
)

func TestIsPlanRisky(t *testing.T) {
	testCases := []struct {
		name        string
		plan        *parser.ExecutionPlan
		expectRisky bool
	}{
		{
			name: "Plan with a risky system.delete_folder action",
			plan: &parser.ExecutionPlan{
				Plan: []parser.ExecutionStage{
					{
						Actions: []parser.Action{
							{Action: "system.delete_folder", Payload: map[string]any{"path": "/tmp"}},
						},
					},
				},
			},
			expectRisky: true,
		},
		{
			name: "Plan with a risky system.execute_shell action",
			plan: &parser.ExecutionPlan{
				Plan: []parser.ExecutionStage{
					{
						Actions: []parser.Action{
							{Action: "system.create_file"},
							{Action: "system.execute_shell"},
						},
					},
				},
			},
			expectRisky: true,
		},
		{
			name: "Plan with only safe actions",
			plan: &parser.ExecutionPlan{
				Plan: []parser.ExecutionStage{
					{
						Actions: []parser.Action{
							{Action: "system.create_file"},
							{Action: "llm.generate_content"},
						},
					},
					{
						Actions: []parser.Action{
							{Action: "web.search"},
						},
					},
				},
			},
			expectRisky: false,
		},
		{
			name:        "Empty plan",
			plan:        &parser.ExecutionPlan{},
			expectRisky: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isRisky := isPlanRisky(tc.plan)

			if isRisky != tc.expectRisky {
				t.Errorf("Expected risky=%v, but got risky=%v", tc.expectRisky, isRisky)
			}
		})
	}
}
