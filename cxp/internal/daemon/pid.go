package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// WritePID writes the current process PID to path.
// Returns an error if a live process already holds the PID file.
// A stale file from a previous crash is silently replaced.
func WritePID(path string) error {
	if pid, err := ReadPID(path); err == nil {
		if processAlive(pid) {
			return fmt.Errorf("daemon already running with PID %d", pid)
		}
		// Stale file from a previous crash — remove it and continue.
		_ = os.Remove(path)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644)
}

// ReadPID reads and returns the PID stored at path.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// RemovePID removes the PID file at path. Not-found errors are silently ignored.
func RemovePID(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// processAlive reports whether a process with the given PID is alive.
// On Unix, sending signal 0 probes the process without actually signalling it.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
