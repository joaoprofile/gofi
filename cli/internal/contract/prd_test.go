package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// prdProject is a project with the PRD template gofi ships and the PRDs given,
// by context.
func prdProject(t *testing.T, prds map[string]string) string {
	t.Helper()
	root := t.TempDir()
	tpl, err := os.ReadFile("../../../ai/templates/prd-template.md")
	if err != nil {
		t.Fatal(err)
	}
	put := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put(layout.Templates().Path("prd-template.md"), string(tpl))
	for ctx, body := range prds {
		put(PRDPath(ctx), body)
	}
	return root
}

const goodPRD = `---
tipo: prd
formato: 3
contexto: tenant
versao: "1.0"
status: proposto
---
# PRD — tenant — Tenant pessoal dos sócios

## 1. Problema

Quem é sócio de mais de uma empresa não vê as próprias retiradas num lugar só.

## 2. Objetivo e métricas de sucesso

| Métrica | Hoje | Meta | Como medir |
|---|---|---|---|
| Sócios com retirada conciliada | 0% | 80% | relatório mensal de conciliação |

## 3. Personas

| Persona | Papel | O que precisa | O que a impede hoje |
|---|---|---|---|
| Sócio | dono de uma ou mais empresas | ver o pró-labore de todas | cada empresa mostra só o seu |

## 4. Escopo

### Dentro
- Um tenant pessoal por pessoa, pelo CPF

### Fora
- Empresa ver dado do tenant pessoal: privacidade do sócio

## 5. Jornadas

### J1 — Ligar uma empresa ao tenant pessoal
1. O administrador convida o sócio
2. O sócio aceita a ligação (o vínculo usa {cpf} como chave)

## 6. Restrições, premissas e riscos

| Tipo | Descrição | Impacto | Mitigação |
|---|---|---|---|
| Premissa | Todo sócio tem CPF válido | sem CPF não há tenant | validar no convite |

## 7. Dúvidas em aberto
`

// A PRD that says what the specification is written from passes with nothing
// to correct — and a brace of its own is not taken for the template's.
func TestAPRDReadyForTheSpecificationPasses(t *testing.T) {
	root := prdProject(t, map[string]string{"tenant": goodPRD})
	findings, state := CheckPRD(root, PRDPath("tenant"))
	if state != PRDCurrent || len(findings) != 0 {
		t.Fatalf("state=%v findings=%v", state, findings)
	}
	if got := PRDs(root); len(got) != 1 || got[0] != "prd/tenant/prd-tenant.md" {
		t.Errorf("PRDs = %v", got)
	}
}

func prdMessages(fs []docs.Finding, errs bool) string {
	var out []string
	for _, f := range fs {
		if f.Error == errs {
			out = append(out, f.Message)
		}
	}
	return strings.Join(out, "\n")
}

// What stands between a PRD and the specification is said, each thing once
// and by what it is: a doubt still open, a section missing or empty, a
// requirement written where it does not belong.
func TestThePRDSensorSaysWhatAsksForACorrection(t *testing.T) {
	weak := strings.NewReplacer(
		"Quem é sócio de mais de uma empresa não vê as próprias retiradas num lugar só.", "> O que dói hoje, para quem.",
		"| Sócios com retirada conciliada | 0% | 80% | relatório mensal de conciliação |", "| Sócios com retirada conciliada | | 80% | |",
		"### Fora\n- Empresa ver dado do tenant pessoal: privacidade do sócio\n", "### Fora\n",
		"1. O administrador convida o sócio\n2. O sócio aceita a ligação (o vínculo usa {cpf} como chave)\n", "O sistema deve permitir o convite.\n\n### Requisito: REQ-01 — Convidar o sócio\n",
		"| Premissa | Todo sócio tem CPF válido | sem CPF não há tenant | validar no convite |\n", "",
		"## 7. Dúvidas em aberto\n", "## 7. Dúvidas em aberto\n\n- [PRECISA ESCLARECER: o administrador pode ser o sócio? — produto]\n- TODO: validar com o jurídico\n",
		"## 3. Personas\n\n| Persona | Papel | O que precisa | O que a impede hoje |\n|---|---|---|---|\n| Sócio | dono de uma ou mais empresas | ver o pró-labore de todas | cada empresa mostra só o seu |\n\n", "",
	).Replace(goodPRD)
	root := prdProject(t, map[string]string{"tenant": weak})
	findings, state := CheckPRD(root, PRDPath("tenant"))
	if state != PRDCurrent {
		t.Fatal("the PRD was not read")
	}
	errs := prdMessages(findings, true)
	for _, want := range []string{
		"§1 sem o problema",
		"§2 sem métrica com hoje, meta e como medir",
		"falta a seção 3 (Personas)",
		"§4 sem nada em \"Fora\"",
		"§5 sem jornada com passos",
		"requisito ou cenário no PRD",
		"dúvida em aberto — responda antes da especificação: [PRECISA ESCLARECER: o administrador pode ser o sócio? — produto]",
		"marcador em aberto \"TODO\"",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("missing %q in:\n%s", want, errs)
		}
	}
	if strings.Contains(errs, "\"Dentro\"") || strings.Count(errs, "\n")+1 != 8 {
		t.Errorf("only what is wrong is said, once each:\n%s", errs)
	}
	warns := prdMessages(findings, false)
	if !strings.Contains(warns, "o sistema deve") || !strings.Contains(warns, "§6 sem restrição, premissa ou risco") {
		t.Errorf("warnings:\n%s", warns)
	}
	if e, w := PRDSummary(findings); e != 8 || w != 2 {
		t.Errorf("summary = %d to correct, %d to read", e, w)
	}
	lines := PRDLines(findings, 3)
	if len(lines) != 4 || lines[0] != "✗ falta a seção 3 (Personas)" || !strings.HasPrefix(lines[1], "✗ §1") || lines[3] != "… +7" {
		t.Errorf("what a section lacks comes first, and the rest is counted: %q", lines)
	}
}

// The template copied and left as it is has everything still to fill in: the
// sensor says so line by line, and does not take the template's own example
// of a doubt for an open one.
func TestAPRDStillTheTemplateIsNotReady(t *testing.T) {
	tpl, _ := os.ReadFile("../../../ai/templates/prd-template.md")
	root := prdProject(t, map[string]string{"tenant": strings.Replace(string(tpl), "contexto: {contexto}", "contexto: tenant", 1)})
	findings, state := CheckPRD(root, PRDPath("tenant"))
	errs := prdMessages(findings, true)
	found := state == PRDCurrent
	if !found || !strings.Contains(errs, "trecho do template não preenchido: {métrica}") || !strings.Contains(errs, "§2 sem métrica") || !strings.Contains(errs, "§4 sem nada em \"Dentro\"") {
		t.Fatalf("found=%v\n%s", found, errs)
	}
	if strings.Contains(errs, "dúvida em aberto") {
		t.Errorf("the template's example of a doubt was read as one:\n%s", errs)
	}
	// Shown, the pieces left in are one line, and what the sections lack is
	// not pushed out of sight by them.
	lines := PRDLines(findings, 8)
	if !strings.HasPrefix(lines[0], "✗ 12 trecho(s) do template sem preencher: L13 {contexto}, L35 {métrica}, L41 {Persona}…") || !strings.Contains(strings.Join(lines, "\n"), "§1 sem o problema") {
		t.Errorf("lines = %q", lines)
	}
}

// A PRD of an older format was written under other sections: its header is
// checked — what gofi index check reads of it — and what it says is not
// judged. A file that is not there says nothing.
func TestAnOlderPRDIsCheckedByItsHeaderOnly(t *testing.T) {
	old := strings.Replace(goodPRD, "formato: 3", "formato: 2", 1)
	old = strings.Replace(old, "## 4. Escopo", "## 4. Outra coisa", 1)
	broken := strings.Replace(old, "status: proposto\n", "status: proposto\ndono: produto\n", 1)
	root := prdProject(t, map[string]string{"legacy": old, "broken": broken})
	if f, state := CheckPRD(root, PRDPath("legacy")); state != PRDOlder || len(f) != 0 {
		t.Errorf("an older PRD with a sound header has nothing to say: state=%v %v", state, f)
	}
	if f, state := CheckPRD(root, PRDPath("broken")); state != PRDOlder || len(f) != 1 || !strings.Contains(f[0].Message, `campo desconhecido "dono"`) {
		t.Errorf("its header is still checked: state=%v %v", state, f)
	}
	if _, state := CheckPRD(root, PRDPath("none")); state != PRDMissing {
		t.Error("a PRD that does not exist was found")
	}
}

// One command closes a PRD: what gofi index check says of the same file —
// a field the format does not have — comes with what the PRD says, and
// nothing of another document does.
func TestThePRDCheckCarriesTheCorpusFindingsOfItsFile(t *testing.T) {
	withField := strings.Replace(goodPRD, "status: proposto\n", "status: proposto\ndono: produto\n", 1)
	root := prdProject(t, map[string]string{"tenant": withField, "other": strings.Replace(goodPRD, "contexto: tenant", "contexto: other\nsobra: x", 1)})
	findings, state := CheckPRD(root, PRDPath("tenant"))
	if state != PRDCurrent || len(findings) != 1 || !findings[0].Error || !strings.Contains(findings[0].Message, `campo desconhecido "dono"`) || findings[0].Path != PRDPath("tenant") {
		t.Errorf("findings = %+v", findings)
	}
}

// The sensor reads the sections the template has: a template that gains,
// loses or renames one leaves the sensor asking for the wrong PRD.
func TestThePRDSensorFollowsTheTemplate(t *testing.T) {
	tpl, err := os.ReadFile("../../../ai/templates/prd-template.md")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, ln := range strings.Split(string(tpl), "\n") {
		if m := reSection.FindStringSubmatch(ln); m != nil {
			got = append(got, strings.TrimSpace(strings.TrimPrefix(ln, "## ")))
		}
	}
	var want []string
	for _, s := range prdSections {
		want = append(want, s.num+". "+s.name)
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the template's sections are %q; the sensor reads %q", got, want)
	}
	for _, heading := range []string{"### Dentro", "### Fora"} {
		if !strings.Contains(string(tpl), "\n"+heading+"\n") {
			t.Errorf("the template lost %q, which the sensor looks for under the scope", heading)
		}
	}
}
