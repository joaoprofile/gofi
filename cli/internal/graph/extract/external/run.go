package external

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Spec locates one extractor.
type Spec struct {
	Language string   // "java", "rust", ...
	Path     string   // executable to run
	Args     []string // extra arguments inserted before the generated ones
}

// RunOptions controls one extractor invocation.
type RunOptions struct {
	Root    string        // repository root, passed as --root
	Deep    bool          // requests --mode deep; extractors may ignore it
	Timeout time.Duration // 0 means no deadline beyond the context
	Limits  Limits
}

// stderrTail keeps the last few KB an extractor wrote to stderr, so a failure
// can be explained without holding a whole build log in memory.
type stderrTail struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (t *stderrTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *stderrTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.buf))
}

// Run executes an extractor and decodes the graph it streams on stdout.
//
// The graph is decoded while the process is still running, so a large
// repository does not have to be buffered in full before parsing starts.
func Run(ctx context.Context, spec Spec, opt RunOptions) (*Result, error) {
	if spec.Path == "" {
		return nil, fmt.Errorf("extractor de %q sem caminho de executavel", spec.Language)
	}
	if opt.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opt.Timeout)
		defer cancel()
	}

	mode := "fast"
	if opt.Deep {
		mode = "deep"
	}
	args := append(append([]string{}, spec.Args...), "--root", opt.Root, "--mode", mode)

	cmd := exec.CommandContext(ctx, spec.Path, args...)
	cmd.Dir = opt.Root
	// Without WaitDelay a child that leaks a grandchild holding the pipe open
	// would hang gofi forever after the extractor itself exited.
	cmd.WaitDelay = 5 * time.Second

	tail := &stderrTail{max: 8 << 10}
	cmd.Stderr = tail

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		// A dead context fails Start with a generic error; saying the run was
		// cancelled is more useful than blaming the executable.
		if ctx.Err() != nil {
			return nil, runError(spec, err, "", ctx.Err())
		}
		return nil, fmt.Errorf("nao foi possivel executar o extractor de %s (%s): %w", spec.Language, spec.Path, err)
	}

	res, decodeErr := Decode(stdout, opt.Limits)
	// Drain whatever is left so the child is never blocked writing into a full
	// pipe while we wait for it to exit.
	if decodeErr != nil {
		_, _ = stdout.Read(make([]byte, 0))
	}
	waitErr := cmd.Wait()

	// A non-zero exit explains a decode failure better than the decode error
	// does, so it is reported first.
	if waitErr != nil {
		return nil, runError(spec, waitErr, tail.String(), ctx.Err())
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("saida do extractor de %s: %w%s", spec.Language, decodeErr, suffix(tail.String()))
	}
	res.Graph.Tool = firstNonEmpty(res.Graph.Tool, spec.Path)
	return res, nil
}

func runError(spec Spec, waitErr error, stderr string, ctxErr error) error {
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return fmt.Errorf("o extractor de %s estourou o tempo limite%s", spec.Language, suffix(stderr))
	}
	if errors.Is(ctxErr, context.Canceled) {
		return fmt.Errorf("o extractor de %s foi cancelado", spec.Language)
	}
	if exit, ok := errors.AsType[*exec.ExitError](waitErr); ok {
		return fmt.Errorf("o extractor de %s saiu com codigo %d%s", spec.Language, exit.ExitCode(), suffix(stderr))
	}
	return fmt.Errorf("o extractor de %s falhou: %w%s", spec.Language, waitErr, suffix(stderr))
}

func suffix(stderr string) string {
	if stderr == "" {
		return ""
	}
	return "\n--- stderr do extractor ---\n" + stderr
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
