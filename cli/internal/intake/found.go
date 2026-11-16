package intake

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/scan"
)

// Found is what the repository already holds of a context gofi has no
// document of yet: its code, the tables it names, the documents written
// before gofi that read as a PRD or a spec. It is read by the same scan as
// `gofi scan` — the disk, no model — so a discovery or a specification of a
// project being adopted starts from what is there instead of asking for it.
type Found struct {
	// Dir is where the context's code is, with its sources and tests counted.
	Dir     string `json:"dir,omitempty"`
	Sources int    `json:"sources,omitempty"`
	Tests   int    `json:"tests,omitempty"`
	// Tables are the tables the schema and the migrations create that the
	// context names.
	Tables []string `json:"tables,omitempty"`
	// Documents are the ones that name the context, each with what it reads as.
	Documents []FoundDoc `json:"documents,omitempty"`
}

// FoundDoc is a document of the repository, outside gofi's own, and what it
// reads as: prd, spec, decision, api, ops, readme.
type FoundDoc struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// What a Found carries at most: pointers, never the text.
const (
	foundTables = 12
	foundDocs   = 6
)

// writesDocs reports whether a plan writes a PRD or a contract — the phases
// that would otherwise ask the person what the repository already says.
func writesDocs(plan []Phase) bool {
	for _, ph := range plan {
		if ph.Produces == "prd" || ph.Produces == "spec" {
			return true
		}
	}
	return false
}

// repository is the scan of a project: the conversation's, when there is one.
func repository(root string, engines *Engines) *scan.Report {
	if engines != nil {
		return engines.scanned(root)
	}
	report, _ := scan.Scan(root)
	return report
}

// namedInRepository is the context of the code that a word of the request
// names — the first one, in the order the request says them —, or "".
func namedInRepository(root string, engines *Engines, ws []string) string {
	report := repository(root, engines)
	if report == nil {
		return ""
	}
	byWord := map[string]string{}
	for _, c := range report.Contexts {
		byWord[normalize(c.Name)] = c.Name
	}
	for _, w := range ws {
		if name, ok := byWord[w]; ok {
			return name
		}
	}
	return ""
}

// foundIn reads what the repository holds of a context, or nil when it holds
// nothing of it.
func foundIn(root string, engines *Engines, name string) *Found {
	report := repository(root, engines)
	if report == nil {
		return nil
	}
	kinds := map[string]string{}
	for _, d := range report.Documents {
		kinds[d.Path] = d.Kind
	}
	want := normalize(name)
	for _, c := range report.Contexts {
		if normalize(c.Name) != want {
			continue
		}
		f := &Found{Dir: c.Dir, Sources: c.Sources, Tests: c.Tests, Tables: c.Tables}
		if len(f.Tables) > foundTables {
			f.Tables = f.Tables[:foundTables]
		}
		for _, p := range c.Documents {
			if len(f.Documents) == foundDocs {
				break
			}
			f.Documents = append(f.Documents, FoundDoc{Path: p, Kind: kinds[p]})
		}
		return f
	}
	return nil
}

// summary says in a line what was found.
func (f *Found) Summary() string {
	var parts []string
	if f.Dir != "" {
		parts = append(parts, fmt.Sprintf("código em `%s` (%d fontes, %d testes)", f.Dir, f.Sources, f.Tests))
	}
	if n := len(f.Tables); n > 0 {
		parts = append(parts, fmt.Sprintf("%d tabela(s)", n))
	}
	if n := len(f.Documents); n > 0 {
		parts = append(parts, fmt.Sprintf("%d documento(s)", n))
	}
	return strings.Join(parts, ", ")
}

// lines are the pointers the assembled request carries.
func (f *Found) lines() []string {
	var out []string
	if f.Dir != "" {
		out = append(out, fmt.Sprintf("- código: `%s` — %d fontes, %d testes", f.Dir, f.Sources, f.Tests))
	}
	if len(f.Tables) > 0 {
		out = append(out, "- tabelas: "+strings.Join(f.Tables, ", "))
	}
	for _, d := range f.Documents {
		kind := d.Kind
		if kind == "" {
			kind = "documento"
		}
		out = append(out, fmt.Sprintf("- `%s` — lê como %s", d.Path, kind))
	}
	return out
}

// scanAge is how long a scan of the repository is taken as current by a
// process that plans many requests. What it reads — folders, schemas,
// documents written before gofi — does not change from one message to the
// next, and reading it again costs up to a second on a large repository.
const scanAge = 5 * time.Minute

type scannedRepo struct {
	at      time.Time
	report  *scan.Report
	reading bool
}

var (
	scansMu sync.Mutex
	scans   = map[*Engines]map[string]*scannedRepo{}
)

// Warm reads the repository aside, so the first request that needs it does
// not wait for the reading: a conversation calls it as it opens.
func (c *Engines) Warm(root string) {
	go c.scanned(root)
}

// scanned is the scan of a repository, read once and kept: a request never
// waits for a second reading — past scanAge the one there is still answers,
// and a new one is read aside for the next.
func (c *Engines) scanned(root string) *scan.Report {
	scansMu.Lock()
	if scans[c] == nil {
		scans[c] = map[string]*scannedRepo{}
	}
	got := scans[c][root]
	if got != nil && got.report != nil {
		if time.Since(got.at) >= scanAge && !got.reading {
			got.reading = true
			go func() {
				report, err := scan.Scan(root)
				scansMu.Lock()
				defer scansMu.Unlock()
				got.reading = false
				if err == nil {
					got.at, got.report = time.Now(), report
				}
			}()
		}
		report := got.report
		scansMu.Unlock()
		return report
	}
	scansMu.Unlock()
	// The first reading: whoever asks waits for it, once.
	report, err := scan.Scan(root)
	if err != nil {
		return nil
	}
	scansMu.Lock()
	defer scansMu.Unlock()
	if kept := scans[c][root]; kept != nil && kept.report != nil {
		return kept.report // another request read it meanwhile
	}
	scans[c][root] = &scannedRepo{at: time.Now(), report: report}
	return report
}
