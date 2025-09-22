# AI-Powered Autonomous Assistant (Go + Gemini/Ollama)

A smart command-line assistant that turns natural-language goals into structured plans and executes them **autonomously in the background**.

Built in Go with a clean **supervisor → executor** pipeline, **per-stage concurrency**, **result piping** (`@results.<id>.<key>`), and **strict action validation** via an external `actions.json` registry.

---

## Key Capabilities

### 1) Goal → Plan (LLM-driven)

You type a goal; the assistant:

* Analyzes intent (whether to **show/confirm**, **run manual plans** from a file, or **cancel**).
* Produces a **multi-stage JSON plan** (stages run sequentially; actions in a stage run in parallel).
* Enforces allowed actions & required payloads from `actions.json`.
* Forbids intra-stage dependencies: if A needs B’s output, A goes in a **later** stage. References use:

  ```
  @results.<action_id>.<output_key>
  ```

### 2) Autonomous Execution & Re-Planning

* Accepted plans run **in the background**; you get the prompt back immediately.
* Plans can set `meta.replan=true` and write evidence to `meta.handoff_path`. The supervisor:

  * Persists evidence under a mission scratch dir (`tmp/scratch/<id>/`),
  * Generates a **follow-up plan** with `EVIDENCE` and `PREV_LAST_STAGE`,
  * Shows a **re-plan preview** for approval when needed,
  * Continues stage numbering across plans.

### 3) Safety, Confirmation & Cancellation

* Confirmation is triggered by **intent** (e.g., “show/preview”) or **risky actions** (see below).
* Say “cancel” (or provide an ID) to stop the current mission.
* Risk detection is centralized (`utils.IsPlanRisky`).

### 4) Timeouts, Concurrency & Retries

* **Per-action timeout:** 30s (config in executor).
* **Fail-fast per stage:** first failure cancels the stage.
* **Retries:** up to 3 attempts with brief backoff.
* `flow.foreach` uses bounded concurrency (**8**) and per-item timeout (defaults to 30s or the template action’s `default_timeout_ms` from the registry).
* `web.batch_request` defaults to concurrency **5** (overridable via payload).

### 5) Short-term Memory

* CLI keeps the last **3** turns (goal + plan + error) to give the planner context.

### 6) Metrics & Logging

* Per-action and per-stage timing; printed upon completion.
* All logs go to `assistant.log`.

---

## Implemented Actions (Current)

### File System (`system.*`)

* `system.create_file` — Create an empty file.
* `system.delete_file` — Delete a file.
* `system.create_folder` — Create a directory (parents included).
* `system.delete_folder` — Recursively delete a directory. **(risky)**
* `system.write_file` — **Append** a line (adds `\n`) to a file (creates if missing).
* `system.write_file_atomic` — **Atomic replace/write** (temp file + `rename`, **no** trailing newline).
* `system.read_file` — Returns `{ "content": string }`.
* `system.list_directory` — Returns `{ "entries": []string }`.

### Web I/O (`web.*`)

* `web.request` — Single HTTP request (method, headers supported).
* `web.batch_request` — Fetch many URLs concurrently; returns JSON array of `{url,status_code,content}`. Payload `concurrency` optional (default 5).

### HTML Parsing (`html.*`)

* `html.links` — Extract all `<a>` links `{text,url}`. **Provide `base_url`** to resolve relatives.
* `html.links_bulk` — Like `links` but over a `pages_json` array (`{url,status_code,content}`).
* `html.select_all` — CSS select; returns **outer HTML strings** array.
* `html.select_attr` — CSS select and return a specific attribute from all matches.
* `html.inner_text` — Return the document’s trimmed text.

### Lists & URLs

* `list.pluck` — From an array of **objects**, pluck a field → array of **strings**.
* `list.unique` — Deduplicate any array.
* `list.concat` — Concatenate two arrays.
* `url.normalize` — Resolve relative URLs against `base_url`.

### LLM (`llm.*`)

* `llm.generate_content` — Free-form text → `{ "generated_content": string }`.
* `llm.extract_structured` — Extract data that **conforms to a provided JSON schema** → `{ "json": "<strict JSON string>" }`.
* `llm.select_from_list` — Return a **subset** of an input JSON array verbatim → `{ "selected_json": "<array JSON string>" }`.

*(Gemini models are guard-railed: default `gemini-2.0-flash` unless payload `model` starts with `gemini-`. Ollama accepts any local model name as-is.)*

### Flow Control (`flow.*`)

* `flow.foreach` — Apply a single-item **template action** to each element in `items_json`.

  * **Strict shape**:

    ```json
    {
      "action": "flow.foreach",
      "payload": {
        "items_json": "<JSON array string>",
        "template": {
          "action": "<category.operation>",
          "payload": { "... with {{item}} or {{item.field}} ..." }
        }
      }
    }
    ```
  * Concurrency **8**; per-item timeout from registry default of the template action (else 30s).
  * Returns:

    * `"results_json"` — JSON array of successful inner outputs,
    * `"errors_json"` — JSON array of `{item,error}`.

### Test Utilities (`test.*`)

* `test.sleep` — Sleep for `duration_ms`.
* `test.fail` — Fail after `duration_ms` (useful to test fail-fast).
* `test.sleep_with_return` — Sleep and return `{status,result}` (cancellable).

> **Risky actions** (confirmation enforced via `utils.IsPlanRisky`):
> `system.delete_folder`, reserved: `system.execute_shell`, `system.shutdown`.

---

## Architecture

1. **CLI** (`internal/cli`)

   * REPL loop, recent history, confirmation prompts.
   * Flags: `--llm` (`gemini` | `ollama`), `--model-name`, `--ollama-host`.
   * Handles re-plan previews via channels and y/n approval.

2. **Planning & Intent** (`internal/parser`)

   * `AnalyzeGoalIntent(goal)` → `{ requires_confirmation, run_manual_plans, manual_plans_path, manual_plan_names, cancel, target_mission_id, target_is_previous }`
   * `GeneratePlan(history, goal)` → JSON plan.
   * Loads `actions.json` into a registry that:

     * Builds the planner prompt section,
     * Validates actions & required payloads,
     * **Rejects intra-stage `@results` references**,
     * Honors per-action `default_timeout_ms` for `flow.foreach` items.

3. **Supervisor** (`internal/supervisor`)

   * Work queue, retries, cancellation (`cancel` or by ID).
   * Evidence accumulation & **re-planning** with approval.
   * Maintains a mission scratch dir (`tmp/scratch/<id>`).
   * Continues stage numbering across re-plans.

4. **Executor** (`internal/executor`)

   * Stages sequential; actions parallel with **30s** timeout/action.
   * Replaces payload placeholders from the **mission-shared results map**.
   * Collects per-action and per-stage metrics.

5. **Actions** (`internal/actions/...`)

   * Category dispatch + concrete handlers for `system`, `web`, `html`, `list`, `url`, `llm`, `flow`, `test`.

6. **LLM Client** (`internal/llm_client`)

   * Pluggable providers: **Gemini** and **Ollama**.
   * `Init(Config{Backend, Model, OllamaHost})`, `Generate`, `GenerateJSON`.

7. **Display & Logging**

   * Pretty plan formatting (incl. meta: `plan_type`, `replan`, `handoff_path`).
   * Metrics formatter; logs to `assistant.log`.

---

## The Action Registry (`actions.json`)

Loaded at startup via `parser.LoadRegistry()` and used to:

* Build the planner’s **“available actions & payloads”** prompt section.
* Validate that plans only use **allowed actions** with **required keys** present.
* Supply optional `default_timeout_ms` per action (used by `flow.foreach` for item timeouts).

**Example**

```json
{
  "actions": [
    {
      "name": "system.create_file",
      "description": "Creates a new empty file.",
      "payload_schema": { "required": ["path"] },
      "output_schema": { "keys": [] },
      "default_timeout_ms": 30000
    },
    {
      "name": "llm.generate_content",
      "description": "Generates text.",
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

Ask the assistant to **run plans from a file**. Supported shapes:

1. **Object with `plans` (preferred)**

```json
{
  "plans": [
    { "name": "alpha", "plan": [ { "stage": 1, "actions": [] } ] },
    { "plan": [ { "stage": 1, "actions": [] } ] },
    [ { "stage": 1, "actions": [] } ]
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
show plans from tests/test_plans.json
execute the plans "Create file", "Import Data" in test.json
run all plans in scripts/batch.json
```

---

## Installation & Run

### 1) Environment

Create `.env` as needed:

```env
# For Gemini backend:
GEMINI_API_KEY=your_api_key_here

# For Ollama backend (optional if using default):
OLLAMA_HOST=http://localhost:11434
```

### 2) Build

```bash
go mod tidy
go build -o assistant ./cmd/assistant
```

### 3) Run

```bash
# Development
go run ./cmd/assistant

# Choose backend/model
go run ./cmd/assistant --llm gemini --model-name gemini-2.0-flash

# Use Ollama (example)
go run ./cmd/assistant --llm ollama --model-name llama3.2 --ollama-host http://localhost:11434
```

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
Meta:
  - plan_type: extraction
  - replan: false
  - handoff_path:
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
Do you want to execute this plan? [y/n] 
>
```

---

## Testing

```bash
go test ./...
```

---

## Troubleshooting

* **Missing keys / invalid actions:** Ensure `actions.json` exists and matches the schema.
* **`GEMINI_API_KEY` not set (Gemini backend):** Set it in `.env`.
* **Long steps time out:** Increase the executor’s default per-action timeout or tune `default_timeout_ms` in `actions.json` (used by `flow.foreach` items).

---

## License

MIT
