package role

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/host"
)

func TestParse(t *testing.T) {
	c, err := Parse([]byte("# comment\ntier: light\n"))
	if err != nil || c.Tier != host.Light {
		t.Fatalf("got %+v, %v", c, err)
	}
	for _, bad := range []string{"tier: cheap\n", "", "tier: light\ntire: deep\n"} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
}

// Every skill gofi ships declares a valid contract.
func TestShippedContracts(t *testing.T) {
	skills, err := filepath.Glob("../../../ai/skills/*/SKILL.md")
	if err != nil || len(skills) == 0 {
		t.Fatalf("no skills found: %v", err)
	}
	for _, s := range skills {
		dir := filepath.Dir(s)
		b, err := os.ReadFile(filepath.Join(dir, ContractFile))
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(dir), err)
			continue
		}
		c, err := Parse(b)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(dir), err)
		}
		if c.Elicit != nil {
			if _, err := os.Stat(filepath.Join(dir, c.Elicit.Checklist)); err != nil {
				t.Errorf("%s: elicit.checklist: %v", filepath.Base(dir), err)
			}
		}
		if !strings.HasPrefix(filepath.Base(dir), "gofi-") {
			t.Errorf("unexpected skill folder %s", dir)
		}
	}
}

func TestParseChecksArtifactsAndIntents(t *testing.T) {
	for _, bad := range []string{
		"tier: light\nproduces: poem\n",
		"tier: light\nrequires:\n  - artifact: spec\n    only_on: [whatever]\n",
		"tier: light\nrequires:\n  - artifact: blueprint\n",
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
	c, err := Parse([]byte("tier: standard\nproduces: code\nrequires:\n  - artifact: spec\n    revise_on: [change]\n  - artifact: prd\n    only_on: [create]\n"))
	if err != nil {
		t.Fatal(err)
	}
	spec, prd := c.Requires[0], c.Requires[1]
	if !spec.Applies("fix") || !spec.Revises("change") || spec.Revises("fix") {
		t.Errorf("spec requirement: %+v", spec)
	}
	if prd.Applies("change") || !prd.Applies("create") {
		t.Errorf("prd requirement: %+v", prd)
	}
}

// The shipped contracts form a plan with no dead end: every role a contract
// names exists, and every artifact required has a role that produces it.
func TestShippedContractsAreClosed(t *testing.T) {
	dir := t.TempDir()
	skills, _ := filepath.Glob("../../../ai/skills/*/" + ContractFile)
	for _, s := range skills {
		name := filepath.Base(filepath.Dir(s))
		b, _ := os.ReadFile(s)
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, ContractFile), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	roles, problems := Load(dir)
	if len(problems) > 0 || len(roles) != len(skills) {
		t.Fatalf("roles=%d skills=%d problems=%v", len(roles), len(skills), problems)
	}
	names, producers := map[string]bool{}, map[string]string{}
	for _, r := range roles {
		names[r.Name] = true
		if p := r.Contract.Produces; p != "" {
			if other, dup := producers[p]; dup {
				t.Errorf("%s and %s both produce %s", other, r.Name, p)
			}
			producers[p] = r.Name
		}
	}
	for _, r := range roles {
		for _, n := range r.Contract.Then {
			if !names[n] {
				t.Errorf("%s: then names %q, which is not a role", r.Name, n)
			}
		}
		for _, req := range r.Contract.Requires {
			if producers[req.Artifact] == "" {
				t.Errorf("%s requires %s, which no role produces", r.Name, req.Artifact)
			}
		}
	}
}

// An elicitation runs below the role's tier, on a checklist of its own.
func TestParseChecksTheElicitation(t *testing.T) {
	c, err := Parse([]byte("tier: deep\nelicit:\n  tier: standard\n  checklist: reference/elicitation.md\n"))
	if err != nil || c.Elicit == nil || c.Elicit.Tier != host.Standard {
		t.Fatalf("got %+v, %v", c.Elicit, err)
	}
	for _, bad := range []string{
		"tier: standard\nelicit:\n  tier: deep\n  checklist: x.md\n",
		"tier: deep\nelicit:\n  tier: deep\n  checklist: x.md\n",
		"tier: deep\nelicit:\n  tier: light\n",
		"tier: deep\nelicit:\n  tier: cheap\n  checklist: x.md\n",
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
}
