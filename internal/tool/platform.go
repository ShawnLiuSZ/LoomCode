// Package tool — cross-platform process management interface.
//
// This file documents the cross-platform interface for process group management
// and shell invocation. The actual implementations live in platform_unix.go and
// platform_windows.go, selected via build tags so only one compiles per OS.
//
// Functions:
//
//   - SetProcessGroup(cmd *exec.Cmd): configure the command to run in its own
//     process group (Unix: Setpgid, Windows: CREATE_NEW_PROCESS_GROUP), so the
//     whole tree can be killed on cancel/timeout.
//
//   - KillProcessGroup(cmd *exec.Cmd) error: kill the process group
//     (Unix: kill -PGID, Windows: taskkill /T /F).
//
//   - ShellName() string: return the default shell ("bash" on Unix, "cmd" on Windows).
//
//   - ShellArgs(command string) []string: return the shell arguments for running
//     a command (-c on Unix, /c on Windows).
package tool
