package confirm

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// ActionWithReader prompts the user for confirmation using the provided reader.
// Returns true if the user confirms, false otherwise.
// If skipConfirm is true, skips the prompt and returns true.
func ActionWithReader(message string, skipConfirm bool, reader io.Reader) bool {
	if skipConfirm {
		return true
	}

	r := bufio.NewReader(reader)
	fmt.Printf("%s [y/N]: ", message)

	response, err := r.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// Action prompts the user for confirmation of a destructive action
// Returns true if the user confirms, false otherwise
// If skipConfirm is true, skips the prompt and returns true
func Action(message string, skipConfirm bool) bool {
	return ActionWithReader(message, skipConfirm, os.Stdin)
}

// DangerousWithReader prompts for confirmation with a warning, using the provided reader.
func DangerousWithReader(message string, skipConfirm bool, reader io.Reader) bool {
	if skipConfirm {
		return true
	}

	fmt.Println("\n⚠️  WARNING: This action is potentially destructive!")
	return ActionWithReader(message, false, reader)
}

// Dangerous prompts for confirmation with a warning for especially dangerous actions
func Dangerous(message string, skipConfirm bool) bool {
	return DangerousWithReader(message, skipConfirm, os.Stdin)
}
