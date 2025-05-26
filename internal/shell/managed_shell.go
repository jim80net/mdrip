package shell

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	// It's good practice to include logging.
	// If slog is available and used in the project, prefer it.
	// Otherwise, standard log is fine.
	// For now, let's assume standard log.
	"log"
)

// ManagedShell represents a shell process that can be controlled.
type ManagedShell struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// NewManagedShell creates a new ManagedShell instance.
// It initializes the command but does not start it.
func NewManagedShell(shellPath string) (*ManagedShell, error) {
	if shellPath == "" {
		return nil, fmt.Errorf("shellPath cannot be empty")
	}
	return &ManagedShell{
		cmd: exec.Command(shellPath),
	}, nil
}

// Start starts the shell process and sets up pipes for stdin, stdout, and stderr.
func (ms *ManagedShell) Start() error {
	var err error

	ms.stdin, err = ms.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	ms.stdout, err = ms.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	ms.stderr, err = ms.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := ms.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start shell process: %w", err)
	}
	log.Println("Shell process started.")
	return nil
}

// Execute sends a command to the shell and reads its output.
// It uses a delimiter to identify the end of the command's output.
func (ms *ManagedShell) Execute(command string) (string, string, error) {
	if ms.cmd == nil || ms.cmd.Process == nil {
		return "", "", fmt.Errorf("shell process not started")
	}

	delimiter := "END_OF_COMMAND_OUTPUT_DELIMITER"
	fullCommand := command + "\necho \"" + delimiter + "\"\n"

	if _, err := ms.stdin.Write([]byte(fullCommand)); err != nil {
		return "", "", fmt.Errorf("failed to write to stdin: %w", err)
	}

	// Buffer to read stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	
	// Goroutine to read stdout
	stdoutChan := make(chan string)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ms.stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("Error reading stdout: %v", err)
				}
				close(stdoutChan)
				return
			}
			stdoutBuf.Write(buf[:n])
			if strings.Contains(stdoutBuf.String(), delimiter) {
				close(stdoutChan)
				return
			}
		}
	}()

	// Goroutine to read stderr (optional, but good for capturing errors)
	// For simplicity in this step, we'll assume stderr does not contain the delimiter
	// and we read it after the command execution might have signaled completion via stdout.
	// A more robust solution would handle stderr more carefully, possibly also looking for a delimiter or using select.
	
	// Wait for stdout to finish (delimiter received)
	<-stdoutChan

	// Attempt to read from stderr. This is a simplified approach.
	// A more robust implementation would read stderr concurrently or use non-blocking reads.
	stderrBytes, err := io.ReadAll(ms.stderr)
	if err != nil && err != io.EOF { // EOF is expected if stderr was empty or already closed
		log.Printf("Error reading stderr: %v", err)
		// Decide if this should be a critical error, for now, we log and continue
	}
	stderrBuf.Write(stderrBytes)


	// Process output to remove delimiter
	stdoutStr := strings.Replace(stdoutBuf.String(), delimiter+"\n", "", -1)
	// Also remove the echo command itself from the output if it appears
	stdoutStr = strings.Replace(stdoutStr, "echo \""+delimiter+"\"\n", "", -1)
	// Trim any leading/trailing newlines or spaces that might have been added
	stdoutStr = strings.TrimSpace(stdoutStr)


	return stdoutStr, stderrBuf.String(), nil
}

// Stop terminates the shell process.
func (ms *ManagedShell) Stop() error {
	if ms.cmd == nil || ms.cmd.Process == nil {
		log.Println("Shell process not started or already stopped.")
		return nil // Or an error indicating it's not running
	}

	// Close stdin to signal the shell that no more commands are coming.
	if err := ms.stdin.Close(); err != nil {
		// Log error but attempt to continue stopping the process
		log.Printf("Failed to close stdin: %v", err)
	}

	// Kill the process
	if err := ms.cmd.Process.Kill(); err != nil {
		log.Printf("Failed to kill process: %v. Attempting to wait...", err)
		// Even if kill fails, try to Wait, as Wait releases resources.
	}

	// Wait for the process to exit and release resources.
	// This is important to prevent zombie processes.
	err := ms.cmd.Wait()
	// The error from Wait can be complex.
	// If Kill() was successful, Wait() will likely return an error like "signal: killed".
	// This is expected. If Kill() failed, Wait() might provide more info or also fail.
	if err != nil {
		// Check if the error is "exit status -1" or "signal: killed", which is expected after Kill()
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Process was killed or exited with an error code.
			// This is often an expected outcome of Stop().
			log.Printf("Shell process stopped. Exit status: %s", exitErr.Error())
		} else {
			// Other type of error during Wait.
			return fmt.Errorf("failed to wait for shell process to exit: %w", err)
		}
	} else {
		log.Println("Shell process stopped successfully.")
	}
	
	// Nil out fields to indicate the shell is stopped
	ms.stdin = nil
	ms.stdout = nil
	ms.stderr = nil
	ms.cmd = nil

	return nil
}
