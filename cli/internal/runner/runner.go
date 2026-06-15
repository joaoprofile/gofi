// Package runner executes the test tasks declared in .gofi.yaml. Each task
// is a shell command; tasks may declare other tasks as dependencies via
// `needs:`. Pre/post hooks run around the chain.
package runner

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

// Runner executes test tasks against a TestSection.
type Runner struct {
	Cfg    *config.TestSection
	Cwd    string    // working directory for shell exec; "" = current
	Stdout io.Writer // stdout for the executed processes
	Stderr io.Writer // stderr for the executed processes
	Stdin  io.Reader // stdin for the executed processes
}

// Run executes the named task after running pre-hooks, resolving dependencies
// and running post-hooks. extraArgs are appended to the command of the *named*
// task (not its dependencies). Post-hooks always run, even if the task or a
// dependency fails.
//
// Returns the first error encountered in the task chain. Hook failures (pre)
// abort the chain; post-hook errors are reported on stderr but do not override
// the task error.
func (r *Runner) Run(taskName string, extraArgs []string) error {
	if r.Cfg == nil {
		return errors.New("nil test config")
	}
	if _, ok := r.Cfg.Tasks[taskName]; !ok {
		return fmt.Errorf("unknown task %q (available: %s)", taskName, strings.Join(r.taskNames(), ", "))
	}

	order, err := topoOrder(r.Cfg.Tasks, taskName)
	if err != nil {
		return err
	}

	for _, h := range r.Cfg.Hooks.Pre {
		if err := r.execShell(h, nil); err != nil {
			return fmt.Errorf("pre hook %q failed: %w", h, err)
		}
	}

	var taskErr error
	for _, name := range order {
		task := r.Cfg.Tasks[name]
		var extras []string
		if name == taskName {
			extras = extraArgs
		}
		if err := r.execShell(task.Run, extras); err != nil {
			taskErr = fmt.Errorf("task %q failed: %w", name, err)
			break
		}
	}

	for _, h := range r.Cfg.Hooks.Post {
		if err := r.execShell(h, nil); err != nil {
			fmt.Fprintf(r.Stderr, "post hook %q failed: %v\n", h, err)
		}
	}

	return taskErr
}

// List returns the task names sorted in alphabetical order.
func (r *Runner) List() []TaskInfo {
	out := make([]TaskInfo, 0, len(r.Cfg.Tasks))
	for name, t := range r.Cfg.Tasks {
		out = append(out, TaskInfo{Name: name, Desc: t.Desc, Needs: t.Needs})
	}
	return out
}

// TaskInfo is the user-facing summary returned by Runner.List.
type TaskInfo struct {
	Name  string
	Desc  string
	Needs []string
}

func (r *Runner) taskNames() []string {
	out := make([]string, 0, len(r.Cfg.Tasks))
	for n := range r.Cfg.Tasks {
		out = append(out, n)
	}
	return out
}

// execShell runs cmd as a shell command. extraArgs are appended (shell-escaped)
// to the command before exec. The shell is `sh -c` on POSIX, `cmd /c` on Windows.
func (r *Runner) execShell(cmdStr string, extraArgs []string) error {
	full := cmdStr
	if len(extraArgs) > 0 {
		full = cmdStr + " " + escapeArgs(extraArgs)
	}
	shell, shellArgs := shellInvocation(full)
	c := exec.Command(shell, shellArgs...)
	c.Dir = r.Cwd
	c.Stdout = r.Stdout
	c.Stderr = r.Stderr
	c.Stdin = r.Stdin
	return c.Run()
}

func shellInvocation(cmd string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", cmd}
	}
	return "sh", []string{"-c", cmd}
}

func escapeArgs(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = shellQuote(a)
	}
	return strings.Join(out, " ")
}

// shellQuote wraps s in single quotes for POSIX shells, escaping any embedded
// single quote. On Windows we use double quotes with backslash-escaped quotes.
func shellQuote(s string) string {
	if runtime.GOOS == "windows" {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return "'" + strings.ReplaceAll(s, `'`, `'\''`) + "'"
}

// topoOrder returns task names in dependency order (deps first, root last)
// starting from start. Returns an error if a cycle is detected or if a
// referenced dependency is missing.
func topoOrder(tasks map[string]config.TestTask, start string) ([]string, error) {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var order []string
	var path []string

	var visit func(name string) error
	visit = func(name string) error {
		if color[name] == gray {
			cycle := append(path, name)
			return fmt.Errorf("cycle detected: %s", strings.Join(cycle, " -> "))
		}
		if color[name] == black {
			return nil
		}
		task, ok := tasks[name]
		if !ok {
			return fmt.Errorf("unknown task %q referenced in needs", name)
		}
		color[name] = gray
		path = append(path, name)
		for _, dep := range task.Needs {
			if err := visit(dep); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		color[name] = black
		order = append(order, name)
		return nil
	}
	if err := visit(start); err != nil {
		return nil, err
	}
	return order, nil
}
