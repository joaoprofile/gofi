package external

import (
	"errors"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

const header = `{"rec":"header","schema":"gofi-graph/v1","language":"java","module":"com.acme.app","tool":"gofi-graph-java 0.1.0","mode":"deep"}`

func decode(t *testing.T, lines ...string) *Result {
	t.Helper()
	res, err := Decode(strings.NewReader(strings.Join(lines, "\n")), DefaultLimits())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return res
}

func TestDecodeStream(t *testing.T) {
	res := decode(t,
		header,
		`{"rec":"node","id":"java:com.acme.api","kind":"package","name":"com.acme.api","file":"src/api","lines":420}`,
		`{"rec":"node","id":"java:com.acme.api.Server","kind":"class","name":"Server","unit":"java:com.acme.api","owner":"","file":"src/api/Server.java","line":12,"vis":"public","sig":"class Server","doc":"HTTP entry point."}`,
		`{"rec":"node","id":"java:com.acme.api.Server#start","kind":"method","name":"start","unit":"java:com.acme.api","owner":"Server","file":"src/api/Server.java","line":31,"vis":"private"}`,
		`{"rec":"edge","from":"java:com.acme.api","to":"java:com.acme.api.Server","rel":"contains","file":"src/api/Server.java","line":12,"conf":1}`,
		`{"rec":"edge","from":"java:com.acme.api.Server#start","to":"java:com.acme.store.Repo#find","rel":"calls","line":34,"conf":0.6}`,
		`{"rec":"diag","severity":"warn","msg":"3 call sites skipped"}`,
		`{"rec":"summary","files":128,"loc":9310,"unresolved":3,"ambiguous":1}`,
	)

	g := res.Graph
	if g.Schema != model.SchemaVersion {
		t.Errorf("schema = %q, want %q", g.Schema, model.SchemaVersion)
	}
	if g.Module != "com.acme.app" || g.Mode != "deep" {
		t.Errorf("module/mode = %q/%q", g.Module, g.Mode)
	}
	// The language is what tells a reader which graph this is, once a repository
	// has more than one.
	if g.Language != "java" {
		t.Errorf("language = %q, want java", g.Language)
	}
	if len(g.Nodes) != 3 || len(g.Edges) != 2 {
		t.Fatalf("got %d nodes and %d edges, want 3 and 2", len(g.Nodes), len(g.Edges))
	}

	srv := g.Get("java:com.acme.api.Server")
	if srv == nil {
		t.Fatal("Server node missing")
	}
	if srv.Kind != model.KindClass {
		t.Errorf("kind = %q, want class", srv.Kind)
	}
	// unit/owner/vis now land on the neutral model unchanged: no adaptation,
	// which is the point of the wire format being the model.
	if srv.Unit != "java:com.acme.api" {
		t.Errorf("unit = %q", srv.Unit)
	}
	if srv.Vis != model.VisPublic {
		t.Errorf("vis = %q, want public", srv.Vis)
	}

	start := g.Get("java:com.acme.api.Server#start")
	if start.Owner != "Server" {
		t.Errorf("owner = %q, want Server", start.Owner)
	}
	if start.Vis != model.VisPrivate {
		t.Errorf("vis = %q, want private", start.Vis)
	}

	if g.Stats.Files != 128 || g.Stats.LOC != 9310 || g.Stats.Unresolved != 3 || g.Stats.Ambiguous != 1 {
		t.Errorf("summary not applied: %+v", g.Stats)
	}
	if res.DiagCount != 1 || len(res.Diagnostics) != 1 || res.Diagnostics[0].Severity != "warn" {
		t.Errorf("diagnostics = %+v (count %d)", res.Diagnostics, res.DiagCount)
	}
}

// A node with no explicit name falls back to its ID, and an edge with no conf
// is taken as fully resolved: both defaults the protocol promises.
func TestDecodeDefaults(t *testing.T) {
	res := decode(t,
		header,
		`{"rec":"node","id":"java:A","kind":"class"}`,
		`{"rec":"node","id":"java:B","kind":"class"}`,
		`{"rec":"edge","from":"java:A","to":"java:B","rel":"uses"}`,
	)
	if got := res.Graph.Get("java:A").Name; got != "java:A" {
		t.Errorf("name = %q, want the id", got)
	}
	if got := res.Graph.Edges[0].Conf; got != 1 {
		t.Errorf("conf = %v, want 1", got)
	}
}

// Unknown record types and unknown fields are how the protocol grows: an older
// gofi must keep reading a stream from a newer extractor.
func TestDecodeIgnoresUnknown(t *testing.T) {
	res := decode(t,
		header,
		`{"rec":"annotation","id":"java:A","payload":{"x":1}}`,
		`{"rec":"node","id":"java:A","kind":"class","name":"A","complexity":42}`,
	)
	if len(res.Graph.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(res.Graph.Nodes))
	}
}

func TestDecodeDiagnosticsAreCapped(t *testing.T) {
	lines := []string{header}
	for range 10 {
		lines = append(lines, `{"rec":"diag","msg":"noise"}`)
	}
	res, err := Decode(strings.NewReader(strings.Join(lines, "\n")), Limits{MaxLineBytes: 1 << 10, MaxRecords: 100, MaxDiags: 3})
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(res.Diagnostics) != 3 || res.DiagCount != 10 {
		t.Errorf("kept %d of %d, want 3 of 10", len(res.Diagnostics), res.DiagCount)
	}
	if res.Diagnostics[0].Severity != "info" {
		t.Errorf("missing severity should default to info, got %q", res.Diagnostics[0].Severity)
	}
}

func TestDecodeRejects(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"no header", []string{`{"rec":"node","id":"java:A","kind":"class"}`}, "header"},
		{"empty stream", nil, "header"},
		{"duplicate header", []string{header, header}, "duplicado"},
		{"wrong major", []string{`{"rec":"header","schema":"gofi-graph/v2","language":"java"}`}, "incompativel"},
		{"no schema", []string{`{"rec":"header","language":"java"}`}, "schema"},
		{"no language", []string{`{"rec":"header","schema":"gofi-graph/v1"}`}, "language"},
		{"malformed json", []string{header, `{"rec":"node"`}, "JSON invalido"},
		{"node without id", []string{header, `{"rec":"node","kind":"class"}`}, "sem id"},
		{"unknown kind", []string{header, `{"rec":"node","id":"java:A","kind":"widget"}`}, "kind desconhecido"},
		{"unknown rel", []string{header, `{"rec":"edge","from":"java:A","to":"java:B","rel":"smells-like"}`}, "rel desconhecida"},
		{"edge without from", []string{header, `{"rec":"edge","to":"java:B","rel":"calls"}`}, "from ou to"},
		{"conf above one", []string{header, `{"rec":"edge","from":"java:A","to":"java:B","rel":"calls","conf":1.5}`}, "conf invalida"},
		{"conf zero", []string{header, `{"rec":"edge","from":"java:A","to":"java:B","rel":"calls","conf":0}`}, "conf invalida"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(strings.Join(tt.lines, "\n")), DefaultLimits())
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestDecodeNoHeaderIsSentinel(t *testing.T) {
	_, err := Decode(strings.NewReader(""), DefaultLimits())
	if !errors.Is(err, ErrNoHeader) {
		t.Errorf("err = %v, want ErrNoHeader", err)
	}
}

// The limits exist because an extractor is a third-party program: gofi must
// fail with a message rather than exhaust memory on a pathological stream.
func TestDecodeEnforcesLimits(t *testing.T) {
	long := header + "\n" + `{"rec":"node","id":"java:` + strings.Repeat("x", 512) + `","kind":"class"}`
	if _, err := Decode(strings.NewReader(long), Limits{MaxLineBytes: 128, MaxRecords: 10, MaxDiags: 1}); err == nil ||
		!strings.Contains(err.Error(), "bytes") {
		t.Errorf("oversized line: err = %v", err)
	}

	lines := []string{header}
	for range 5 {
		lines = append(lines, `{"rec":"diag","msg":"x"}`)
	}
	_, err := Decode(strings.NewReader(strings.Join(lines, "\n")), Limits{MaxLineBytes: 1 << 10, MaxRecords: 3, MaxDiags: 10})
	if err == nil || !strings.Contains(err.Error(), "registros") {
		t.Errorf("too many records: err = %v", err)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add(header)
	f.Add(header + "\n" + `{"rec":"node","id":"java:A","kind":"class","name":"A","vis":"public"}`)
	f.Add(header + "\n" + `{"rec":"edge","from":"java:A","to":"java:B","rel":"calls","conf":0.5}`)
	f.Add(header + "\n" + `{"rec":"diag","severity":"warn","msg":"x"}` + "\n" + `{"rec":"summary","files":1,"loc":2}`)
	f.Add(`{"rec":"node","id":"java:A","kind":"class"}`)
	f.Add(header + "\n" + header)
	f.Add(header + "\n" + `{"rec":"edge","from":"java:A","to":"java:B","rel":"calls","conf":9}`)
	f.Add(header + "\n" + `{"rec":"???","id":`)
	f.Add("\x00\xff not json at all")

	// Small limits on purpose: what is being fuzzed is parsing and validation,
	// not volume, and a cheap target explores far more of it per second. The
	// limits themselves are covered by TestDecodeEnforcesLimits.
	lim := Limits{MaxLineBytes: 512, MaxRecords: 200, MaxDiags: 10}
	f.Fuzz(func(t *testing.T, in string) {
		res, err := Decode(strings.NewReader(in), lim)
		if err != nil {
			return
		}
		// A successful decode must hand back a usable graph, never a nil one:
		// callers index and query it without a second nil check.
		if res == nil || res.Graph == nil {
			t.Fatal("Decode returned no graph without an error")
		}
		res.Graph.Index()
		for _, n := range res.Graph.Nodes {
			if n.ID == "" || n.Name == "" {
				t.Fatalf("accepted an unusable node: %+v", n)
			}
		}
		for _, e := range res.Graph.Edges {
			if e.Conf <= 0 || e.Conf > 1 {
				t.Fatalf("accepted conf %v", e.Conf)
			}
		}
	})
}
