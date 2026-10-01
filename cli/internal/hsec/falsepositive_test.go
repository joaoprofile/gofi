package hsec

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
)

func TestGlobToRegexp(t *testing.T) {
	cases := []struct {
		glob, path string
		want       bool
	}{
		{"**/locales/*.json", "frontend/src/features/auth/locales/en.json", true},
		{"**/locales/*.json", "locales/en.json", true},
		{"**/locales/*.json", "frontend/locales/sub/en.json", false},
		{"backend/**", "backend/domain/x/y.go", true},
		{"backend/**", "frontend/backend.go", false},
		{"**/*.tsx", "a/b/c.tsx", true},
		{"**/*.tsx", "a/b/c.ts", false},
		{"src/?.go", "src/a.go", true},
		{"src/?.go", "src/ab.go", false},
		{"a.b/*.go", "aXb/x.go", false},
	}
	for _, c := range cases {
		re, err := globToRegexp(c.glob)
		if err != nil {
			t.Fatalf("%s: %v", c.glob, err)
		}
		if got := re.MatchString(c.path); got != c.want {
			t.Errorf("glob %q vs %q: got %v want %v", c.glob, c.path, got, c.want)
		}
	}
}

func i18nRule() config.HsecSuppression {
	return config.HsecSuppression{Rule: "HS-LEAKS-26", Path: "**/locales/*.json", Reason: "i18n labels"}
}

func TestApplyFalsePositives_MatchesRulePathAndCode(t *testing.T) {
	findings := []Finding{
		{RuleID: "HS-LEAKS-26", File: "frontend/src/auth/locales/pt.json", Code: `"NEW_PASSWORD": "Nova senha"`},
		{RuleID: "HS-LEAKS-26", File: "backend/domain/x/secret.go", Code: `password := "hunter2"`},
		{RuleID: "HS-OTHER-1", File: "frontend/src/auth/locales/pt.json"},
		{RuleID: "HS-JAVASCRIPT-2", File: "frontend/src/a.tsx", Code: "confirm({ title })"},
		{RuleID: "HS-JAVASCRIPT-2", File: "frontend/src/b.tsx", Code: "window.alert('x')"},
	}
	rules := []config.HsecSuppression{
		i18nRule(),
		{Rule: "HS-JAVASCRIPT-2", Code: "confirm(", Reason: "useConfirm hook, not window.confirm"},
	}
	res, err := ApplyFalsePositives(findings, rules)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Suppressed) != 2 || len(res.Remaining) != 3 {
		t.Fatalf("got %d suppressed / %d remaining: %+v", len(res.Suppressed), len(res.Remaining), res)
	}
	for _, f := range res.Remaining {
		if f.File == "frontend/src/auth/locales/pt.json" && f.RuleID == "HS-LEAKS-26" {
			t.Error("i18n label should have been suppressed")
		}
		if f.Code == "confirm({ title })" {
			t.Error("confirm( snippet should have been suppressed")
		}
	}
	if res.Suppressed[0].Rule.Reason != "i18n labels" {
		t.Errorf("suppression must carry its rule: %+v", res.Suppressed[0])
	}
}

// Regression: horusec's hash includes the line, so a hash list resurfaces the
// finding on any edit above it. A rule-based suppression must not.
func TestApplyFalsePositives_SurvivesLineShift(t *testing.T) {
	before := Finding{RuleID: "HS-LEAKS-26", File: "fe/locales/en.json", Line: "4", Hash: "182488d8"}
	after := Finding{RuleID: "HS-LEAKS-26", File: "fe/locales/en.json", Line: "7", Hash: "606946f4"}
	for _, f := range []Finding{before, after} {
		res, err := ApplyFalsePositives([]Finding{f}, []config.HsecSuppression{i18nRule()})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Suppressed) != 1 {
			t.Fatalf("line %s / hash %s must stay suppressed", f.Line, f.Hash)
		}
	}
}

func writeOutput(t *testing.T, root string, vulns ...map[string]string) {
	t.Helper()
	type entry struct {
		Vulnerabilities map[string]string `json:"vulnerabilities"`
	}
	doc := struct {
		Status                  string  `json:"status"`
		AnalysisVulnerabilities []entry `json:"analysisVulnerabilities"`
	}{Status: "success"}
	for _, v := range vulns {
		doc.AnalysisVulnerabilities = append(doc.AnalysisVulnerabilities, entry{Vulnerabilities: v})
	}
	body, _ := json.Marshal(doc)
	if err := os.MkdirAll(filepath.Join(root, ".gofi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, OutputFileName), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func fpHsec() config.HsecConfig {
	c := config.DefaultHsec()
	c.ReturnErrorOnFinding = true
	c.FalsePositives = []config.HsecSuppression{i18nRule()}
	return c
}

func TestEvaluate_OnlySuppressedFindingsPass(t *testing.T) {
	root := t.TempDir()
	writeOutput(t, root, map[string]string{"rule_id": "HS-LEAKS-26", "file": "fe/locales/en.json", "severity": "CRITICAL", "vulnHash": "h1"})
	res, err := Evaluate(root, fpHsec())
	if err != nil || len(res.Suppressed) != 1 || len(res.Remaining) != 0 {
		t.Fatalf("expected pass with 1 suppressed, got %+v, %v", res, err)
	}
}

func TestEvaluate_RemainingFindingFailsGate(t *testing.T) {
	root := t.TempDir()
	writeOutput(t, root,
		map[string]string{"rule_id": "HS-LEAKS-26", "file": "fe/locales/en.json", "severity": "CRITICAL", "vulnHash": "h1"},
		map[string]string{"rule_id": "G101", "file": "backend/x.go", "severity": "HIGH", "vulnHash": "h2"},
	)
	_, err := Evaluate(root, fpHsec())
	if !errors.Is(err, ErrFindings) {
		t.Fatalf("expected ErrFindings, got %v", err)
	}
}

func TestSuppressedHashes_FromLastLocalScan(t *testing.T) {
	root := t.TempDir()
	if _, _, err := SuppressedHashes(root, fpHsec()); !errors.Is(err, ErrNoLocalScan) {
		t.Fatalf("publish with rules and no local scan must fail, got %v", err)
	}
	writeOutput(t, root,
		map[string]string{"rule_id": "HS-LEAKS-26", "file": "fe/locales/en.json", "vulnHash": "h1"},
		map[string]string{"rule_id": "G101", "file": "backend/x.go", "vulnHash": "h2"},
	)
	got, _, err := SuppressedHashes(root, fpHsec())
	if err != nil || !slices.Equal(got, []string{"h1"}) {
		t.Fatalf("expected only the suppressed hash, got %v, %v", got, err)
	}
	none, _, err := SuppressedHashes(t.TempDir(), config.DefaultHsec())
	if err != nil || none != nil {
		t.Fatalf("no rules → no hashes and no local-scan requirement, got %v, %v", none, err)
	}
}

func TestBuildHorusecConfig_FalsePositivesMoveVerdictToGofi(t *testing.T) {
	c := fpHsec()
	c.OutputFormat = "text"
	body, err := BuildHorusecConfig(c, "/tmp/out.json")
	if err != nil {
		t.Fatal(err)
	}
	var got horusecJSON
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.ReturnErrorIfFoundVulnerability {
		t.Error("with false positives gofi decides the verdict; horusec must not fail on suppressed findings")
	}
	if got.PrintOutputType != "json" {
		t.Errorf("gofi needs the JSON output to apply false positives, got %q", got.PrintOutputType)
	}
}

func TestBuildArgs_SendsFalsePositiveHashes(t *testing.T) {
	args := buildArgs(config.DefaultHsec(), RunOptions{ProjectRoot: "/p", ConfigPath: "/c", FalsePositiveHashes: []string{"h1", "h2"}})
	i := slices.Index(args, "-F")
	if i < 0 || args[i+1] != "h1,h2" {
		t.Fatalf("expected -F h1,h2, got %v", args)
	}
	if slices.Contains(buildArgs(config.DefaultHsec(), RunOptions{ProjectRoot: "/p", ConfigPath: "/c"}), "-F") {
		t.Error("no hashes → no -F")
	}
}

// Regression: with false-positive rules gofi decides the verdict, and it used
// to read a `status: error` (partial) analysis as clean.
func TestEvaluate_IncompleteAnalysisNeverPasses(t *testing.T) {
	root := t.TempDir()
	body := `{"status":"error","errors":"Error while running tool HorusecEngine: too many open files","analysisVulnerabilities":[]}`
	if err := os.MkdirAll(filepath.Join(root, ".gofi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, OutputFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Evaluate(root, fpHsec())
	if !errors.Is(err, ErrAnalysisIncomplete) {
		t.Fatalf("expected ErrAnalysisIncomplete, got %v", err)
	}
}

func TestEvaluate_MissingOutputNeverPasses(t *testing.T) {
	if _, err := Evaluate(t.TempDir(), fpHsec()); !errors.Is(err, ErrNoOutput) {
		t.Fatalf("expected ErrNoOutput, got %v", err)
	}
}

func TestVerifyAnalysis_OutputOptionalWithoutRules(t *testing.T) {
	if err := VerifyAnalysis(t.TempDir(), false); err != nil {
		t.Fatalf("text output mode writes no JSON; that is not a failure: %v", err)
	}
}

func tlsRiskAccept() config.HsecSuppression {
	return config.HsecSuppression{Rule: "G402", Path: "backend/scraper/unblocker.go", Reason: "provider intercepts TLS"}
}

func TestClassify_FalsePositivesThenRiskAccepts(t *testing.T) {
	c := fpHsec()
	c.RiskAccepts = []config.HsecSuppression{tlsRiskAccept()}
	findings := []Finding{
		{RuleID: "HS-LEAKS-26", File: "fe/locales/en.json", Hash: "h1"},
		{RuleID: "G402", File: "backend/scraper/unblocker.go", Hash: "h2"},
		{RuleID: "G402", File: "backend/other/client.go", Hash: "h3"},
	}
	res, err := Classify(findings, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Suppressed) != 1 || len(res.RiskAccepted) != 1 || len(res.Remaining) != 1 {
		t.Fatalf("got fp=%d ra=%d remaining=%d", len(res.Suppressed), len(res.RiskAccepted), len(res.Remaining))
	}
	if res.Remaining[0].File != "backend/other/client.go" {
		t.Errorf("a risk accept must stay scoped to its path: %+v", res.Remaining)
	}
}

func TestEvaluate_RiskAcceptedFindingPassesGate(t *testing.T) {
	root := t.TempDir()
	writeOutput(t, root, map[string]string{"rule_id": "G402", "file": "backend/scraper/unblocker.go", "severity": "HIGH", "vulnHash": "h2"})
	c := config.DefaultHsec()
	c.ReturnErrorOnFinding = true
	c.RiskAccepts = []config.HsecSuppression{tlsRiskAccept()}
	res, err := Evaluate(root, c)
	if err != nil || len(res.RiskAccepted) != 1 || len(res.Remaining) != 0 {
		t.Fatalf("expected pass with 1 risk accept, got %+v, %v", res, err)
	}
}

func TestSuppressedHashes_SplitsFalsePositivesAndRiskAccepts(t *testing.T) {
	root := t.TempDir()
	writeOutput(t, root,
		map[string]string{"rule_id": "HS-LEAKS-26", "file": "fe/locales/en.json", "vulnHash": "h1"},
		map[string]string{"rule_id": "G402", "file": "backend/scraper/unblocker.go", "vulnHash": "h2"},
	)
	c := fpHsec()
	c.RiskAccepts = []config.HsecSuppression{tlsRiskAccept()}
	fp, ra, err := SuppressedHashes(root, c)
	if err != nil || !slices.Equal(fp, []string{"h1"}) || !slices.Equal(ra, []string{"h2"}) {
		t.Fatalf("the Manager must get false positives (-F) and risk accepts (-R) apart: fp=%v ra=%v err=%v", fp, ra, err)
	}
}

func TestBuildArgs_SendsRiskAcceptHashes(t *testing.T) {
	args := buildArgs(config.DefaultHsec(), RunOptions{ProjectRoot: "/p", ConfigPath: "/c", RiskAcceptHashes: []string{"h2"}})
	i := slices.Index(args, "-R")
	if i < 0 || args[i+1] != "h2" {
		t.Fatalf("expected -R h2, got %v", args)
	}
}

func TestBuildHorusecConfig_RiskAcceptsAloneMoveVerdictToGofi(t *testing.T) {
	c := config.DefaultHsec()
	c.ReturnErrorOnFinding = true
	c.RiskAccepts = []config.HsecSuppression{tlsRiskAccept()}
	body, err := BuildHorusecConfig(c, "")
	if err != nil {
		t.Fatal(err)
	}
	var got horusecJSON
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.ReturnErrorIfFoundVulnerability || got.PrintOutputType != "json" {
		t.Fatalf("risk accepts need gofi to decide the verdict: %+v", got)
	}
}
