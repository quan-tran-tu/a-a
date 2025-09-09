package display

import (
	"a-a/internal/parser"
	"strings"
	"testing"
)

func TestFormatPlan(t *testing.T) {
	plan := &parser.ExecutionPlan{
		Plan: []parser.ExecutionStage{
			{
				Stage: 1,
				Actions: []parser.Action{
					{
						ID:      "create_file",
						Action:  "system.create_file",
						Payload: map[string]any{"path": "test.txt"},
					},
				},
			},
			{
				Stage: 2,
				Actions: []parser.Action{
					{
						ID:      "write_content",
						Action:  "system.write_file",
						Payload: map[string]any{"path": "test.txt", "content": "hello world"},
					},
					{
						ID:      "open_app",
						Action:  "apps.open",
						Payload: map[string]any{"appName": "Notepad"},
					},
				},
			},
		},
	}

	resultString := FormatPlan(plan)

	if !strings.Contains(resultString, "Proposed execution plan") {
		t.Errorf("The plan output is missing the main header.")
	}

	if !strings.Contains(resultString, "Stage 1") {
		t.Errorf("The plan output is missing 'Stage 1'.")
	}
	if !strings.Contains(resultString, "Action: system.create_file (ID: create_file)") {
		t.Errorf("The plan output is missing the action details for stage 1.")
	}

	if !strings.Contains(resultString, "Stage 2") {
		t.Errorf("The plan output is missing 'Stage 2'.")
	}
	if !strings.Contains(resultString, "Action: system.write_file (ID: write_content)") {
		t.Errorf("The plan output is missing the first action for stage 2.")
	}
	if !strings.Contains(resultString, "Action: apps.open (ID: open_app)") {
		t.Errorf("The plan output is missing the second action for stage 2.")
	}

	if !strings.Contains(resultString, "appName: Notepad") {
		t.Errorf("The plan output is missing a payload detail.")
	}
}

func TestFormatPlan_WithLongPayload(t *testing.T) {
	longContent := strings.Repeat("a", 200)
	plan := &parser.ExecutionPlan{
		Plan: []parser.ExecutionStage{
			{
				Stage: 1,
				Actions: []parser.Action{
					{ID: "long_write", Action: "system.write_file", Payload: map[string]any{"content": longContent}},
				},
			},
		},
	}

	resultString := FormatPlan(plan)

	if !strings.Contains(resultString, "...") {
		t.Errorf("Expected long payload content to be truncated with '...', but it wasn't.")
	}
	if strings.Contains(resultString, longContent) {
		t.Errorf("Expected long payload content to be truncated, but the full string was found.")
	}
}
