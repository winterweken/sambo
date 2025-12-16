package system

import (
	"os"
	"os/exec"
)

// CommandExecutor defines the interface for executing commands
type CommandExecutor interface {
	Run(name string, args ...string) error
	LookPath(file string) (string, error)
}

// RealCommandExecutor implements CommandExecutor using os/exec
type RealCommandExecutor struct{}

// Run executes the named command with the given arguments
func (e *RealCommandExecutor) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// LookPath searches for an executable in the directories named by the PATH environment variable
func (e *RealCommandExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}
