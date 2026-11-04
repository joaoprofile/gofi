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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
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
	// Cache remembers the build, static and test that passed, by the tree
	// they passed on: the same command on the same tree is not run again —
	// the sense after a turn that ended on a green verify.
	Cache bool
}

// cacheable are the stages worth remembering: the ones that compile and
// run the project. The others are gofi's own, and quick.
var cacheable = map[string]bool{"build": true, "static": true, "test": true}

// Run runs the stages in order and stops at the first that fails.
func (r Runner) Run(ctx context.Context, stages []string) loop.SensorResult {
	var res loop.SensorResult
	var notes []string
	tree, passed := "", map[string]string{}
	if r.Cache {
		tree = r.treeKey(ctx)
		passed = r.loadPassed()
	}
	for _, stage := range stages {
		c, ok := r.Commands[stage]
		if !ok || strings.TrimSpace(c.Run) == "" {
			notes = append(notes, stage+": não verificado — sem comando no pack nem em loop.sensors")
			continue
		}
		key := ""
		if tree != "" && cacheable[stage] {
			key = hashOf(c.Run, c.Format, c.Report, tree)
			if passed[stage] == key {
				notes = append(notes, stage+": verde na mesma árvore — não rodou de novo")
				continue
			}
		}
		findings, output, failed := r.stage(ctx, stage, c)
		if key != "" {
			if failed {
				delete(passed, stage)
			} else {
				passed[stage] = key
			}
			r.savePassed(passed)
		}
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
	// A report an earlier run left never stands for this one.
	for _, p := range r.reports(c) {
		_ = os.Remove(p)
	}
	cmd := exec.CommandContext(sctx, shell, flag, c.Run)
	cmd.Dir = r.Root
	cmd.Env = append(os.Environ(), r.Env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var findings []loop.SensorFinding
	if c.Report == "" {
		findings = ReadWith(stage, c, stdout.Bytes(), stderr.Bytes(), r.srcOf(c))
	} else {
		findings = ReadWith(stage, c, nil, stderr.Bytes(), r.srcOf(c))
		for _, p := range r.reports(c) {
			if b, rerr := os.ReadFile(p); rerr == nil {
				findings = append(findings, ReadWith(stage, c, b, nil, r.srcOf(c))...)
			}
		}
	}
	// A tool that names files by their absolute path — ruff, javac — is
	// placed from the project root, as every other.
	if root, err := filepath.Abs(r.Root); err == nil {
		for i := range findings {
			if f := findings[i].File; filepath.IsAbs(f) {
				if rel, err := filepath.Rel(root, f); err == nil && !strings.HasPrefix(rel, "..") {
					findings[i].File = filepath.ToSlash(rel)
				}
			}
		}
	}
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

// reports are the report files of a command as they are now: the one it
// names, or every one its glob matches.
func (r Runner) reports(c pack.Command) []string {
	if c.Report == "" {
		return nil
	}
	p := filepath.Join(r.Root, filepath.FromSlash(c.Report))
	if !strings.ContainsAny(c.Report, "*?[") {
		return []string{p}
	}
	m, _ := filepath.Glob(p)
	return m
}

// srcOf is the folder a command runs its tool in — what its file:line
// positions are relative to — from `cd <dir> &&`, `go -C <dir>` or
// `--prefix <dir>`.
func (r Runner) srcOf(c pack.Command) string {
	f := strings.Fields(c.Run)
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "-C" || f[i] == "--prefix" || i == 0 && f[i] == "cd" {
			return f[i+1]
		}
	}
	return ""
}

// Read reads a stage's report in its format into findings, positions made
// relative to the project root.
func Read(stage, format string, stdout, stderr []byte, src string) []loop.SensorFinding {
	return ReadWith(stage, pack.Command{Format: format}, stdout, stderr, src)
}

// ReadWith is Read with what the command says of its report.
func ReadWith(stage string, c pack.Command, stdout, stderr []byte, src string) []loop.SensorFinding {
	switch c.Format {
	case pack.FormatLines:
		return lines(stage, append(append([]byte{}, stdout...), stderr...), src)
	case pack.FormatGoTestJSON:
		return append(goTestJSON(stage, stdout, src), lines(stage, stderr, src)...)
	case pack.FormatSARIF:
		return sarif(stage, stdout)
	case pack.FormatJUnit:
		return append(junit(stage, stdout, src, c.ClassFile), lines(stage, stderr, src)...)
	case pack.FormatCargoTest:
		return append(cargoTest(stage, append(append([]byte{}, stdout...), stderr...), src), lines(stage, stderr, src)...)
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
func junit(stage string, out []byte, src, classFile string) []loop.SensorFinding {
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
			} else if classFile != "" && c.Classname != "" {
				// The report names the class, not its file: the pack knows
				// where a class's file is, from the project root.
				file = strings.ReplaceAll(classFile, "{class}", strings.ReplaceAll(c.Classname, ".", "/"))
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

var (
	reCargoResult = regexp.MustCompile(`^test (\S+) \.\.\. (FAILED|ignored)`)
	reCargoPanic  = regexp.MustCompile(`^thread '([^']+)'(?: \(\d+\))? panicked at ([^:\s]+\.rs):(\d+):\d+:`)
)

// cargoTest reads libtest's output: each test that failed, placed where it
// panicked, with what it printed — and each one ignored, which proves
// nothing.
func cargoTest(stage string, out []byte, src string) []loop.SensorFinding {
	type place struct {
		file string
		line int
		msg  strings.Builder
	}
	panics := map[string]*place{}
	var res []loop.SensorFinding
	var current *place
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var order []string
	results := map[string]string{}
	for sc.Scan() {
		ln := sc.Text()
		if m := reCargoPanic.FindStringSubmatch(ln); m != nil {
			n, _ := strconv.Atoi(m[3])
			current = &place{file: within(src, m[2]), line: n}
			panics[m[1]] = current
			continue
		}
		if m := reCargoResult.FindStringSubmatch(ln); m != nil {
			if _, seen := results[m[1]]; !seen {
				order = append(order, m[1])
			}
			results[m[1]] = m[2]
			current = nil
			continue
		}
		if current != nil && current.msg.Len() < 1200 && !strings.HasPrefix(ln, "note:") {
			current.msg.WriteString(ln + "\n")
		}
	}
	for _, name := range order {
		f := loop.SensorFinding{Stage: stage, Rule: name, Severity: "blocker"}
		if p := panics[name]; p != nil {
			f.File, f.Line = p.file, p.line
			f.Message = name + " falhou: " + strings.TrimSpace(p.msg.String())
		} else {
			f.Message = name + " falhou"
		}
		if results[name] == "ignored" {
			f.Message = name + " foi ignorado: um teste ignorado não prova nada — faça-o rodar ou remova-o com o requisito"
		}
		res = append(res, f)
	}
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

// passedFile is where the stages that passed are remembered, in the tree
// they ran on.
func (r Runner) passedFile() string {
	return filepath.Join(r.Root, ".gofi", "cache", "sensors.json")
}

func (r Runner) loadPassed() map[string]string {
	m := map[string]string{}
	if b, err := os.ReadFile(r.passedFile()); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	return m
}

func (r Runner) savePassed(m map[string]string) {
	b, _ := json.Marshal(m)
	if err := os.MkdirAll(filepath.Dir(r.passedFile()), 0o755); err == nil {
		_ = os.WriteFile(r.passedFile(), b, 0o644)
	}
}

// treeKey is the tree as git sees it — the commit, what changed since, and
// the files git does not track yet —, so any edit anywhere is a new key. The
// reports the stages write and gofi's own folder are not of the tree.
// Empty outside git: then nothing is remembered.
func (r Runner) treeKey(ctx context.Context) string {
	git := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", r.Root}, args...)...)
		return cmd.Output()
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	diff, err := git("diff", "HEAD", "--binary")
	if err != nil {
		return ""
	}
	untracked, err := git("ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return ""
	}
	var reports []string
	for _, c := range r.Commands {
		reports = append(reports, r.reports(c)...)
	}
	h := sha256.New()
	h.Write(head)
	h.Write(diff)
	for _, f := range strings.Split(string(untracked), "\x00") {
		if f == "" || strings.HasPrefix(f, ".gofi/") {
			continue
		}
		abs := filepath.Join(r.Root, filepath.FromSlash(f))
		if slices.Contains(reports, abs) {
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "%s\x00%d\x00", f, len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashOf(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])
}
