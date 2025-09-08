package listener

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func GetInput() string {
	fmt.Print("> ")
	return readLine()
}

func GetConfirmation(prompt string) string {
	fmt.Print(prompt)
	return readLine()
}

func readLine() string {
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(input))
}
