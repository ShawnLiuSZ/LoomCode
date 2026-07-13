//go:build !windows

package tool

import (
	"os/exec"
	"syscall"
)

// SetProcessGroup configures the command to run in its own process group (Unix)
// so we can kill the whole tree on cancel/timeout.
func SetProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// KillProcessGroup kills the process group (Unix: kill -PGID).
// Returns nil if the process has not started.
func KillProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// ShellName returns the default shell for Unix ("bash").
func ShellName() string {
	return "bash"
}

// ShellArgs returns the shell arguments for running a command on Unix (-c).
func ShellArgs(command string) []string {
	return []string{"-c", command}
}
