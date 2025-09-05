package listener

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func GetInput() string {
	fmt.Print("> ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}
