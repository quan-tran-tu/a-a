# AI-Powered Autonomous Assistant (Go + Gemini)

A smart command-line assistant that turns natural language goals into structured plans and executes them **autonomously in the background**.

Built in Go with a clean supervisor → executor architecture, per-stage concurrency, and strict action validation driven by an external `actions.json` registry.

---

## Key Capabilities

### 1) Goal → Plan (LLM-driven)
You type a goal; the assistant:
- Analyzes whether it should show the plan for confirmation.
- Produces a **multi-stage JSON plan** where actions in the same stage run in parallel; stages run sequentially.
- Enforces allowed actions and payloads from `actions.json`.

Plan JSON can reference earlier outputs via:
```
@results.<action_id>.<output_key>
```

### 2) Autonomous Background Execution
Once you accept a plan (when needed), the mission runs in the background and you immediately get your prompt back.

Example:
```
> create hello.md and write "hello" 10 times
Generating plan for the above query, plan's ID: 4f69caf8 ...
[Plan 4f69caf8 ACCEPTED] Mission 1a2b3c4d started
>
... (later) ...
[Mission 1a2b3c4d SUCCEEDED]
```

### 3) Safety & Confirmation
Two mechanisms can trigger a confirmation step:
- **Intent**: If your request explicitly asks to “show/confirm the plan”.
- **Risky actions**: Currently `system.delete_folder` (and reserved `system.execute_shell`) are considered risky.

You’ll see a formatted preview of the plan and must approve it before execution.

### 4) Retries (Fail-fast per stage)
- Actions within a stage run concurrently with a **30s per-action timeout**.
- If any action in a stage fails, the stage cancels (fail-fast), and the mission **retries** up to **3** times (with a brief backoff).
- **Note**: Currently, retries **reuse the same plan**. Auto-**replanning** after failures is on the roadmap (see below).

### 5) Short-term Memory (Context)
The assistant keeps the last **3** conversation turns (goal + plan + any error) to provide context to the planner and display.

### 6) Logging
Everything goes to `assistant.log`: startup, plans (full in logs, pretty-printed in CLI), executions, errors, mission results.

---

## Implemented Actions (Current)

**File system (system.\*)**
- `system.create_file` — Create an empty file.
- `system.delete_file` — Delete a file.
- `system.create_folder` — Create a directory (all parents).
- `system.delete_folder` — Recursively delete a directory.
- `system.write_file` — **Append** a line (adds `\n`) to a file, creating it if needed.
- `system.write_file_atomic` — **Atomic replace/write** using a temp file + rename (no implicit newline).
- `system.read_file` — Read file → returns `{ "content": string }`.
- `system.list_directory` — List entries → returns `{ "entries": []string }`.

**LLM (llm.\*)**
- `llm.generate_content` — Generate text with Gemini → returns `{ "generated_content": string }`.

---

## Architecture breakdown

1. **CLI loop** (`internal/cli`): Reads your goal, captures recent history, and calls the planner.
2. **Planner** (`internal/planner`, `internal/parser`):
   - `AnalyzeGoalIntent` → `{ requires_confirmation: bool }`
   - `GeneratePlan` → JSON plan, validated against **`actions.json`**
3. **Supervisor** (`internal/supervisor`):
   - Queues missions, runs them, retries on failure, emits results to a channel for async status lines.
4. **Executor** (`internal/executor`):
   - Runs stages sequentially; actions within a stage concurrently (30s timeout/action).
   - Replaces payload placeholders with prior results (`@results.<id>.<key>`).
5. **Actions** (`internal/actions`): Dispatches to `system.*` and `llm.*` implementations.

---

## The Action Registry (`actions.json`)

At startup the assistant loads **`actions.json`** (see `parser.LoadRegistry()`), uses it to:
- Generate the planner’s “Available actions” prompt section.
- Validate that any LLM-generated plan only uses **allowed actions** with **required payload keys**.

### Minimal example (`actions.json`)
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

## Installation & Run

1) **Environment** \
Create `.env` with your Gemini key:
```env
GEMINI_API_KEY=your_api_key_here
```

2) **Build**
```bash
go mod tidy
go build -o assistant ./cmd/assistant
```

3) **Run**
```bash
./assistant
# or during development:
go run ./cmd/assistant
```

---

## Usage Examples

### Create and write to a file (with result piping)
Goal:
```
create hello.md and write into it 10 times hello for me, give me the plan first
```

A typical plan (formatted preview) might look like:
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

- **Concurrency:** Actions within a stage run in parallel goroutines.
- **Timeouts:** Each action has a **30s** timeout.
- **Fail-fast per stage:** Any action error cancels the stage; mission may retry (up to **3** attempts).
- **Placeholder resolution:** Payload strings are scanned for `@results.<id>.<key>` and replaced before execution.

---

## Testing

Run all tests:
```bash
go test ./...
```

Notes:
- Some tests reference non-implemented actions like `apps.open` or `web.search` to verify **formatting** and **risk detection** only.
- Risk detection currently flags `system.delete_folder` (and a reserved `system.execute_shell`) as risky.

---

## Roadmap (Near-term)

- **Extend actions:** Web.
- **Multi-plan:** Some goals can require multiple plans, where later plan depends on the output of the previous one.


---

## Troubleshooting

- **`GEMINI_API_KEY` not set**  
  `Fatal Error: Could not initialize LLM client` — Export the key or put it in `.env`.

- **`actions.json` missing or invalid**  
  `Fatal Error: Could not load action registry` — Ensure `actions.json` exists in the working directory and matches the schema above.

- **Long-running steps time out**  
  Increase timeouts in code (`internal/executor/executor.go`) if needed.

---

## Project Layout

```
cmd/
  assistant/
    main.go           # boot: .env → logger → LLM → action registry → CLI
internal/
  actions/
  cli/
    root.go           # REPL loop, history, confirmation, mission submit
    execute.go
  display/            # pretty/compact plan formatting
  executor/           # per-stage concurrency, timeout, @results resolution
  llm_client/         # google.golang.org/genai wrapper
  listener/           # terminal input helper (readline)
  logger/             # file logger (assistant.log)
  parser/             # plan+intent prompts, JSON parse, registry validation
  planner/            # glue: builds intent+plan (with plan ID)
  supervisor/         # mission queue, retries, result channel
```

---

## License

MIT
