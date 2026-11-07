package contract

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// Archived is what archiving a change did.
type Archived struct {
	Specs []string // living specs written, relative to root
	Dir   string   // where the change went, relative to root
}

// Archive applies an approved change (0003, S4): each delta becomes its
// context's living spec — requirements added after the last one, modified in
// place, removed — the version goes up, the spec is sealed again, and the
// change folder moves to changes/archive/<date>-<id>/. No model is involved:
// applying a delta is replacing, appending and removing blocks by id, and the
// seal of the result stands for the delta the person approved.
//
// Every spec is built and checked before anything is written; a change that
// is not sealed, does not pass the check, or would leave a spec that does not,
// is refused with nothing touched.
func Archive(root string, all []Doc, id, date string) (Archived, error) {
	c := docsOf(all, id)
	if c.proposal == nil {
		return Archived{}, fmt.Errorf("nenhuma mudança aberta %s", id)
	}
	placeholders, _ := Placeholders(root)
	for _, d := range append(append([]Doc{*c.proposal}, c.deltas...), designDocs(c)...) {
		if state, _ := SealOf(d); state != Sealed {
			return Archived{}, fmt.Errorf("%s não está aprovado como está — rode `gofi change approve %s`", d.Path, id)
		}
		if b := Blocking(root, all, d, placeholders); len(b) > 0 {
			return Archived{}, fmt.Errorf("%s: %s", b[0].Path, b[0].Message)
		}
	}

	type output struct{ path, text string }
	var outs []output
	for _, d := range c.deltas {
		text, err := apply(root, all, d, date)
		if err != nil {
			return Archived{}, fmt.Errorf("%s: %w", d.Path, err)
		}
		rel := layout.Specs().Path(d.Context, "spec.md")
		if living, ok := Living(all, d.Context); ok {
			rel = living.Path
		}
		outs = append(outs, output{rel, text})
	}
	// The result is held to the rules of a living spec before it is written.
	for _, o := range outs {
		doc := parsed(o.path, o.text)
		if found := content(root, nil, doc, placeholders); Errors(found) > 0 {
			for _, f := range found {
				if f.Error {
					return Archived{}, fmt.Errorf("a spec resultante %s não passaria no check: %s", o.path, f.Message)
				}
			}
		}
	}

	from, ok := FindChange(root, id)
	if !ok {
		return Archived{}, fmt.Errorf("a pasta da mudança %s não foi encontrada", id)
	}
	res := Archived{Dir: archiveOf(from, date, id)}
	for _, o := range outs {
		if err := writeAtomic(filepath.Join(root, filepath.FromSlash(o.path)), o.text); err != nil {
			return res, err
		}
		res.Specs = append(res.Specs, o.path)
	}
	proposalPath := filepath.Join(root, filepath.FromSlash(c.proposal.Path))
	if raw, err := os.ReadFile(proposalPath); err == nil {
		if out, err := setFields(string(raw), [][2]string{{"status", "arquivado"}}); err == nil {
			_ = writeAtomic(proposalPath, out)
		}
	}
	dest := filepath.Join(root, filepath.FromSlash(res.Dir))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return res, err
	}
	if IsGenesis(from) {
		return res, archiveGenesis(root, from, dest, c)
	}
	return res, os.Rename(filepath.Join(root, filepath.FromSlash(from)), dest)
}

// archiveGenesis moves a genesis out of its context's folder, file by file —
// the folder stays, the living spec now in it: the proposal, the deltas, the
// design and the tasks the plan derived.
func archiveGenesis(root, from, dest string, c changeDocs) error {
	dir := filepath.Join(root, filepath.FromSlash(from))
	var names []string
	for _, d := range append(append([]Doc{*c.proposal}, c.deltas...), designDocs(c)...) {
		rel, err := filepath.Rel(dir, filepath.Join(root, filepath.FromSlash(d.Path)))
		if err != nil {
			return err
		}
		names = append(names, rel)
	}
	if _, err := os.Stat(filepath.Join(dir, "tasks.md")); err == nil {
		names = append(names, "tasks.md")
	}
	for _, n := range names {
		to := filepath.Join(dest, n)
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(dir, n), to); err != nil {
			return err
		}
	}
	// deltas/ goes when nothing but what was archived was in it.
	_ = os.Remove(filepath.Join(dir, "deltas"))
	return nil
}

func designDocs(c changeDocs) []Doc {
	if c.design == nil {
		return nil
	}
	return []Doc{*c.design}
}

// apply builds the living spec a delta leaves: the current one with the
// delta's blocks applied, or a new one from the delta's Propósito and its
// added requirements. Its frontmatter carries the new version and the seal.
func apply(root string, all []Doc, d Doc, date string) (string, error) {
	blocks := deltaBlocks(d)
	living, exists := Living(all, d.Context)
	var front, body []string
	version := "1.0"
	if exists {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(living.Path)))
		if err != nil {
			return "", err
		}
		front, body = splitFront(string(raw))
		version = bump(value(living.Front, "versao"))
		body = applyBlocks(body, blocks)
	} else {
		purpose := sectionBody(d, "Propósito")
		if len(strings.TrimSpace(strings.Join(purpose, ""))) == 0 {
			return "", errors.New("contexto novo: o delta precisa de ## Propósito")
		}
		front = []string{"tipo: spec", "formato: 3", "contexto: " + d.Context}
		if kw := strings.TrimSpace(d.Front["keywords"]); kw != "" {
			front = append(front, "keywords: "+kw)
		}
		body = []string{"# Spec — " + d.Context, "", "## Propósito", ""}
		body = append(body, trimBlank(purpose)...)
		body = append(body, "", "## Requisitos")
		for _, b := range blocks[OpAdded] {
			body = append(body, "")
			body = append(body, b.lines...)
		}
		body = append(body, "")
	}
	fields := [][2]string{
		{"versao", `"` + version + `"`},
		{"status", StatusApproved},
		{"atualizado", date},
	}
	if _, set := d.Front[FieldDepends]; set {
		fields = append(fields, [2]string{FieldDepends, strings.TrimSpace(d.Front[FieldDepends])})
	}
	// Built once without the seal to hash the body, then sealed.
	doc := "---\n" + strings.Join(front, "\n") + "\n---\n" + strings.Join(body, "\n")
	doc, err := setFields(doc, fields)
	if err != nil {
		return "", err
	}
	_, finalBody := splitFront(doc)
	return setFields(doc, [][2]string{
		{FieldSealVersion, `"` + version + `"`},
		{FieldSeal, `"` + Seal(finalBody) + `"`},
	})
}

// rawBlock is a requirement as written: its heading and every line under it
// up to the next heading of its level or above.
type rawBlock struct {
	id    string
	lines []string
}

// deltaBlocks reads a delta's requirement blocks by operation, as written. A
// block under a ## that is not an operation belongs to none.
func deltaBlocks(d Doc) map[string][]rawBlock {
	out := map[string][]rawBlock{}
	for _, b := range blocksOf(d.Body) {
		if op, ok := docs.DeltaOp(b.section); ok {
			out[op] = append(out[op], b.rawBlock)
		}
	}
	return out
}

type indexedBlock struct {
	rawBlock
	start, end int    // body indexes, end exclusive
	section    string // the ## heading the block sits under, as written
}

// blocksOf finds the requirement blocks in body lines outside code, each
// with the ## section it sits under.
func blocksOf(body []string) []indexedBlock {
	var out []indexedBlock
	var cur *indexedBlock
	section := ""
	end := func(i int) {
		if cur != nil {
			cur.end = i
			cur.lines = trimBlank(body[cur.start:i])
			out = append(out, *cur)
			cur = nil
		}
	}
	fenced := false
	for i, ln := range body {
		if docs.IsFence(ln) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if id, _, ok := docs.RequirementHeading(ln); ok {
			end(i)
			cur = &indexedBlock{rawBlock: rawBlock{id: id}, start: i, section: section}
			continue
		}
		if reHeading2.MatchString(ln) || strings.HasPrefix(ln, "### ") {
			end(i)
			if reHeading2.MatchString(ln) {
				section = ln
			}
		}
	}
	end(len(body))
	return out
}

// applyBlocks applies a delta's blocks to a living spec's body: modified in
// place, removed, added after the last requirement.
func applyBlocks(body []string, delta map[string][]rawBlock) []string {
	modified := map[string][]string{}
	for _, b := range delta[OpModified] {
		modified[b.id] = b.lines
	}
	removed := map[string]bool{}
	for _, b := range delta[OpRemoved] {
		removed[b.id] = true
	}
	blocks := blocksOf(body)
	insertAt := len(body)
	if len(blocks) > 0 {
		insertAt = blocks[len(blocks)-1].end
	} else if i := sectionEnd(body, "Requisitos"); i >= 0 {
		insertAt = i
	}

	var out []string
	next := 0
	for _, b := range blocks {
		out = append(out, body[next:b.start]...)
		switch {
		case removed[b.id]:
		case modified[b.id] != nil:
			out = append(out, modified[b.id]...)
			out = append(out, "")
		default:
			out = append(out, body[b.start:b.end]...)
		}
		next = b.end
		if b.end == insertAt {
			out = appendAdded(out, delta[OpAdded])
		}
	}
	if len(blocks) == 0 {
		out = append(out, body[:insertAt]...)
		out = appendAdded(out, delta[OpAdded])
		next = insertAt
	}
	out = append(out, body[next:]...)
	return collapseBlank(out)
}

func appendAdded(out []string, added []rawBlock) []string {
	for _, b := range added {
		out = append(trimBlank(out), "")
		out = append(out, b.lines...)
		out = append(out, "")
	}
	return out
}

// sectionEnd is the body index where a ## section ends — the next ## heading,
// or the end — or -1 when there is no such section.
func sectionEnd(body []string, title string) int {
	in := false
	for i, ln := range body {
		if reHeading2.MatchString(ln) {
			if in {
				return i
			}
			in = fold(strings.TrimSpace(strings.TrimPrefix(ln, "##"))) == fold(title)
		}
	}
	if in {
		return len(body)
	}
	return -1
}

// bump raises a version by one minor step: 1.3 → 1.4, 2 → 2.1.
func bump(v string) string {
	major, minor, ok := strings.Cut(v, ".")
	if !ok {
		minor = "0"
	}
	n, err := strconv.Atoi(minor)
	if err != nil || major == "" {
		return "1.0"
	}
	return major + "." + strconv.Itoa(n+1)
}

// splitFront splits a document into its frontmatter lines (without the
// fences) and its body lines.
func splitFront(doc string) (front, body []string) {
	lines := strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, lines
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return lines[1:i], lines[i+1:]
		}
	}
	return nil, lines
}

// parsed reads a document from its text, as Docs reads one from disk.
func parsed(rel, text string) Doc {
	fm, body := docs.Parse(text)
	return Doc{Path: rel, Kind: value(fm, "tipo"), Format: formatOf(fm), Context: value(fm, "contexto"),
		Contexts: []string{value(fm, "contexto")}, Front: fm, Body: body, Offset: bodyLine(text, body)}
}

func trimBlank(lines []string) []string {
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

// collapseBlank leaves at most one blank line between blocks.
func collapseBlank(lines []string) []string {
	var out []string
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" && len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			continue
		}
		out = append(out, ln)
	}
	return out
}

func writeAtomic(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
