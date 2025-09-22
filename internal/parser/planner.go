package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"a-a/internal/llm_client"
	"a-a/internal/logger"
)

var registry *ActionRegistry

// Main prompt for generating plan of a mission
func buildPlanPrompt(history []ConversationTurn, userGoal string) string {
	var sb strings.Builder

	sb.WriteString(`You are an expert AI workflow planner. Convert the user's goal into a STRICT JSON execution plan.
Respond ONLY with JSON. No extra text.

OUTPUT SHAPE (not a schema; just the shape):
{
  "meta": {
    "plan_type": "<string>",            // e.g., "exploration", "extraction", "refinement"
    "replan": <bool>,                   // true if a follow-up plan is required
    "handoff_path": "<tmp/... or empty>"
  },
  "plan": [
    { "stage": <int>, "actions": [
      { "id": "<slug>", "action": "<category.operation>", "payload": { ... } }
    ] }
  ]
}

GLOBAL PRINCIPLES
- Stages run SEQUENTIALLY; actions within a stage run IN PARALLEL.
- Actions in the SAME stage must NOT reference "@results.<id>.<key>" of other actions (no dependencies within a stage). If A needs B's output, put A in a LATER stage.
- Later stages may reference earlier outputs with "@results.<action_id>.<key>", given that the outputs are from the actions from previous stages.
- ALWAYS start at stage = 1. The runtime will renumber to continue after previous stages.
- Do NOT invent URLs. Discover links from fetched HTML only.
- Persist temporary artifacts under "tmp/". Final deliverables can be top-level files.
- Write JSON only to ".json"; raw HTML only to ".html"; free text only to ".txt".

EVIDENCE / RE-PLANNING PROTOCOL
- If page structure is unknown, first produce an EXPLORATION plan:
  Stage 1: fetch the seed URL (web.request).
  Stage 2: persist concise evidence JSON to "tmp/<name>.json" with concrete keys that help the next plan, e.g.:
    {
      "seed_url": "<url>",
      "pagination_urls_hint": ["..."],          // if detected (may be empty)
      "profile_link_patterns": ["..."],         // e.g., CSS hints or substrings
      "notes": "minimal, actionable hints only"
    }
  You MAY also persist "tmp/seed.html" if helpful (system.write_file_atomic).
  Set meta.replan = true and meta.handoff_path = the evidence JSON path.
- Follow-up plans MUST reuse the previously fetched HTML or the evidence where possible
  (via @results.<id>.content or by parsing evidence), and should avoid redundant fetches.
- The runtime will renumber stages; do not try to continue numbering yourself.

ACTION USAGE RULES
- NETWORK I/O:
  - Single URL -> "web.request".
  - Many URLs -> "flow.foreach" with template.action="web.request".
- HTML PARSING:
  - Use "html.links" to extract all <a> links (returns an array of {text,url}). Always provide "base_url" so relative hrefs resolve.
  - "html.select_all" returns an array of OUTER HTML STRINGS (NOT objects). Do NOT pipe that into list.pluck.
    If you need hrefs/URLs, prefer "html.links" + list.pluck(field="url").
- LIST DISCIPLINE:
  - If you have an array of OBJECTS and need a field -> "list.pluck(field=...)" first to get an array of STRINGS.
  - Operations like "url.normalize", "list.unique", "list.concat", and "flow.foreach.items_json" expect arrays of STRINGS.
  - Never mix arrays of objects and arrays of strings.
- URL RESOLUTION: Provide "base_url" for "html.links" and "url.normalize".
- FILES: All temp/evidence under "tmp/"; final outputs with correct extension.

FLOW.FOREACH CONTRACT (STRICT)
Use EXACTLY this shape for foreach:
{
  "action": "flow.foreach",
  "payload": {
    "items_json": "<JSON array string>",
    "template": {
      "action": "<category.operation>",      // e.g., "web.request"
      "payload": { ... }                     // use {{item}} or {{item.field}} placeholders
    }
  }
}
Do NOT put "action" at the top-level payload; it MUST be inside template.

IDS
- Action IDs must be short, unique, lowercase. Never reuse a prior action ID; refer to old outputs via @results.

FINAL OUTPUTS
- Persist final deliverables with "system.write_file_atomic" using correct extension.
- Keep JSON outputs compact (no unnecessary prose).

AVAILABLE ACTIONS & PAYLOADS:
`)

	// Include the dynamic registry section
	sb.WriteString(registry.GeneratePromptPart())
	sb.WriteString("\n")

	// History context (if any)
	if len(history) > 0 {
		sb.WriteString("CONVERSATION HISTORY (context):\n")
		for _, turn := range history {
			sb.WriteString(fmt.Sprintf("User Goal: %q\n", turn.UserGoal))
			sb.WriteString(fmt.Sprintf("Previous Assistant Plan: %s\n", turn.AssistantPlan))
			if strings.TrimSpace(turn.ExecutionError) != "" {
				sb.WriteString(fmt.Sprintf("Previous Execution Error: %s\n", turn.ExecutionError))
			}
		}
		sb.WriteString("\n")
	}

	// The actual task
	sb.WriteString("Generate the plan now for this goal:\n")
	sb.WriteString(fmt.Sprintf("User Goal: %q\n", userGoal))
	sb.WriteString("Assistant: ")

	return sb.String()
}

// Prompt for first hand analyzing user intent
func buildIntentPrompt(userGoal string) string {
	var sb strings.Builder
	sb.WriteString("You are an expert user intent analyzer. Respond ONLY with this JSON (no extra text):\n")
	sb.WriteString("{\"requires_confirmation\": <bool>, \"run_manual_plans\": <bool>, \"manual_plans_path\": \"<string or empty>\", \"manual_plan_names\": [<zero or more strings in order>], \"cancel\": <bool>, \"target_mission_id\": \"<string or empty>\", \"target_is_previous\": <bool>}\n\n")

	sb.WriteString("Rules:\n")
	sb.WriteString("- requires_confirmation: true ONLY if the user asks to see/review/confirm/approve/preview before execution OR uses verbs like 'show', 'list', 'preview'.\n")
	sb.WriteString("- run_manual_plans: true if the user asks to execute (or show/preview) plans/missions from a local .json file.\n")
	sb.WriteString("- manual_plans_path: extract the local .json path verbatim (quoted or unquoted). If none, use empty string.\n")
	sb.WriteString("- manual_plan_names: if the user names specific missions, return them in order; otherwise an empty array. If empty and run_manual_plans is true, default behavior is to run ALL missions in the file.\n")
	sb.WriteString("- cancel: true if the user asks to stop/abort/kill/cancel a mission or plan (treat plan == mission).\n")
	sb.WriteString("- target_mission_id: if the user mentions a specific mission/plan ID, put it here (otherwise empty).\n")
	sb.WriteString("- target_is_previous: true if the user says 'previous', 'last', or 'most recent' mission/plan (otherwise false).\n")
	sb.WriteString("- Only consider local files ending with .json. Ignore URLs.\n\n")

	sb.WriteString("Examples:\n")
	sb.WriteString("User: \"show me the plans from tests/test_plans.json\"\n")
	sb.WriteString("Assistant: {\"requires_confirmation\": true, \"run_manual_plans\": true, \"manual_plans_path\": \"tests/test_plans.json\", \"manual_plan_names\": []}\n\n")
	sb.WriteString("User: \"execute the plans 'Create file', 'Import Data' in test.json\"\n")
	sb.WriteString("Assistant: {\"requires_confirmation\": false, \"run_manual_plans\": true, \"manual_plans_path\": \"test.json\", \"manual_plan_names\": [\"Create file\", \"Import Data\"]}\n\n")

	sb.WriteString("User Goal: \"")
	sb.WriteString(userGoal)
	sb.WriteString("\"\nAssistant JSON response: ")
	return sb.String()
}

// Generating plan for a mission
func GeneratePlan(ctx context.Context, history []ConversationTurn, userGoal string) (*ExecutionPlan, error) {
	prompt := buildPlanPrompt(history, userGoal)

	cleanJson, err := llm_client.GenerateJSON(ctx, prompt, "gemini-2.0-flash", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate plan from LLM: %w", err)
	}

	var plan ExecutionPlan
	err = json.Unmarshal([]byte(cleanJson), &plan)
	if err != nil {
		return nil, fmt.Errorf("error parsing generated plan JSON: %v\nRaw Response: %s", err, cleanJson)
	}

	// Validate actions and plan structure
	if err := ValidatePlan(&plan); err != nil {
		logger.Log.Printf("Plan JSON failed validation:\n%s", cleanJson)
		return nil, fmt.Errorf("generated plan invalid: %w", err)
	}

	return &plan, nil
}

func AnalyzeGoalIntent(ctx context.Context, userGoal string) (*GoalIntent, error) {
	prompt := buildIntentPrompt(userGoal)

	cleanJson, err := llm_client.GenerateJSON(ctx, prompt, "gemini-2.0-flash", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate intent from LLM: %w", err)
	}

	var intent GoalIntent
	err = json.Unmarshal([]byte(cleanJson), &intent)
	if err != nil {
		return nil, fmt.Errorf("error parsing generated intent JSON: %v\nRaw Response: %s", err, cleanJson)
	}

	if !intent.RunManualPlans {
		intent.ManualPlansPath = ""
	}
	if !intent.Cancel {
		intent.TargetMissionID = ""
		intent.TargetIsPrevious = false
	}
	return &intent, nil
}
