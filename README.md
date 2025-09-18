# AI-Powered Autonomous Assistant (Go + Gemini)

A smart command-line assistant that turns natural-language goals into structured plans and executes them **autonomously in the background**.

Built in Go with a clean **supervisor → executor** pipeline, **per-stage concurrency**, **result piping** (`@results.<id>.<key>`), and **strict action validation** via an external `actions.json` registry.

---

## Key Capabilities

### 1) Goal → Plan (LLM-driven)

You type a goal; the assistant:

* Analyzes intent (whether to **show/confirm** a plan, or run **manual plans** from a file).
* Produces a **multi-stage JSON plan** where actions in the same stage run in parallel; stages run sequentially.
* Enforces allowed actions & required payloads from `actions.json`.

Plans can reference earlier outputs via:

```
@results.<action_id>.<output_key>
```

### 2) Autonomous Background Execution

If a plan needs confirmation (intent/risk), you’ll see a preview. Once accepted, the **mission runs in the background** and you immediately get your prompt back.

```
> create hello.md and write "hello" 10 times
Generating plan for the above query, plan's ID: 4f69caf8 ...
[Plan 4f69caf8 ACCEPTED] Mission 1a2b3c4d started
>
... (later) ...
[Mission 1a2b3c4d SUCCEEDED]
```

### 3) Safety & Confirmation

Two triggers:

* **Intent**: e.g., “show/preview/confirm the plan”.
* **Risky actions**: currently `system.delete_folder` (and reserved `system.execute_shell`) are treated as **risky**.

Risky plans require **explicit approval**.

### 4) Retries & Timeouts (Fail-fast per stage)

* Actions inside a stage run concurrently with a **30s per-action timeout**.
* If any action fails, the **stage cancels** (fail-fast).
* Mission **retries up to 3 times** with brief backoff (same plan; re-planning is on the roadmap).

### 5) Short-term Memory

The CLI keeps the last **3** turns (goal + plan + error) to give the planner context.

### 6) Metrics & Logging

* Per-action and per-stage timing printed after completion.
* All logs go to `assistant.log` (startup, plans, runs, errors, results).

---

## Implemented Actions (Current)

### File system (`system.*`)

* `system.create_file` — Create an empty file.
* `system.delete_file` — Delete a file.
* `system.create_folder` — Create a directory (parents included).
* `system.delete_folder` — Recursively delete a directory. **(risky)**
* `system.write_file` — **Append** a line (adds `\n`) to a file (creates if missing).
* `system.write_file_atomic` — **Atomic replace/write** (temp file + `rename`, **no** trailing newline).
* `system.read_file` — Returns `{ "content": string }`.
* `system.list_directory` — Returns `{ "entries": []string }`.

### LLM (`llm.*`)

* `llm.generate_content` — Generate text with Gemini → `{ "generated_content": string }`.
  *Model guardrail:* defaults to `gemini-2.0-flash` unless payload `model` starts with `gemini-`.

### Web (`web.*`)

* Placeholder for web related actions.

---

## Architecture

1. **CLI** (`internal/cli`)
   REPL loop, recent history, confirmation, mission submission.

2. **Planning & Intent** (`internal/parser/planner.go`)

   * `AnalyzeGoalIntent(goal)` → `{ requires_confirmation, run_manual_plans, manual_plans_path, manual_plan_names }`
   * `GeneratePlan(history, goal)` → JSON plan (validated against `actions.json`)

3. **Supervisor** (`internal/supervisor`)
   Mission queue, retries, risk checks, and async result publication.

4. **Executor** (`internal/executor`)

   * Stages sequential; actions within a stage **concurrent** (30s timeout/action).
   * Replaces payload placeholders via `@results.<action_id>.<output_key>` before execution.
   * Collects **per-action** and **per-stage** metrics.

5. **Actions** (`internal/actions/...`)

   * `actions.Execute` only **routes** to category handlers.
   * Implementations live in subpackages:

     * `actions/system`, `actions/llm`, `actions/web`, `actions/test`.

6. **LLM Client** (`internal/llm_client`)
   Thin wrapper around `google.golang.org/genai` with helpers:

   * `InitGeminiClient()`, `Generate()`, `GenerateJSON()`.

7. **Display** (`internal/display`)
   Pretty plan output, catalog printing, and metrics formatting.

---

## The Action Registry (`actions.json`)

Loaded at startup via `parser.LoadRegistry()` and used to:

* Build the planner’s “available actions & payloads” prompt section.
* Validate that LLM plans only use **allowed actions** with **required keys** present.

**Example**

```json
{
  "actions": [
    {
      "name": "system.create_file",
      "description": "Creates a new empty file.",
      "payload_schema": { "required": ["path"] },
      "output_schema": { "keys": [] }
    },
    {
      "name": "system.write_file",
      "description": "Appends a line to a file (adds newline). Creates the file if missing.",
      "payload_schema": { "required": ["path", "content"] },
      "output_schema": { "keys": [] }
    },
    {
      "name": "llm.generate_content",
      "description": "Generates text using Gemini.",
      "payload_schema": { "required": ["prompt"] },
      "output_schema": { "keys": ["generated_content"] }
    },
    {
      "name": "intent.unknown",
      "description": "Represents unknown/neutral intent.",
      "payload_schema": { "required": [] },
      "output_schema": { "keys": [] }
    }
  ]
}
```

---

## Manual Missions from a JSON File

Ask the assistant to **run plans from a file** (single or multiple). Supported shapes:

1. **Object with `plans` (preferred)**

```json
{
  "plans": [
    { "name": "alpha", "plan": [ { "stage": 1, "actions": [] } ] },
    { "plan": [ { "stage": 1, "actions": [] } ] },
    [ { "stage": 1, "actions": [] } ]   // bare array entry
  ]
}
```

2. **Bare array (multi-plan)**

```json
[
  { "name": "alpha", "plan": [] },
  { "plan": [] },
  [ { "stage": 1, "actions": [] } ]
]
```

3. **Single plan**

```json
{ "plan": [] }
```

or

```json
[ { "stage": 1, "actions": [] } ]
```

* Unnamed plans auto-named: `manual:<base>#<index>`.
* Selecting by name is **exact, case-insensitive**.
* If `requires_confirmation` is true, you’ll see a **catalog** and a **prompt** before running.

**Examples**

```
> show plans from tests/test_plans.json
# (prints catalog and asks to proceed)

> execute the plans "Create file", "Import Data" in test.json
# runs selected missions in order; warns if any names are missing

> run all plans in scripts/batch.json
# runs every valid mission in the file
```

---

## Installation & Run

1. **Environment**

```env
# .env
GEMINI_API_KEY=your_api_key_here
```

2. **Build**

```bash
go mod tidy
go build -o assistant ./cmd/assistant
```

3. **Run**

```bash
./assistant
# or during development:
go run ./cmd/assistant
```

---

## Testing

Run all tests:

```bash
go test ./...
```

**Notes:** Current tests only cover a few unit tests.

---

## Usage Example (typical plan preview)

Goal:

```
create hello.md and write into it 10 times hello for me, give me the plan first
```

Preview:

```
Proposed execution plan:
--------------------------------------------------
Stage 1:
  - Action: system.create_file (ID: create_file)
    Payload:
      path: hello.md
Stage 2:
  - Action: llm.generate_content (ID: generate_content)
    Payload:
      prompt: Write the word 'hello' 10 times.
Stage 3:
  - Action: system.write_file_atomic (ID: write_to_file)
    Payload:
      path: hello.md
      content: @results.generate_content.generated_content
--------------------------------------------------
Do you want to execute this plan? [y/n] >
```

---

## Timeouts, Concurrency & Errors

* **Concurrency:** Actions within the same stage run in goroutines.
* **Timeouts:** Each action has a **30s** timeout.
* **Fail-fast:** First action error cancels the stage. Mission may retry up to **3** times.
* **Placeholder resolution:** Strings matching `@results.<id>.<key>` (IDs/keys are `\w+`) are replaced with prior outputs before execution.

---

## Project Layout (Current)

```
cmd/
  assistant/
    main.go            # boot: .env → logger → LLM → action registry → CLI

internal/
  actions/
    actions.go         # dispatcher (category → handler)
    llm/
      llm.go           # llm calls handler
    system/
      system.go        # system ops handler
    test/
      test.go          # helpers handler for testing the architecture flow
    web/
      web.go           # placeholder
  cli/
    root.go            # REPL, history, confirmation, mission submission
    execute.go
  display/
    plans.go           # plan formatting + catalog
    metrics.go         # metrics formatting
  executor/
    executor.go        # per-stage concurrency, timeout, @results resolution
    executor_test.go
  listener/
    listener.go        # readline-based UI helpers
  llm_client/
    gemini.go          # genai client, Generate/GenerateJSON helpers
  logger/
    logger.go
  metrics/
    metrics.go
  parser/
    action.go          # shared plan/registry types
    plan_loader.go     # load/parse multi/single-plan JSON files
    planner.go         # AnalyzeGoalIntent, GeneratePlan
    registry.go        # load/validate action registry, prompt part
    registry_test.go
  supervisor/
    mission.go         # mission model
    result.go          # result channel payload
    supervisor.go      # queue, retries, risk detection
    supervisor_test.go
  utils/
    get_payload.go     # payload parsing helpers
```

---

## Troubleshooting

* **`GEMINI_API_KEY` not set**
  → “Could not initialize LLM client” — put the key in `.env`.

* **`actions.json` missing/invalid**
  → “Could not load action registry” — ensure file exists and matches the schema.

* **Long steps time out**
  → Increase `actionTimeout` in `internal/executor/executor.go`.

---

## License

MIT
