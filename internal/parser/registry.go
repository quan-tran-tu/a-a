package parser

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

type ActionDefinition struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	PayloadSchema struct {
		Required []string `json:"required"`
	} `json:"payload_schema"`
}

type ActionRegistry struct {
	Actions    []ActionDefinition
	actionsMap map[string]ActionDefinition
}

// Reads the action definitions from a JSON file
func LoadActionRegistry(filePath string) (*ActionRegistry, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not read action registry file: %w", err)
	}

	var registry struct {
		Actions []ActionDefinition `json:"actions"`
	}
	if err := json.Unmarshal(file, &registry); err != nil {
		return nil, fmt.Errorf("could not parse action registry JSON: %w", err)
	}

	// Create a map for quick lookups
	actionsMap := make(map[string]ActionDefinition)
	for _, action := range registry.Actions {
		actionsMap[action.Name] = action
	}

	return &ActionRegistry{
		Actions:    registry.Actions,
		actionsMap: actionsMap,
	}, nil
}

// Returns the definition for a specific action
func (r *ActionRegistry) GetDefinition(actionName string) (ActionDefinition, bool) {
	def, found := r.actionsMap[actionName]
	return def, found
}

// Creates the text block for the LLM prompt
func (r *ActionRegistry) GeneratePromptPart() string {
	var sb strings.Builder
	sb.WriteString("AVAILABLE ACTIONS & PAYLOADS:\n")
	for _, action := range r.Actions {
		requiredKeys := strings.Join(action.PayloadSchema.Required, ", ")
		sb.WriteString(fmt.Sprintf("- `%s`: %s Payload requires keys: `[%s]`.\n", action.Name, action.Description, requiredKeys))
	}
	return sb.String()
}

// Checks if a parsed action's payload matches its schema
func (r *ActionRegistry) ValidateAction(action *Action) error {
	def, found := r.GetDefinition(action.Action)
	if !found {
		return fmt.Errorf("action '%s' is not defined in the registry", action.Action)
	}

	for _, requiredKey := range def.PayloadSchema.Required {
		if _, ok := action.Payload[requiredKey]; !ok {
			return fmt.Errorf("action '%s' is missing required payload key: '%s'", action.Action, requiredKey)
		}
	}
	return nil
}

func LoadRegistry() {
	var err error
	registry, err = LoadActionRegistry("actions.json")
	if err != nil {
		log.Fatalf("Fatal Error: Could not load action registry: %v", err)
	}
}
