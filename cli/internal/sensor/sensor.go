// Package sensor runs the deterministic checks of the loop — build, static
// analysis, tests — and reads what each tool reports into findings with a
// place: file, line, what is wrong (decision 0002, D6). The run decides by
// them; a model never does. The same checks answer the agent mid-turn
// (MCP verify), but only the run's, after the turn, count.
package sensor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/pack"
)

// Timeout bounds one stage: a test suite that hangs must not hold the run.
var Timeout = 20 * time.Minute

// maxFindings is how many findings a stage keeps: the first ones are what a
// fix starts from, and a turn has no room for a thousand.
const maxFindings = 30

// Runner runs a project's sensor stages.
type Runner struct {
	Root string
	// Commands are each stage's, from the pack and loop.sensors; a stage
	// with none is recorded as not checked — never as passed.
	Commands map[string]pack.Command
	// Env is added to the commands' environment.
	Env []string
}

// Run runs the stages in order and stops at the first that fails.
func (r Runner) Run(ctx context.Context, stages []string) loop.SensorResult {
	var res loop.SensorResult
	var notes []string
	for _, stage := range stages {
		c, ok := r.Commands[stage]
		if !ok || strings.TrimSpace(c.Run) == "" {
			notes = append(notes, stage+": não verificado — sem comando no pack nem em loop.sensors")
			continue
		}
		findings, output, failed := r.stage(ctx, stage, c)
		if failed {
			res.Failed = []string{stage}
			res.Findings = findings
			res.Output = strings.Join(append(notes, stage+": "+c.Run+"\n"+output), "\n")
			return res
		}
	}
	res.Output = strings.Join(notes, "\n")
	return res
}

// stage runs one command and reads its report. A stage fails when the
// command does, and when its report holds a finding that fails it — a
// skipped test — even if the command exits clean.
func (r Runner) stage(ctx context.Context, stage string, c pack.Command) ([]loop.SensorFinding, string, bool) {
	sctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	shell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd", "/C"
	}
	cmd := exec.CommandContext(sctx, shell, flag, c.Run)
	cmd.Dir = r.Root
	cmd.Env = append(os.Environ(), r.Env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	report := stdout.Bytes()
	if c.Report != "" {
		if b, rerr := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(c.Report))); rerr == nil {
			report = b
		}
	}
	findings := Read(stage, c.Format, report, stderr.Bytes(), r.srcOf(c))
	if sctx.Err() == context.DeadlineExceeded {
		findings = append(findings, loop.SensorFinding{Stage: stage, Severity: "blocker", Message: fmt.Sprintf("o estágio passou de %s e foi interrompido", Timeout)})
	}
	failed := err != nil
	for _, f := range findings {
		if f.Severity == "blocker" {
			failed = true
		}
	}
	if failed && len(findings) == 0 {
		// The tool failed and said nothing a parser could place: its output
		// is the finding.
		findings = []loop.SensorFinding{{Stage: stage, Severity: "blocker", Message: tail(stderr.String()+stdout.String(), 1500)}}
	}
	output := tail(stdout.String()+"\n"+stderr.String(), 4000)
	if c.Format == pack.FormatGoTestJSON {
		output = tail(stderr.String(), 4000) // the JSON is read into findings
	}
	return cap30(findings), output, failed
}

// srcOf is the folder a command runs its tool in — what its file:line
// positions are relative to — from `go -C <dir>` or `--prefix <dir>`.
func (r Runner) srcOf(c pack.Command) string {
	f := strings.Fields(c.Run)
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "-C" || f[i] == "--prefix" {
			return f[i+1]
		}
	}
	return ""
}

// Read reads a stage's report in its format into findings, positions made
// relative to the project root.
func Read(stage, format string, stdout, stderr []byte, src string) []loop.SensorFinding {
	switch format {
	case pack.FormatLines:
		return lines(stage, append(append([]byte{}, stdout...), stderr...), src)
	case pack.FormatGoTestJSON:
		return append(goTestJSON(stage, stdout, src), lines(stage, stderr, src)...)
	case pack.FormatSARIF:
		return sarif(stage, stdout)
	case pack.FormatJUnit:
		return append(junit(stage, stdout, src), lines(stage, stderr, src)...)
	}
	return nil
}

// The shapes a compiler places an error in: file:line[:col]: message — Go,
// Rust, gcc, most linters —; file(line,col): message — tsc —; and
// [ERROR] file:[line,col] message — javac through Maven.
var (
	reLine   = regexp.MustCompile(`^\s*([^\s:][^:]*\.[A-Za-z0-9]+):(\d+)(?::\d+)?:\s*(.+)$`)
	reParens = regexp.MustCompile(`^\s*([^\s(][^(]*\.[A-Za-z0-9]+)\((\d+),\d+\):\s*(.+)$`)
	reMaven  = regexp.MustCompile(`^\[ERROR\]\s+(\S+\.[A-Za-z0-9]+):\[(\d+),\d+\]\s*(.+)$`)
)

// lines reads the shapes compilers print an error in.
func lines(stage string, out []byte, src string) []loop.SensorFinding {
	var res []loop.SensorFinding
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		m := reLine.FindStringSubmatch(sc.Text())
		if m == nil {
			m = reParens.FindStringSubmatch(sc.Text())
		}
		if m == nil {
			m = reMaven.FindStringSubmatch(sc.Text())
		}
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		f := loop.SensorFinding{Stage: stage, File: within(src, m[1]), Line: n, Severity: "blocker", Message: strings.TrimSpace(m[3])}
		key := fmt.Sprintf("%s:%d:%s", f.File, f.Line, f.Message)
		if !seen[key] {
			seen[key] = true
			res = append(res, f)
		}
	}
	return res
}

// testEvent is one line of go test -json.
type testEvent struct {
	Action  string
	Package string
	Test    string
	Output  string
}

// goTestJSON reads which tests failed, with what they printed, and which
// were skipped. A skipped test is a blocker: it is a test that proves
// nothing, kept green.
func goTestJSON(stage string, out []byte, src string) []loop.SensorFinding {
	outputs := map[string]*strings.Builder{}
	var res []loop.SensorFinding
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var e testEvent
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		key := e.Package + " " + e.Test
		switch e.Action {
		case "output":
			b := outputs[key]
			if b == nil {
				b = &strings.Builder{}
				outputs[key] = b
			}
			b.WriteString(e.Output)
		case "fail":
			if e.Test == "" {
				continue // the package fails because a test did; that one is reported
			}
			msg := "falhou"
			if b := outputs[key]; b != nil {
				msg += ":\n" + tail(b.String(), 1200)
			}
			res = append(res, loop.SensorFinding{Stage: stage, File: testFile(e.Package, outputs[key], src), Rule: e.Test, Severity: "blocker",
				Message: e.Test + " (" + e.Package + ") " + msg})
		case "skip":
			if e.Test == "" {
				continue // a package with no test files
			}
			res = append(res, loop.SensorFinding{Stage: stage, File: testFile(e.Package, outputs[key], src), Rule: e.Test, Severity: "blocker",
				Message: e.Test + " (" + e.Package + ") foi pulado: um teste pulado não prova nada — faça-o rodar ou remova-o com o requisito"})
		}
	}
	return res
}

var reTestPos = regexp.MustCompile(`([A-Za-z0-9_./-]+_test\.go):\d+`)

// testFile is the test's file, from the position its output names.
func testFile(pkg string, out *strings.Builder, src string) string {
	if out == nil {
		return ""
	}
	// go test prints a test's position by file name alone, relative to its
	// package: only a path is placed, never a guess.
	if m := reTestPos.FindStringSubmatch(out.String()); m != nil && strings.Contains(m[1], "/") {
		return within(src, m[1])
	}
	return ""
}

// junit reads JUnit XML — one suite or many —: each test case that failed
// or erred, with its message, and each one skipped, which proves nothing.
func junit(stage string, out []byte, src string) []loop.SensorFinding {
	type outcome struct {
		Message string `xml:"message,attr"`
		Text    string `xml:",chardata"`
	}
	type testCase struct {
		Name      string    `xml:"name,attr"`
		Classname string    `xml:"classname,attr"`
		File      string    `xml:"file,attr"`
		Line      int       `xml:"line,attr"`
		Failure   *outcome  `xml:"failure"`
		Error     *outcome  `xml:"error"`
		Skipped   *struct{} `xml:"skipped"`
	}
	type suite struct {
		File   string     `xml:"file,attr"`
		Cases  []testCase `xml:"testcase"`
		Suites []suite    `xml:"testsuite"`
	}
	var doc struct {
		suite
		XMLName xml.Name
	}
	if xml.Unmarshal(out, &doc) != nil {
		return nil
	}
	var res []loop.SensorFinding
	var walk func(s suite)
	walk = func(s suite) {
		for _, c := range s.Cases {
			file := c.File
			if file == "" {
				file = s.File
			}
			if file != "" {
				file = within(src, file)
			}
			name := strings.TrimSpace(strings.Trim(c.Classname+" "+c.Name, " "))
			switch {
			case c.Failure != nil || c.Error != nil:
				o := c.Failure
				if o == nil {
					o = c.Error
				}
				res = append(res, loop.SensorFinding{Stage: stage, File: file, Line: c.Line, Rule: c.Name, Severity: "blocker",
					Message: name + " falhou: " + tail(strings.TrimSpace(o.Message+"\n"+o.Text), 1200)})
			case c.Skipped != nil:
				res = append(res, loop.SensorFinding{Stage: stage, File: file, Line: c.Line, Rule: c.Name, Severity: "blocker",
					Message: name + " foi pulado: um teste pulado não prova nada — faça-o rodar ou remova-o com o requisito"})
			}
		}
		for _, child := range s.Suites {
			walk(child)
		}
	}
	walk(doc.suite)
	return res
}

// sarif reads SARIF 2.1: each result with its rule, level and first place.
func sarif(stage string, out []byte) []loop.SensorFinding {
	var doc struct {
		Runs []struct {
			Results []struct {
				RuleID  string `json:"ruleId"`
				Level   string `json:"level"`
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
				Locations []struct {
					Physical struct {
						Artifact struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
					Logical []struct {
						FullyQualifiedName string `json:"fullyQualifiedName"`
					} `json:"logicalLocations"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if json.Unmarshal(out, &doc) != nil {
		return nil
	}
	var res []loop.SensorFinding
	for _, run := range doc.Runs {
		for _, r := range run.Results {
			f := loop.SensorFinding{Stage: stage, Rule: r.RuleID, Message: r.Message.Text, Severity: "major"}
			if r.Level == "error" {
				f.Severity = "blocker"
			}
			if len(r.Locations) > 0 {
				f.File = strings.TrimPrefix(r.Locations[0].Physical.Artifact.URI, "file://")
				f.Line = r.Locations[0].Physical.Region.StartLine
				if l := r.Locations[0].Logical; len(l) > 0 {
					f.Symbol = l[0].FullyQualifiedName
				}
			}
			res = append(res, f)
		}
	}
	return res
}

// within makes a tool's path relative to the project root: tools run in the
// language's folder print paths relative to it.
func within(src, p string) string {
	p = filepath.ToSlash(strings.TrimPrefix(p, "./"))
	if src == "" || src == "." || filepath.IsAbs(p) || strings.HasPrefix(p, src+"/") {
		return p
	}
	return filepath.ToSlash(filepath.Join(src, p))
}

func cap30(f []loop.SensorFinding) []loop.SensorFinding {
	sort.SliceStable(f, func(i, j int) bool { return f[i].Severity == "blocker" && f[j].Severity != "blocker" })
	if len(f) > maxFindings {
		return f[:maxFindings]
	}
	return f
}

// tail keeps the end of a long output, where a tool says what broke.
func tail(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return "…\n" + s[len(s)-max:]
}
