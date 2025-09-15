package listener

import (
	"fmt"
	"strings"
	"sync"

	"github.com/chzyer/readline"
)

var rl *readline.Instance
var mu sync.Mutex

func Init() error {
	var err error
	rl, err = readline.NewEx(&readline.Config{
		Prompt:          "> ",
		InterruptPrompt: "",
		EOFPrompt:       "",
	})
	return err
}

func Close() {
	if rl != nil {
		_ = rl.Close()
	}
}

func SetPrompt(p string) {
	mu.Lock()
	defer mu.Unlock()
	if rl != nil {
		rl.SetPrompt(p)
	}
}

// Standard input (returns raw text; caller can lowercase if needed)
func GetInput() string {
	line, err := rl.Readline()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

// Ask y/n (lowercased), with its own prompt, then restore default "> "
func GetConfirmation(prompt string) string {
	mu.Lock()
	old := rl.Config.Prompt
	rl.SetPrompt(prompt)
	mu.Unlock()

	line, err := rl.Readline()
	if err != nil {
		line = ""
	}
	ans := strings.TrimSpace(strings.ToLower(line))

	mu.Lock()
	rl.SetPrompt(old)
	mu.Unlock()
	return ans
}

// Asynchronous safe println that doesn't destroy the current line.
// It writes, then refreshes the input line with whatever the user was typing.
func AsyncPrintln(s string) {
	mu.Lock()
	defer mu.Unlock()
	if rl == nil {
		fmt.Println(s)
		return
	}
	// Move to a new line, print, new line, then redraw the prompt + buffer
	_, _ = rl.Write([]byte("\r\n" + s + "\r\n"))
	rl.Refresh()
}
