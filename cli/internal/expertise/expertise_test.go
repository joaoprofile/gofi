package expertise

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const kafka = `---
pack: messaging-kafka
title: Mensageria com Kafka
summary: consumidores, produtores, naming de group id
applies_when:
  imports: [gofi/msq]
  symbols: ["*Consumer", "*Producer"]
  intents: [integrar, consumir, publicar]
serves: [spec, eng, qa]
---
# Mensageria com Kafka
`

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReadsTheContracts(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".claude/expertise/messaging-kafka/PACK.md", kafka)
	write(t, root, ".claude/expertise/messaging-kafka/consumers.md", "# Consumers\n")
	packs, problems := Load(root)
	if len(problems) != 0 || len(packs) != 1 {
		t.Fatalf("packs=%+v problems=%v", packs, problems)
	}
	p := packs[0]
	if p.Name != "messaging-kafka" || p.Dir != ".claude/expertise/messaging-kafka" || len(p.When.Imports) != 1 || len(p.Serves) != 3 {
		t.Errorf("pack = %+v", p)
	}
}

// A broken contract is reported and routes nothing.
func TestLoadReportsBrokenContracts(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".claude/expertise/typo/PACK.md", strings.Replace(kafka, "applies_when", "applies_whn", 1))
	write(t, root, ".claude/expertise/no-signal/PACK.md", "---\npack: no-signal\ntitle: X\nserves: [eng]\n---\n")
	write(t, root, ".claude/expertise/wrong-role/PACK.md", "---\npack: wrong-role\ntitle: X\napplies_when:\n  intents: [x]\nserves: [dev]\n---\n")
	write(t, root, ".claude/expertise/mismatch/PACK.md", kafka)
	write(t, root, ".claude/expertise/bare/notes.md", "# x\n")
	packs, problems := Load(root)
	if len(packs) != 0 {
		t.Errorf("a broken pack was loaded: %+v", packs)
	}
	joined := ""
	for _, p := range problems {
		joined += p.String() + "\n"
	}
	for _, want := range []string{"applies_whn", "no signal", `"dev" is not a role`, "does not match its folder", "needs a PACK.md"} {
		if !strings.Contains(joined, want) {
			t.Errorf("problems lack %q:\n%s", want, joined)
		}
	}
}

// A pack every task needs says so, instead of faking a signal.
func TestAlwaysIsASignal(t *testing.T) {
	p, errs := Parse([]byte("---\npack: protocols\ntitle: X\napplies_when:\n  always: true\nserves: [eng]\n---\n"))
	if len(errs) != 0 || !p.When.Always {
		t.Fatalf("pack=%+v errs=%v", p, errs)
	}
}

// Every pack gofi ships has a valid contract, named after its folder.
func TestShippedPacks(t *testing.T) {
	dirs, err := os.ReadDir("../../../ai/expertise")
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) == 0 {
		t.Fatal("no packs shipped")
	}
	for _, d := range dirs {
		b, err := os.ReadFile(filepath.Join("../../../ai/expertise", d.Name(), ManifestFile))
		if err != nil {
			t.Errorf("%s: %v", d.Name(), err)
			continue
		}
		p, errs := Parse(b)
		if p.Name != d.Name() {
			errs = append(errs, "pack "+p.Name+" does not match its folder")
		}
		for _, e := range errs {
			t.Errorf("%s: %s", d.Name(), e)
		}
	}
}

// Every file a seeded copy is retired in favour of is one gofi still ships.
func TestMovedTargetsExist(t *testing.T) {
	for old, target := range Moved {
		if _, err := os.Stat(filepath.Join("../../../ai", filepath.FromSlash(target))); err != nil {
			t.Errorf("%s → %s: %v", old, target, err)
		}
	}
}
