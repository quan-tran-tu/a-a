# AI-Powered Autonomous Assistant

This is an intelligent, command-line assistant powered by Google's Gemini models. It's designed to understand natural language goals, create a step-by-step execution plan, and carry out those plans autonomously in the background to accomplish complex tasks.

The assistant is built with a robust, concurrent architecture in Go, featuring a decoupled supervisor-executor model that allows it to handle long-running jobs, self-correct from failures, and interact with the user for critical decisions.

## Core Features

### 1. Natural Language Understanding & Planning
Simply state your goal in plain English. The assistant uses a large language model to parse your intent and generates a structured, multi-stage execution plan to achieve it.

- **Example Goal:** `"I need a new CLI command in my project called 'users'. It should follow the exact same structure and boilerplate as the existing 'products' command located in 'cmd/products/products.go'."`

### 2. Autonomous Background Execution
Missions are executed asynchronously in the background. Once you submit a goal, you get your command prompt back immediately, allowing you to queue up multiple tasks or continue your work while the assistant handles the rest.

```
> Create a summary of the top 3 headlines on Hacker News.
[Mission a1b2c3d4 started]
>
```

### 3. Self-Correction Loop
If an action in a plan fails, the assistant doesn't just give up. It analyzes the error, adds the failure context to its short-term memory, and automatically generates a new, revised plan to overcome the obstacle. This allows it to handle common issues like trying to write a file to a folder that doesn't exist yet.

- **Log Snippet:**
  ```
  [Supervisor] Mission 'create-and-write' FAILED on attempt 1. Error: system.write_file: no such file or directory...
  [Supervisor] Attempting self-correction for mission 'create-and-write'...
  [Supervisor] Mission 'create-and-write' - Attempt 2/3
  [Supervisor] Auto-approving plan for mission 'create-and-write'. Executing now...
  🤖 Proposed execution plan:
  --------------------------------------------------
  Stage 1:
    - Action: system.create_folder (ID: create_dir)
  Stage 2:
    - Action: system.write_file (ID: write_the_file)
  --------------------------------------------------
  ```

### 4. Interactive Confirmation for Risky Actions
The assistant is designed for safety. If a plan contains a potentially destructive action (like `system.delete_folder`), the background mission will pause and prompt you for explicit approval before proceeding.

```
----------------- USER ACTION REQUIRED -----------------
Mission 'cleanup-project' requires your approval for a risky plan.
🤖 Proposed execution plan:
--------------------------------------------------
Stage 1:
  - Action: system.delete_folder (ID: delete_build)
    Payload:
      path: ./build
--------------------------------------------------
Do you want to execute this plan? [y/n] >
```

### 5. Conversational Context (Memory)
The assistant maintains a short-term memory of your last few commands. This allows you to issue follow-up instructions that refer to previous tasks, creating a more natural, conversational workflow.

- **Command 1:** `> create a new file named 'main.go'`
- **Command 2:** `> now, write a simple hello world program in it`

### 6. Powerful Action Toolkit
The assistant comes with a built-in set of "tools" or actions that it can use to interact with your system.

- **File System:** `create_file`, `delete_file`, `write_file`, `create_folder`, `delete_folder`, `read_file`, `list_directory`
- **Web Interaction:** `search` (opens browser), `fetch_page_content` (for scraping and analysis)
- **Developer Tools:** `tools.git` for version control operations (clone, commit, etc.).
- **Application Control:** `apps.open` to launch desktop applications.
- **Internal LLM:** `llm.generate_content` allows the assistant to use an LLM as a tool within a larger plan (e.g., for summarization, code generation, or analysis).

### 7. Comprehensive Logging
All background activities, including generated plans, action executions, errors, and self-correction attempts, are logged to `assistant.log`. This provides a complete audit trail and is invaluable for debugging and understanding the agent's reasoning process.

## How to Run

1.  **Create a `.env` file** in the root directory with your Gemini API key:
    ```
    GEMINI_API_KEY=your_api_key_here
    ```
2.  **Build the application:**
    ```sh
    go build -o assistant ./cmd/assistant
    ```
3.  **Run the assistant:**
    ```sh
    ./assistant
    ```
    Alternatively, you can run it directly for development:
    ```sh
    go run ./cmd/assistant
    ```

## Project Structure
The project is organized with a clean, decoupled architecture:
-   `cmd/`: Main application entry point.
-   `internal/`:
    -   `cli/`: Handles the main user input loop, starts background services, and displays final mission results.
    -   `supervisor/`: The "mission control" that manages the entire lifecycle of autonomous tasks, including the self-correction loop.
    -   `parser/`: Responsible for building prompts and communicating with the LLM to generate structured execution plans.
    -   `executor/`: The low-level engine that runs the stages and actions of a single plan with concurrency and error handling.
    -   `actions/`: The actual implementation of all the tools and capabilities the assistant can use (e.g., file system, web, git).
    -   `llm_client/`: The low-level client for communicating with the external LLM API (Google Gemini).
    -   `display/`: A utility for formatting complex data, like execution plans, into human-readable strings for logging and user confirmation.
    -   `listener/`: A simple, dedicated utility for capturing user input from the command line.
    -   `logger/`: Centralized file logging setup and configuration.