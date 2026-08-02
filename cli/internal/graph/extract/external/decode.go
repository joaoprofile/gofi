package external

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// Limits bound what a single extractor run may produce. An extractor is a
// third-party program: it may be buggy, it may be fed a pathological
// repository, and gofi has to fail with a message instead of exhausting memory.
type Limits struct {
	MaxLineBytes int // longest single record
	MaxRecords   int // total records accepted
	MaxDiags     int // diagnostics kept; the rest are counted only
}

// DefaultLimits are sized for a very large monorepo with room to spare.
func DefaultLimits() Limits {
	return Limits{
		MaxLineBytes: 1 << 20,   // 1 MB — a node with a huge signature still fits
		MaxRecords:   4_000_000, // ~4M nodes+edges
		MaxDiags:     100,
	}
}

// Result is what one extractor run produced.
type Result struct {
	Graph       *model.Graph
	Diagnostics []Diagnostic
	DiagCount   int // total emitted, including those dropped past MaxDiags
}

// ErrNoHeader means the stream never identified itself. Usually the program is
// not a gofi extractor at all, or it crashed before writing anything.
var ErrNoHeader = errors.New("o extractor nao enviou o registro header")

// Decode reads an NDJSON extractor stream and builds the graph.
//
// Malformed input is an error, not a silent skip: a half-read graph is worse
// than no graph, because nothing downstream can tell the difference between "no
// edge exists" and "the edge was dropped". Unknown record types and unknown
// fields are the exception — those are forward compatibility, and are ignored.
func Decode(r io.Reader, lim Limits) (*Result, error) {
	if lim.MaxLineBytes <= 0 {
		lim = DefaultLimits()
	}
	sc := bufio.NewScanner(r)
	// Scanner takes the larger of cap(buf) and max, so the starting buffer has
	// to stay under MaxLineBytes or the limit would never bite.
	sc.Buffer(make([]byte, 0, min(64*1024, lim.MaxLineBytes)), lim.MaxLineBytes)

	g := model.New("", "", "fast")
	res := &Result{Graph: g}

	var header bool
	var records int
	var lineNo int

	for sc.Scan() {
		lineNo++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		records++
		if records > lim.MaxRecords {
			return nil, fmt.Errorf("o extractor passou de %d registros", lim.MaxRecords)
		}

		var l line
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			return nil, fmt.Errorf("linha %d: JSON invalido: %w", lineNo, err)
		}
		if !header && l.Rec != recHeader {
			return nil, fmt.Errorf("linha %d: esperado o registro header primeiro, veio %q", lineNo, l.Rec)
		}

		switch l.Rec {
		case recHeader:
			if header {
				return nil, fmt.Errorf("linha %d: header duplicado", lineNo)
			}
			if err := applyHeader(g, l); err != nil {
				return nil, fmt.Errorf("linha %d: %w", lineNo, err)
			}
			header = true

		case recNode:
			n, err := toNode(l)
			if err != nil {
				return nil, fmt.Errorf("linha %d: %w", lineNo, err)
			}
			g.AddNode(n)

		case recEdge:
			e, err := toEdge(l)
			if err != nil {
				return nil, fmt.Errorf("linha %d: %w", lineNo, err)
			}
			g.AddEdge(e)

		case recDiag:
			res.DiagCount++
			if len(res.Diagnostics) < lim.MaxDiags {
				sev := l.Severity
				if sev == "" {
					sev = "info"
				}
				res.Diagnostics = append(res.Diagnostics, Diagnostic{Severity: sev, Message: l.Msg})
			}

		case recSummary:
			g.Stats.Files = l.Files
			g.Stats.LOC = l.LOC
			g.Stats.Unresolved = l.Unresolved
			g.Stats.Ambiguous = l.Ambiguous

		default:
			// An extractor built against a newer protocol may emit records this
			// build has never heard of. Skipping them is the compatibility rule.
		}
	}
	if err := sc.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf("linha %d passou de %d bytes", lineNo+1, lim.MaxLineBytes)
		}
		return nil, err
	}
	if !header {
		return nil, ErrNoHeader
	}
	return res, nil
}

func applyHeader(g *model.Graph, l line) error {
	if err := checkSchema(l.Schema); err != nil {
		return err
	}
	if strings.TrimSpace(l.Language) == "" {
		return errors.New("header sem campo language")
	}
	g.Schema = model.SchemaVersion
	g.Language = strings.ToLower(strings.TrimSpace(l.Language))
	g.Module = l.Module
	g.Tool = l.Tool
	g.Mode = l.Mode
	if g.Mode == "" {
		g.Mode = "fast"
	}
	return nil
}

func toNode(l line) (*model.Node, error) {
	if l.ID == "" {
		return nil, errors.New("no sem id")
	}
	kind, ok := wireKinds[l.Kind]
	if !ok {
		return nil, fmt.Errorf("no %q com kind desconhecido %q", l.ID, l.Kind)
	}
	name := l.Name
	if name == "" {
		name = l.ID
	}
	return &model.Node{
		ID:      l.ID,
		Kind:    kind,
		Name:    name,
		Unit:    l.Unit,
		Owner:   l.Owner,
		File:    l.File,
		Line:    l.Line,
		Vis:     toVis(l.Vis),
		Context: l.Context,
		Doc:     l.Doc,
		Sig:     l.Sig,
		Lines:   l.Lines,
	}, nil
}

// toVis maps the protocol's visibility keyword onto the model. An extractor may
// use its own language's word, and an omitted one means public: a declaration
// an extractor bothered to emit is visible unless it says otherwise.
func toVis(vis string) model.Vis {
	switch strings.ToLower(strings.TrimSpace(vis)) {
	case "", "public", "exported", "open":
		return model.VisPublic
	case "protected":
		return model.VisProtected
	case "package", "internal", "package-private":
		return model.VisPackage
	default:
		return model.VisPrivate
	}
}

func toEdge(l line) (*model.Edge, error) {
	if l.From == "" || l.To == "" {
		return nil, errors.New("aresta sem from ou to")
	}
	rel, ok := wireRels[l.Rel]
	if !ok {
		return nil, fmt.Errorf("aresta %s->%s com rel desconhecida %q", l.From, l.To, l.Rel)
	}
	// A missing conf means the extractor is sure; a present one must be usable
	// as a weight, so the report can rank by it.
	conf := 1.0
	if l.Conf != nil {
		conf = *l.Conf
	}
	if conf <= 0 || conf > 1 {
		return nil, fmt.Errorf("aresta %s->%s com conf invalida %v (esperado 0 < conf <= 1)", l.From, l.To, conf)
	}
	return &model.Edge{
		From: l.From,
		To:   l.To,
		Rel:  rel,
		File: l.File,
		Line: l.Line,
		Conf: conf,
	}, nil
}
