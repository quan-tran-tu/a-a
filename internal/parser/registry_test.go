package parser

import (
	"strings"
	"testing"
)

func TestValidateAction(t *testing.T) {
	registry := &ActionRegistry{
		actionsMap: map[string]ActionDefinition{
			"system.create_file": {
				Name: "system.create_file",
				PayloadSchema: struct {
					Required []string `json:"required"`
				}{
					Required: []string{"path"},
				},
			},
			"llm.generate_content": {
				Name: "llm.generate_content",
				PayloadSchema: struct {
					Required []string `json:"required"`
				}{
					Required: []string{"prompt"},
				},
			},
		},
	}

	testCases := []struct {
		name         string
		actionToTest Action
		expectError  bool
	}{
		{
			name: "Valid action with all required keys",
			actionToTest: Action{
				Action:  "system.create_file",
				Payload: map[string]any{"path": "/tmp/file.txt"},
			},
			expectError: false,
		},
		{
			name: "Action missing a required key",
			actionToTest: Action{
				Action:  "system.create_file",
				Payload: map[string]any{"content": "hello"}, // Missing 'path'
			},
			expectError: true,
		},
		{
			name: "Action that is not defined in the registry",
			actionToTest: Action{
				Action:  "system.non_existent_action",
				Payload: map[string]any{},
			},
			expectError: true,
		},
		{
			name: "Valid action with extra, non-required keys",
			actionToTest: Action{
				Action:  "llm.generate_content",
				Payload: map[string]any{"prompt": "hello", "temperature": 0.9},
			},
			expectError: false, // Extra keys are allowed
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := registry.ValidateAction(&tc.actionToTest)

			if tc.expectError && err == nil {
				t.Error("Expected an error, but got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Did not expect an error, but got: %v", err)
			}
		})
	}
}

func TestGeneratePromptPart(t *testing.T) {
	// ARRANGE: Create a mock ActionRegistry with a variety of action types.
	registry := &ActionRegistry{
		// The order in the slice is important for a predictable output.
		Actions: []ActionDefinition{
			{
				Name:        "system.create_file",
				Description: "Creates a new empty file.",
				PayloadSchema: struct {
					Required []string `json:"required"`
				}{
					Required: []string{"path"},
				},
			},
			{
				Name:        "llm.generate_content",
				Description: "Generates content from an LLM.",
				PayloadSchema: struct {
					Required []string `json:"required"`
				}{
					Required: []string{"prompt"},
				},
				OutputSchema: struct {
					Keys []string `json:"keys"`
				}{
					Keys: []string{"generated_content"},
				},
			},
			{
				Name:        "intent.unknown",
				Description: "Handles unknown intents.",
				PayloadSchema: struct {
					Required []string `json:"required"`
				}{
					Required: []string{}, // Test an empty required list
				},
			},
		},
	}

	// ACT: Call the function we want to test.
	promptPart := registry.GeneratePromptPart()

	// ASSERT: Check for the presence of key phrases and structures.
	// This makes the test robust against minor wording changes.

	// Check that the main header exists.
	if !strings.HasPrefix(promptPart, "AVAILABLE ACTIONS & PAYLOADS:\n") {
		t.Error("Prompt part is missing the correct header.")
	}

	// Check the formatting for the simple file creation action.
	expectedAction1 := "- `system.create_file`: Creates a new empty file. Payload requires keys: `[path]`."
	if !strings.Contains(promptPart, expectedAction1) {
		t.Errorf("Prompt part is missing or has incorrect formatting for system.create_file.\nExpected to find: %s", expectedAction1)
	}

	// Check the formatting for the complex LLM action with an output schema.
	expectedAction2 := "- `llm.generate_content`: Generates content from an LLM. Payload requires keys: `[prompt]`. Returns output with keys: `[generated_content]`."
	if !strings.Contains(promptPart, expectedAction2) {
		t.Errorf("Prompt part is missing or has incorrect formatting for llm.generate_content.\nExpected to find: %s", expectedAction2)
	}

	// Check the formatting for an action with no required keys.
	expectedAction3 := "Payload requires keys: `[]`."
	if !strings.Contains(promptPart, expectedAction3) {
		t.Errorf("Prompt part did not correctly format an empty required keys list.\nExpected to find: %s", expectedAction3)
	}
}
