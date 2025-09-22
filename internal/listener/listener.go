package listener

import (
	"fmt"
	"strings"
	"sync"

	"github.com/chzyer/readline"
)

var rl *readline.Instance
var mu sync.Mutex
var holdAsync bool
var heldLines []string

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

func BeginInteractive() {
	mu.Lock()
	holdAsync = true
	mu.Unlock()
}

func EndInteractive() {
	mu.Lock()
	defer mu.Unlock()
	holdAsync = false
	for _, s := range heldLines {
		if rl == nil {
			fmt.Println(s)
		} else {
			_, _ = rl.Write([]byte("\r\n" + s + "\r\n"))
		}
	}
	heldLines = nil
	if rl != nil {
		rl.Refresh()
	}
}

func printAboveUnlocked(s string) {
	if rl == nil {
		fmt.Println(s)
		return
	}
	_, _ = rl.Write([]byte("\r\n" + s + "\r\n"))
	rl.Refresh()
}

func PrintAbove(s string) {
	mu.Lock()
	defer mu.Unlock()
	printAboveUnlocked(s)
}

func GetInput() string {
	line, err := rl.Readline()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

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

func AsyncPrintln(s string) {
	mu.Lock()
	defer mu.Unlock()
	if holdAsync {
		heldLines = append(heldLines, s)
		return
	}
	if rl == nil {
		fmt.Println(s)
		return
	}
	_, _ = rl.Write([]byte("\r\n" + s + "\r\n"))
	rl.Refresh()
}

func AskYesNo(question string) bool {
	BeginInteractive()
	defer EndInteractive()

	PrintAbove(question + " [y/n]")

	for {
		ans := GetConfirmation("> ")
		if ans == "y" || ans == "yes" {
			return true
		}
		if ans == "n" || ans == "no" {
			return false
		}
		PrintAbove("Please answer y/n.")
	}
}
