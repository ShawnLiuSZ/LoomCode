//go:build windows

package tool

import (
	"os/exec"
	"strconv"
	"syscall"
)

// CREATE_NEW_PROCESS_GROUP is the Windows creation flag for a new process group.
const createNewProcessGroup = 0x00000200

// SetProcessGroup configures the command with creation flags for Windows
// (CREATE_NEW_PROCESS_GROUP) so we can kill the whole tree on cancel/timeout.
func SetProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}

// KillProcessGroup kills the process tree on Windows (taskkill /T /F).
// Returns nil if the process has not started.
func KillProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	return kill.Run()
}

// ShellName returns the default shell for Windows ("cmd").
func ShellName() string {
	return "cmd"
}

// ShellArgs returns the shell arguments for running a command on Windows (/c).
func ShellArgs(command string) []string {
	return []string{"/c", command}
}
