package hsec

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

// ErrFindings marks a scan that left findings at or above the threshold after
// false positives were applied.
var ErrFindings = errors.New("vulnerabilities found at or above the severity threshold")

// ErrNoLocalScan blocks a publish that has suppression rules but no local
// scan to take the current hashes from.
var ErrNoLocalScan = errors.New("no local scan recorded; run `gofi hsec start` first — publish sends the false positives and risk accepts of the last local scan")

// Suppressed pairs a finding with the rule that took it out of the verdict.
type Suppressed struct {
	Finding
	Rule config.HsecSuppression
}

// Result is a scan outcome after false positives and risk accepts.
type Result struct {
	Remaining    []Finding
	Suppressed   []Suppressed
	RiskAccepted []Suppressed
}

// HasSuppressions reports whether gofi, not horusec, decides the verdict.
func HasSuppressions(cfg config.HsecConfig) bool {
	return len(cfg.FalsePositives) > 0 || len(cfg.RiskAccepts) > 0
}

// Classify applies false positives first, then risk accepts to what remains.
func Classify(findings []Finding, cfg config.HsecConfig) (Result, error) {
	fp, err := ApplyFalsePositives(findings, cfg.FalsePositives)
	if err != nil {
		return Result{}, err
	}
	ra, err := ApplyFalsePositives(fp.Remaining, cfg.RiskAccepts)
	if err != nil {
		return Result{}, err
	}
	return Result{Remaining: ra.Remaining, Suppressed: fp.Suppressed, RiskAccepted: ra.Suppressed}, nil
}

// ApplyFalsePositives splits findings into remaining and matched by rules.
// The first matching rule wins.
func ApplyFalsePositives(findings []Finding, rules []config.HsecSuppression) (Result, error) {
	matchers, err := compileRules(rules)
	if err != nil {
		return Result{}, err
	}
	var res Result
	for _, f := range findings {
		if i := firstMatch(matchers, f); i >= 0 {
			res.Suppressed = append(res.Suppressed, Suppressed{Finding: f, Rule: rules[i]})
			continue
		}
		res.Remaining = append(res.Remaining, f)
	}
	return res, nil
}

// Evaluate reads the last scan output and applies the configured false
// positives. Returns ErrFindings when return_error_on_finding is set and
// anything remains.
func Evaluate(projectRoot string, cfg config.HsecConfig) (Result, error) {
	if err := VerifyAnalysis(projectRoot, true); err != nil {
		return Result{}, err
	}
	findings, err := ParseFindings(projectRoot)
	if err != nil {
		return Result{}, err
	}
	res, err := Classify(findings, cfg)
	if err != nil {
		return Result{}, err
	}
	if cfg.ReturnErrorOnFinding && len(res.Remaining) > 0 {
		return res, ErrFindings
	}
	return res, nil
}

// SuppressedHashes returns the horusec hashes of the last local scan's false
// positives and risk accepts. The hash includes the line number, so it is only
// valid for the code that was scanned — hence "last local scan".
func SuppressedHashes(projectRoot string, cfg config.HsecConfig) (falsePositives, riskAccepts []string, err error) {
	if !HasSuppressions(cfg) {
		return nil, nil, nil
	}
	findings, err := ParseFindings(projectRoot)
	if err != nil {
		return nil, nil, err
	}
	if findings == nil {
		return nil, nil, ErrNoLocalScan
	}
	res, err := Classify(findings, cfg)
	if err != nil {
		return nil, nil, err
	}
	return hashesOf(res.Suppressed), hashesOf(res.RiskAccepted), nil
}

func hashesOf(ss []Suppressed) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s.Hash != "" {
			out = append(out, s.Hash)
		}
	}
	return out
}

type ruleMatcher struct {
	rule string
	path *regexp.Regexp
	code string
}

func compileRules(rules []config.HsecSuppression) ([]ruleMatcher, error) {
	out := make([]ruleMatcher, 0, len(rules))
	for i, r := range rules {
		m := ruleMatcher{rule: strings.TrimSpace(r.Rule), code: r.Code}
		if r.Path != "" {
			re, err := globToRegexp(r.Path)
			if err != nil {
				return nil, fmt.Errorf("hsec.false_positives[%d].path %q: %w", i, r.Path, err)
			}
			m.path = re
		}
		out = append(out, m)
	}
	return out, nil
}

func firstMatch(matchers []ruleMatcher, f Finding) int {
	file := filepath.ToSlash(f.File)
	for i, m := range matchers {
		if m.rule != f.RuleID {
			continue
		}
		if m.path != nil && !m.path.MatchString(file) {
			continue
		}
		if m.code != "" && !strings.Contains(f.Code, m.code) {
			continue
		}
		return i
	}
	return -1
}

// globToRegexp supports *, ? and ** (any number of directories).
func globToRegexp(glob string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch {
		case c == '*' && i+1 < len(glob) && glob[i+1] == '*':
			i++
			if i+1 < len(glob) && glob[i+1] == '/' {
				i++
				b.WriteString("(?:.*/)?")
			} else {
				b.WriteString(".*")
			}
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
