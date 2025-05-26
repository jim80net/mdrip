package shell

import (
	"os/exec"
	"strings"
	"testing"
	"time" // For potential timeouts or delays if needed
)

// Helper function to get a valid shell path.
// Prefers /bin/bash, falls back to /bin/sh if bash is not found.
func getValidShellPath(t *testing.T) string {
	bashPath, err := exec.LookPath("/bin/bash")
	if err == nil {
		return bashPath
	}
	t.Log("'/bin/bash' not found, trying '/bin/sh'")
	shPath, err := exec.LookPath("/bin/sh")
	if err == nil {
		return shPath
	}
	t.Fatalf("Neither '/bin/bash' nor '/bin/sh' found in PATH. Cannot run shell tests.")
	return "" // Should not reach here
}

func TestNewManagedShell(t *testing.T) {
	validShell := getValidShellPath(t)

	t.Run("SuccessfulCreation", func(t *testing.T) {
		ms, err := NewManagedShell(validShell)
		if err != nil {
			t.Fatalf("NewManagedShell with valid path failed: %v", err)
		}
		if ms == nil {
			t.Fatal("NewManagedShell returned nil ManagedShell for valid path")
		}
		if ms.cmd == nil {
			t.Fatal("ManagedShell.cmd is nil after successful creation")
		}
		if ms.cmd.Path != validShell {
			t.Errorf("ManagedShell.cmd.Path = %q, want %q", ms.cmd.Path, validShell)
		}
	})

	t.Run("EmptyShellPath", func(t *testing.T) {
		_, err := NewManagedShell("")
		if err == nil {
			t.Fatal("NewManagedShell with empty path did not return an error")
		}
	})

	t.Run("InvalidShellPath", func(t *testing.T) {
		invalidPath := "/nonexistent/shell/path/guaranteed/to/fail"
		ms, err := NewManagedShell(invalidPath)
		if err != nil {
			// This is tricky. NewManagedShell itself might not error if exec.LookPath isn't called by it.
			// The error might only occur on Start().
			// For now, we assume NewManagedShell only sets up the command string.
			// Let's check if ms.cmd.Path is what we set.
			if ms == nil {
				t.Fatalf("NewManagedShell with invalid path returned nil ms, but expected it to set cmd.Path")
			}
			if ms.cmd.Path != invalidPath {
				t.Errorf("ms.cmd.Path = %q, want %q for invalid path", ms.cmd.Path, invalidPath)
			}
			// The actual failure for an invalid path is better tested in Start()
		} else if ms.cmd.Path != invalidPath {
			t.Errorf("ms.cmd.Path = %q, want %q for invalid path", ms.cmd.Path, invalidPath)
		}
	})
}

func TestManagedShell_StartStop(t *testing.T) {
	validShell := getValidShellPath(t)
	ms, err := NewManagedShell(validShell)
	if err != nil {
		t.Fatalf("Failed to create ManagedShell: %v", err)
	}

	err = ms.Start()
	if err != nil {
		t.Fatalf("ms.Start() failed: %v", err)
	}
	if ms.cmd == nil || ms.cmd.Process == nil {
		t.Fatal("ms.cmd.Process is nil after Start()")
	}
	if ms.stdin == nil {
		t.Fatal("ms.stdin is nil after Start()")
	}
	if ms.stdout == nil {
		t.Fatal("ms.stdout is nil after Start()")
	}
	if ms.stderr == nil {
		t.Fatal("ms.stderr is nil after Start()")
	}

	// Check if process is running (platform-dependent check, so be careful)
	// A simple check is to see if Process.Pid is > 0
	if ms.cmd.Process.Pid <= 0 {
		t.Errorf("Expected process to be running (PID > 0), got PID %d", ms.cmd.Process.Pid)
	}

	err = ms.Stop()
	if err != nil {
		t.Fatalf("ms.Stop() failed: %v", err)
	}
	if ms.cmd != nil {
		// After Stop, we've decided to nil out ms.cmd in the implementation
		t.Error("ms.cmd is not nil after Stop()")
	}

	// Try stopping again (should be idempotent or return a specific error)
	err = ms.Stop()
	if err != nil {
		// Depending on implementation, this might error or just log.
		// Current implementation logs "Shell process not started or already stopped." and returns nil.
		t.Logf("Second Stop() call returned: %v (this might be expected)", err)
	}
}

func TestManagedShell_Start_InvalidPath(t *testing.T) {
	invalidPath := "/nonexistent/shell/path/guaranteed/to/fail/on/start"
	ms, _ := NewManagedShell(invalidPath) // Error check for New is in TestNewManagedShell

	err := ms.Start()
	if err == nil {
		t.Fatal("ms.Start() with an invalid shell path did not return an error")
		// If it somehow started, try to stop it
		_ = ms.Stop()
	}
	t.Logf("Start() with invalid path correctly failed: %v", err)
}

func TestManagedShell_Execute_SimpleCommand(t *testing.T) {
	validShell := getValidShellPath(t)
	ms, err := NewManagedShell(validShell)
	if err != nil {
		t.Fatalf("Failed to create ManagedShell: %v", err)
	}
	if err := ms.Start(); err != nil {
		t.Fatalf("Failed to start ManagedShell: %v", err)
	}
	defer ms.Stop()

	command := `echo "hello world"`
	stdout, stderr, err := ms.Execute(command)

	if err != nil {
		t.Errorf("Execute(%q) returned error: %v", command, err)
	}
	// The Execute method implementation trims space, so we expect "hello world" not "hello world\n"
	expectedStdout := "hello world"
	if stdout != expectedStdout {
		t.Errorf("Execute(%q) stdout = %q, want %q", command, stdout, expectedStdout)
	}
	if stderr != "" {
		t.Errorf("Execute(%q) stderr = %q, want \"\"", command, stderr)
	}
}

func TestManagedShell_Execute_StderrOutput(t *testing.T) {
	validShell := getValidShellPath(t)
	ms, err := NewManagedShell(validShell)
	if err != nil {
		t.Fatalf("Failed to create ManagedShell: %v", err)
	}
	if err := ms.Start(); err != nil {
		t.Fatalf("Failed to start ManagedShell: %v", err)
	}
	defer ms.Stop()

	command := `>&2 echo "error message"`
	stdout, stderr, err := ms.Execute(command)

	if err != nil {
		t.Errorf("Execute(%q) returned error: %v", command, err)
	}
	// stdout might contain the delimiter echo, but our Execute function should clean it.
	// However, the command itself (>&2 echo "error message") produces no stdout.
	if stdout != "" {
		t.Errorf("Execute(%q) stdout = %q, want \"\"", command, stdout)
	}
	// Stderr output from `echo` includes a newline.
	// The current implementation of Execute does not trim stderr.
	expectedStderr := "error message\n"
	if stderr != expectedStderr {
		t.Errorf("Execute(%q) stderr = %q, want %q", command, stderr, expectedStderr)
	}
}

func TestManagedShell_Execute_CommandWithQuotesAndSpecialChars(t *testing.T) {
	validShell := getValidShellPath(t)
	ms, err := NewManagedShell(validShell)
	if err != nil {
		t.Fatalf("Failed to create ManagedShell: %v", err)
	}
	if err := ms.Start(); err != nil {
		t.Fatalf("Failed to start ManagedShell: %v", err)
	}
	defer ms.Stop()

	command := `echo 'hello  "world" with $var and *'`
	// Note: variable expansion ($var) and globbing (*) might behave differently
	// depending on the shell and how the command is processed.
	// For a simple echo, these are typically treated as literals if single-quoted.
	stdout, stderr, err := ms.Execute(command)

	if err != nil {
		t.Errorf("Execute(%q) returned error: %v", command, err)
	}
	// Execute trims space, so no trailing newline from echo.
	expectedStdout := `hello  "world" with $var and *`
	if stdout != expectedStdout {
		t.Errorf("Execute(%q) stdout = %q, want %q", command, stdout, expectedStdout)
	}
	if stderr != "" {
		t.Errorf("Execute(%q) stderr = %q, want \"\"", command, stderr)
	}
}

func TestManagedShell_Execute_MultipleCommandsSequentially(t *testing.T) {
	validShell := getValidShellPath(t)
	ms, err := NewManagedShell(validShell)
	if err != nil {
		t.Fatalf("Failed to create ManagedShell: %v", err)
	}
	if err := ms.Start(); err != nil {
		t.Fatalf("Failed to start ManagedShell: %v", err)
	}
	defer ms.Stop()

	// Command 1
	cmd1 := `echo "command1"`
	stdout1, stderr1, err1 := ms.Execute(cmd1)
	if err1 != nil {
		t.Errorf("Execute(%q) (1st) returned error: %v", cmd1, err1)
	}
	expectedStdout1 := "command1"
	if stdout1 != expectedStdout1 {
		t.Errorf("Execute(%q) (1st) stdout = %q, want %q", cmd1, stdout1, expectedStdout1)
	}
	if stderr1 != "" {
		t.Errorf("Execute(%q) (1st) stderr = %q, want \"\"", cmd1, stderr1)
	}

	// Command 2
	cmd2 := `echo "command2"`
	stdout2, stderr2, err2 := ms.Execute(cmd2)
	if err2 != nil {
		t.Errorf("Execute(%q) (2nd) returned error: %v", cmd2, err2)
	}
	expectedStdout2 := "command2"
	if stdout2 != expectedStdout2 {
		t.Errorf("Execute(%q) (2nd) stdout = %q, want %q", cmd2, stdout2, expectedStdout2)
	}
	if stderr2 != "" {
		t.Errorf("Execute(%q) (2nd) stderr = %q, want \"\"", cmd2, stderr2)
	}
}

func TestManagedShell_StopBeforeStart(t *testing.T) {
	validShell := getValidShellPath(t)
	ms, err := NewManagedShell(validShell)
	if err != nil {
		t.Fatalf("Failed to create ManagedShell: %v", err)
	}

	// Call Stop() before Start()
	// Based on current implementation, this should log and return nil.
	err = ms.Stop()
	if err != nil {
		t.Errorf("Stop() before Start() returned error: %v. Expected nil or specific gentle error.", err)
	}
	// Ensure internal state allows for a subsequent Start if desired (though not strictly tested here)
	if ms.cmd == nil { // Stop nils out cmd
		t.Log("Stop before Start correctly nilled out ms.cmd as per current Stop impl.")
	}
}


func TestManagedShell_Execute_AfterStop(t *testing.T) {
	validShell := getValidShellPath(t)
	ms, err := NewManagedShell(validShell)
	if err != nil {
		t.Fatalf("Failed to create ManagedShell: %v", err)
	}

	if err := ms.Start(); err != nil {
		t.Fatalf("Failed to start ManagedShell: %v", err)
	}

	if err := ms.Stop(); err != nil {
		t.Fatalf("Failed to stop ManagedShell: %v", err)
	}

	// Attempt to Execute after Stop
	command := `echo "too late"`
	_, _, err = ms.Execute(command)
	if err == nil {
		t.Fatal("Execute() after Stop() did not return an error")
	}
	// Check for a specific error message if your Execute function provides one
	expectedErrorMsg := "shell process not started"
	if !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("Execute() after Stop() error = %q, want error containing %q", err.Error(), expectedErrorMsg)
	}
}

func TestManagedShell_Execute_LongRunningCommand_AndStop(t *testing.T) {
	// This test is to ensure Stop can terminate a running command.
	validShell := getValidShellPath(t)
	ms, err := NewManagedShell(validShell)
	if err != nil {
		t.Fatalf("Failed to create ManagedShell: %v", err)
	}

	if err := ms.Start(); err != nil {
		t.Fatalf("Failed to start ManagedShell: %v", err)
	}

	// Execute a command that sleeps for a while
	// We don't wait for its full completion in the Execute call itself
	// because Execute for long-running commands without immediate output
	// termination (like our delimiter) is tricky.
	// The main point here is to test Stop's ability to kill.

	// Send the command. We don't use the typical Execute() here because
	// the delimiter might not be reached if we Stop it early.
	// Instead, we directly write to stdin and then try to stop.
	// This is a more direct test of Stop's interruption capability.

	sleepCmd := "sleep 10\n" // Sleep for 10 seconds
	go func() {
		if _, err := ms.stdin.Write([]byte(sleepCmd)); err != nil {
			// This goroutine might error if stdin is closed by Stop before Write completes.
			// We can't t.Error or t.Fatal from here directly.
			// Consider using a channel to report errors if this becomes an issue.
			// For now, we assume the write will likely succeed or the test will hang/timeout if Stop fails.
			t.Logf("Error writing sleep command to stdin (might be ok if Stop is fast): %v", err)
		}
	}()

	// Give a very short time for the command to start
	time.Sleep(100 * time.Millisecond)

	// Now, try to stop the shell. This should kill the sleep command.
	stopErr := ms.Stop()
	if stopErr != nil {
		t.Fatalf("Stop() failed to terminate a long-running command: %v", stopErr)
	}
	t.Log("Stop() called, presumably terminating the sleep command.")
	// If Stop hangs or fails to kill the process, the test might time out here or fail above.
	// The check ms.cmd.Wait() in Stop() is crucial.
}
