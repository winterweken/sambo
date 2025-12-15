package confirm

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Action prompts the user for confirmation of a destructive action
// Returns true if the user confirms, false otherwise
// If skipConfirm is true, skips the prompt and returns true
func Action(message string, skipConfirm bool) bool {
	if skipConfirm {
		return true
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", message)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// Dangerous prompts for confirmation with a warning for especially dangerous actions
func Dangerous(message string, skipConfirm bool) bool {
	if skipConfirm {
		return true
	}

	fmt.Println("\n⚠️  WARNING: This action is potentially destructive!")
	return Action(message, false)
}
