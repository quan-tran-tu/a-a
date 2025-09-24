package listener

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	sc        *bufio.Scanner
	mu        sync.Mutex
	curPrompt = "> "
	prompting bool // True if waiting for input
)

// Init sets up a buffered scanner on stdin.
func Init() error {
	sc = bufio.NewScanner(os.Stdin)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	return nil
}

func SetPrompt(p string) {
	mu.Lock()
	defer mu.Unlock()
	curPrompt = p
}

func printPrompt() {
	mu.Lock()
	defer mu.Unlock()
	fmt.Print(curPrompt)
	prompting = true
}

func GetInput(ctx context.Context) string {
	printPrompt()

	lineCh := make(chan string, 1)

	go func() {
		if sc.Scan() {
			lineCh <- sc.Text()
			return
		}
		lineCh <- ""
	}()

	select {
	case <-ctx.Done():
		mu.Lock()
		if prompting {
			fmt.Print("\n")
			prompting = false
		}
		mu.Unlock()
		return ""
	case line := <-lineCh:
		mu.Lock()
		prompting = false
		mu.Unlock()
		return strings.TrimSpace(line)
	}
}

func GetConfirmation(ctx context.Context, prompt string) string {
	mu.Lock()
	old := curPrompt
	curPrompt = prompt
	mu.Unlock()

	ans := GetInput(ctx)

	mu.Lock()
	curPrompt = old
	mu.Unlock()

	return strings.TrimSpace(strings.ToLower(ans))
}

func AsyncPrintln(s string) {
	mu.Lock()
	defer mu.Unlock()

	if prompting {
		fmt.Print("\r\n" + s + "\r\n")
		fmt.Print(curPrompt)
	} else {
		fmt.Println(s)
	}
}

func AsyncPrintlnNoPrompt(s string) {
	mu.Lock()
	defer mu.Unlock()
	if prompting {
		fmt.Print("\r\n" + s + "\r\n")
		prompting = false
	} else {
		fmt.Println(s)
	}
}

func AsyncPrintBlock(lines ...string) {
	mu.Lock()
	defer mu.Unlock()

	if prompting {
		fmt.Print("\r\n")
		for _, s := range lines {
			fmt.Println(s)
		}
		fmt.Print(curPrompt)
	} else {
		for _, s := range lines {
			fmt.Println(s)
		}
	}
}
