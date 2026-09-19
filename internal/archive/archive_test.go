package archive

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestArchive_EmptyID tests that empty and whitespace-only session IDs are rejected immediately.
func TestArchive_EmptyID(t *testing.T) {
	lookPathCalled := false
	cmdContextCalled := false

	runner := New(
		WithLookPath(func(file string) (string, error) {
			lookPathCalled = true
			return "/fake/codex", nil
		}),
		WithCommandContext(func(ctx context.Context, name string, args ...string) *exec.Cmd {
			cmdContextCalled = true
			return exec.CommandContext(ctx, "true")
		}),
	)

	testCases := []struct {
		name string
		id   string
	}{
		{name: "empty string", id: ""},
		{name: "spaces only", id: "   "},
		{name: "tabs and newlines", id: "\t\n  "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lookPathCalled = false
			cmdContextCalled = false

			res, err := runner.Archive(context.Background(), tc.id)
			if err == nil {
				t.Fatalf("expected error for empty session id, got nil")
			}
			if res != nil {
				t.Fatalf("expected nil result, got %+v", res)
			}
			if !errors.Is(err, ErrEmptySessionID) {
				t.Errorf("expected ErrEmptySessionID, got %v", err)
			}
			var arcErr *ArchiveError
			if !errors.As(err, &arcErr) {
				t.Errorf("expected *ArchiveError, got %T", err)
			} else {
				if arcErr.SessionID != tc.id {
					t.Errorf("expected SessionID %q, got %q", tc.id, arcErr.SessionID)
				}
				if arcErr.ExitCode != -1 {
					t.Errorf("expected ExitCode -1, got %d", arcErr.ExitCode)
				}
			}
			if lookPathCalled {
				t.Errorf("lookPath should not be called when session id is empty")
			}
			if cmdContextCalled {
				t.Errorf("commandContext should not be called when session id is empty")
			}
		})
	}
}

// TestArchive_MissingBinary tests handling when the codex executable is not found in PATH.
func TestArchive_MissingBinary(t *testing.T) {
	runner := New(
		WithLookPath(func(file string) (string, error) {
			return "", exec.ErrNotFound
		}),
	)

	sessionID := "session-missing-bin-123"
	res, err := runner.Archive(context.Background(), sessionID)
	if err == nil {
		t.Fatalf("expected error when binary missing, got nil")
	}
	if res != nil {
		t.Fatalf("expected nil result, got %+v", res)
	}

	if !errors.Is(err, ErrBinaryNotFound) {
		t.Errorf("expected error to wrap ErrBinaryNotFound, got %v", err)
	}

	var arcErr *ArchiveError
	if !errors.As(err, &arcErr) {
		t.Fatalf("expected error to be *ArchiveError, got %T", err)
	}
	if arcErr.SessionID != sessionID {
		t.Errorf("expected SessionID %q, got %q", sessionID, arcErr.SessionID)
	}
	if arcErr.ExitCode != -1 {
		t.Errorf("expected ExitCode -1, got %d", arcErr.ExitCode)
	}
	if !strings.Contains(err.Error(), sessionID) {
		t.Errorf("expected error string to contain session id %q: %s", sessionID, err.Error())
	}
}

// TestArchive_Success_InjectedCommandFactory tests successful archiving via injected command factory.
func TestArchive_Success_InjectedCommandFactory(t *testing.T) {
	sessionID := "01912345-6789-abcd-ef01-23456789abcd"
	var capturedName string
	var capturedArgs []string

	runner := New(
		WithLookPath(func(file string) (string, error) {
			if file != "codex" {
				t.Errorf("expected binary name 'codex', got %q", file)
			}
			return "/usr/local/bin/codex", nil
		}),
		WithCommandContext(func(ctx context.Context, name string, args ...string) *exec.Cmd {
			capturedName = name
			capturedArgs = args
			// Simulate a command that outputs to stdout and exits 0
			return exec.CommandContext(ctx, "sh", "-c", "echo 'Archived session successfully'; echo 'warning notice' >&2")
		}),
	)

	res, err := runner.Archive(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil result")
	}

	if capturedName != "/usr/local/bin/codex" {
		t.Errorf("expected binary name /usr/local/bin/codex, got %q", capturedName)
	}
	expectedArgs := []string{"archive", sessionID}
	if len(capturedArgs) != len(expectedArgs) || capturedArgs[0] != expectedArgs[0] || capturedArgs[1] != expectedArgs[1] {
		t.Errorf("expected args %v, got %v", expectedArgs, capturedArgs)
	}

	if res.SessionID != sessionID {
		t.Errorf("expected result session ID %q, got %q", sessionID, res.SessionID)
	}
	if !strings.Contains(res.Stdout, "Archived session successfully") {
		t.Errorf("expected stdout to contain success message, got %q", res.Stdout)
	}
	if !strings.Contains(res.Stderr, "warning notice") {
		t.Errorf("expected stderr to contain warning notice, got %q", res.Stderr)
	}
}

// TestArchive_Failure_InjectedCommandFactory tests command failure (non-zero exit) and error formatting.
func TestArchive_Failure_InjectedCommandFactory(t *testing.T) {
	sessionID := "session-failed-456"

	runner := New(
		WithLookPath(func(file string) (string, error) {
			return "/mock/codex", nil
		}),
		WithCommandContext(func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Simulate non-zero exit code 2 and an error message on stderr
			return exec.CommandContext(ctx, "sh", "-c", "echo 'session does not exist' >&2; exit 2")
		}),
	)

	res, err := runner.Archive(context.Background(), sessionID)
	if err == nil {
		t.Fatalf("expected error on command failure, got nil")
	}
	if res != nil {
		t.Fatalf("expected nil result on failure, got %+v", res)
	}

	var arcErr *ArchiveError
	if !errors.As(err, &arcErr) {
		t.Fatalf("expected *ArchiveError, got %T", err)
	}

	if arcErr.SessionID != sessionID {
		t.Errorf("expected SessionID %q, got %q", sessionID, arcErr.SessionID)
	}
	if arcErr.ExitCode != 2 {
		t.Errorf("expected ExitCode 2, got %d", arcErr.ExitCode)
	}
	if !strings.Contains(arcErr.Stderr, "session does not exist") {
		t.Errorf("expected stderr to contain 'session does not exist', got %q", arcErr.Stderr)
	}

	errStr := err.Error()
	if !strings.Contains(errStr, sessionID) {
		t.Errorf("error string should contain session ID %q: %s", sessionID, errStr)
	}
	if !strings.Contains(errStr, "exit code 2") {
		t.Errorf("error string should contain exit code: %s", errStr)
	}
	if !strings.Contains(errStr, "session does not exist") {
		t.Errorf("error string should contain stderr message: %s", errStr)
	}
}

// TestArchive_NoEnvironmentLeak ensures environment variables are not leaked into error messages.
func TestArchive_NoEnvironmentLeak(t *testing.T) {
	secretKey := "SUPER_SECRET_TOKEN_XYZ"
	secretVal := "secret-token-value-999"
	t.Setenv(secretKey, secretVal)

	sessionID := "session-leak-test"

	runner := New(
		WithLookPath(func(file string) (string, error) {
			return "/mock/codex", nil
		}),
		WithCommandContext(func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Subprocess outputs an error containing an auth token
			return exec.CommandContext(ctx, "sh", "-c", "echo 'Error: Bearer secret-bearer-abc-123 failed' >&2; exit 1")
		}),
	)

	_, err := runner.Archive(context.Background(), sessionID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errStr := err.Error()
	// Must not leak OS environment variable
	if strings.Contains(errStr, secretVal) || strings.Contains(errStr, secretKey) {
		t.Errorf("error string leaked environment variable: %s", errStr)
	}
	// Must have redacted bearer token in error snippet
	if strings.Contains(errStr, "secret-bearer-abc-123") {
		t.Errorf("error string leaked bearer token: %s", errStr)
	}
	if !strings.Contains(errStr, "[REDACTED]") {
		t.Errorf("expected redaction in error string, got %s", errStr)
	}
}

// TestArchive_BoundedOutput tests that excessive output is bounded to maxOutputBytes.
func TestArchive_BoundedOutput(t *testing.T) {
	maxBytes := 256
	sessionID := "session-big-output"

	runner := New(
		WithMaxOutputBytes(maxBytes),
		WithLookPath(func(file string) (string, error) {
			return "/mock/codex", nil
		}),
		WithCommandContext(func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Generate 1000 characters of stdout and stderr
			return exec.CommandContext(ctx, "sh", "-c", "python3 -c 'print(\"A\" * 1000); import sys; sys.stderr.write(\"B\" * 1000)'")
		}),
	)

	res, err := runner.Archive(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Stdout) > maxBytes {
		t.Errorf("expected stdout length <= %d, got %d", maxBytes, len(res.Stdout))
	}
	if len(res.Stderr) > maxBytes {
		t.Errorf("expected stderr length <= %d, got %d", maxBytes, len(res.Stderr))
	}
}

// TestArchive_WithFakeScript tests using a temporary fake executable on PATH.
func TestArchive_WithFakeScript(t *testing.T) {
	tmpDir := t.TempDir()
	fakeScriptPath := filepath.Join(tmpDir, "codex")

	scriptContent := `#!/bin/sh
if [ "$1" = "archive" ]; then
    if [ "$2" = "valid-session" ]; then
        echo "session valid-session successfully archived"
        exit 0
    else
        echo "error: session $2 not found" >&2
        exit 1
    fi
fi
echo "unsupported command" >&2
exit 127
`
	if err := os.WriteFile(fakeScriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write fake script: %v", err)
	}

	// Prepend tmpDir to PATH so exec.LookPath("codex") finds our fake script
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+":"+origPath)

	runner := New() // Uses default exec.LookPath and exec.CommandContext!

	// 1. Test success with fake script
	res, err := runner.Archive(context.Background(), "valid-session")
	if err != nil {
		t.Fatalf("expected success with fake script, got %v", err)
	}
	if !strings.Contains(res.Stdout, "session valid-session successfully archived") {
		t.Errorf("unexpected stdout: %q", res.Stdout)
	}

	// 2. Test failure with fake script
	_, err = runner.Archive(context.Background(), "invalid-session")
	if err == nil {
		t.Fatalf("expected error for invalid session, got nil")
	}
	var arcErr *ArchiveError
	if !errors.As(err, &arcErr) {
		t.Fatalf("expected *ArchiveError, got %T", err)
	}
	if arcErr.ExitCode != 1 {
		t.Errorf("expected ExitCode 1, got %d", arcErr.ExitCode)
	}
	if !strings.Contains(arcErr.Stderr, "error: session invalid-session not found") {
		t.Errorf("unexpected stderr: %q", arcErr.Stderr)
	}
}

// TestArchive_ContextCancellation tests error handling when context is cancelled.
func TestArchive_ContextCancellation(t *testing.T) {
	runner := New(
		WithLookPath(func(file string) (string, error) {
			return "/mock/codex", nil
		}),
		WithCommandContext(func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Sleep so context cancellation interrupts it
			return exec.CommandContext(ctx, "sleep", "10")
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := runner.Archive(ctx, "session-timeout")
	if err == nil {
		t.Fatal("expected error on context deadline, got nil")
	}
	var arcErr *ArchiveError
	if !errors.As(err, &arcErr) {
		t.Fatalf("expected *ArchiveError, got %T", err)
	}
	if arcErr.SessionID != "session-timeout" {
		t.Errorf("expected session id in error, got %s", arcErr.SessionID)
	}
}

// TestArchive_WithBinaryName tests overriding the binary name.
func TestArchive_WithBinaryName(t *testing.T) {
	lookedUp := ""
	runner := New(
		WithBinaryName("custom-codex"),
		WithLookPath(func(file string) (string, error) {
			lookedUp = file
			return "/custom/" + file, nil
		}),
		WithCommandContext(func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "true")
		}),
	)
	_, err := runner.Archive(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lookedUp != "custom-codex" {
		t.Errorf("expected binary looked up to be custom-codex, got %s", lookedUp)
	}
}

// TestArchive_PackageLevel tests the package-level Archive function.
func TestArchive_PackageLevel(t *testing.T) {
	_, err := Archive(context.Background(), "")
	if !errors.Is(err, ErrEmptySessionID) {
		t.Errorf("expected ErrEmptySessionID, got %v", err)
	}
}
