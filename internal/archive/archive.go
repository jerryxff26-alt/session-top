package archive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

// DefaultBinaryName is the default executable name for the Codex CLI.
const DefaultBinaryName = "codex"

// DefaultMaxOutputBytes is the default buffer limit for captured stdout and stderr.
const DefaultMaxOutputBytes = 64 * 1024

var (
	// ErrEmptySessionID is returned when an empty or whitespace-only session ID is provided.
	ErrEmptySessionID = errors.New("archive: session ID cannot be empty")

	// ErrBinaryNotFound is returned when the codex executable is not found.
	ErrBinaryNotFound = errors.New("archive: codex binary not found")
)

var (
	bearerSecretRe = regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9_\-\.]+`)
	namedSecretRe  = regexp.MustCompile(`(?i)((?:api[_-]?key|password|secret|token)\s*[=:]\s*)[^\s,"'}]+`)
)

func sanitizeOutput(s string) string {
	s = bearerSecretRe.ReplaceAllString(s, "${1}[REDACTED]")
	s = namedSecretRe.ReplaceAllString(s, "${1}[REDACTED]")
	return s
}

// Result holds the outputs from a successful archive command execution.
type Result struct {
	SessionID string
	Stdout    string
	Stderr    string
}

// ArchiveError captures failure details of an archive operation.
// It explicitly includes session id and exit information while avoiding environment leakage.
type ArchiveError struct {
	SessionID string
	ExitCode  int
	Err       error
	Stdout    string
	Stderr    string
}

func (e *ArchiveError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "archive failed for session %q", e.SessionID)
	if e.ExitCode >= 0 {
		fmt.Fprintf(&sb, " (exit code %d)", e.ExitCode)
	}
	if e.Err != nil {
		fmt.Fprintf(&sb, ": %v", e.Err)
	}
	stderrSnippet := strings.TrimSpace(e.Stderr)
	if stderrSnippet != "" {
		stderrSnippet = sanitizeOutput(stderrSnippet)
		if len(stderrSnippet) > 512 {
			stderrSnippet = stderrSnippet[:512] + "..."
		}
		fmt.Fprintf(&sb, ": %s", stderrSnippet)
	}
	return sb.String()
}

// Unwrap returns the underlying error.
func (e *ArchiveError) Unwrap() error {
	return e.Err
}

// Runner defines the injectable interface for archiving sessions.
type Runner interface {
	Archive(ctx context.Context, sessionID string) (*Result, error)
}

// LookPathFunc resolves an executable name to an executable path.
type LookPathFunc func(file string) (string, error)

// CommandContextFunc creates an *exec.Cmd using context.
type CommandContextFunc func(ctx context.Context, name string, args ...string) *exec.Cmd

// ExecRunner implements Runner using system command execution.
type ExecRunner struct {
	binaryName     string
	lookPath       LookPathFunc
	commandContext CommandContextFunc
	maxOutputBytes int
}

// Option configures an ExecRunner.
type Option func(*ExecRunner)

// WithBinaryName sets the binary name to resolve (default: "codex").
func WithBinaryName(name string) Option {
	return func(r *ExecRunner) {
		r.binaryName = name
	}
}

// WithLookPath sets the path resolver function (default: exec.LookPath).
func WithLookPath(lp LookPathFunc) Option {
	return func(r *ExecRunner) {
		r.lookPath = lp
	}
}

// WithCommandContext sets the command context factory (default: exec.CommandContext).
func WithCommandContext(cc CommandContextFunc) Option {
	return func(r *ExecRunner) {
		r.commandContext = cc
	}
}

// WithMaxOutputBytes sets the maximum buffer size for captured stdout and stderr.
func WithMaxOutputBytes(n int) Option {
	return func(r *ExecRunner) {
		r.maxOutputBytes = n
	}
}

// New creates an ExecRunner configured with the given options.
func New(opts ...Option) *ExecRunner {
	r := &ExecRunner{
		binaryName:     DefaultBinaryName,
		lookPath:       exec.LookPath,
		commandContext: exec.CommandContext,
		maxOutputBytes: DefaultMaxOutputBytes,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

// DefaultRunner is the default package-level runner instance.
var DefaultRunner = New()

// Archive executes archiving for the given session ID using DefaultRunner.
func Archive(ctx context.Context, sessionID string) (*Result, error) {
	return DefaultRunner.Archive(ctx, sessionID)
}

// Archive runs `codex archive <sessionID>` safely without executing shell or leaking env.
func (r *ExecRunner) Archive(ctx context.Context, sessionID string) (*Result, error) {
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return nil, &ArchiveError{
			SessionID: sessionID,
			ExitCode:  -1,
			Err:       ErrEmptySessionID,
		}
	}

	lp := r.lookPath
	if lp == nil {
		lp = exec.LookPath
	}
	cc := r.commandContext
	if cc == nil {
		cc = exec.CommandContext
	}
	binName := r.binaryName
	if binName == "" {
		binName = DefaultBinaryName
	}
	maxBytes := r.maxOutputBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxOutputBytes
	}

	binary, err := lp(binName)
	if err != nil {
		return nil, &ArchiveError{
			SessionID: trimmedID,
			ExitCode:  -1,
			Err:       fmt.Errorf("%w: %v", ErrBinaryNotFound, err),
		}
	}

	cmd := cc(ctx, binary, "archive", trimmedID)

	stdoutBuf := newBoundedBuffer(maxBytes)
	stderrBuf := newBoundedBuffer(maxBytes)
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	err = cmd.Run()
	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if err != nil {
		exitCode := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return nil, &ArchiveError{
			SessionID: trimmedID,
			ExitCode:  exitCode,
			Err:       err,
			Stdout:    stdoutStr,
			Stderr:    stderrStr,
		}
	}

	return &Result{
		SessionID: trimmedID,
		Stdout:    stdoutStr,
		Stderr:    stderrStr,
	}, nil
}

type boundedBuffer struct {
	mu    sync.Mutex
	limit int
	buf   bytes.Buffer
}

func newBoundedBuffer(limit int) *boundedBuffer {
	return &boundedBuffer{limit: limit}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		toWrite := p
		if len(toWrite) > remaining {
			toWrite = toWrite[:remaining]
		}
		_, _ = b.buf.Write(toWrite)
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
