package retrieval

import (
	"math"
	"sort"
)

// Fields of a searchable unit, and how much a match in each counts. A term in
// a heading or a symbol name says what the unit is about; in the body it may be
// incidental; in the path it is a weak hint.
type field int

const (
	fTitle field = iota // section heading, symbol name
	fHead               // document title, context and facets; package and owner
	fBody               // section text; doc comment and signature
	fPath               // file path
	nFields
)

var weights = [nFields]float64{fTitle: 3, fHead: 1.5, fBody: 1, fPath: 0.5}

// docShare is how much of its document's score a section inherits. Measured
// on bench/: anything from 0.5 to 1 lifts every metric over the section alone,
// and the lower end keeps the section itself deciding within its document.
var docShare = 0.5

// titleBonus is what a query term found in the title adds beyond BM25F, in
// units of the term's idf. Measured on bench/: 0.5 to 0.75 is a plateau that
// lifts the right section from 47% to 58%; higher starts to rank headings
// over content.
var titleBonus = 0.5

// surfaceWeight is how much the exact form of a word asked counts, on top of
// its stem: enough to break a tie between inflections, never to outvote the
// stem.
var surfaceWeight = 0.3

// pairWeight is how much a pair of adjacent words asked counts when the unit
// has them adjacent too. A tie-breaker: from 0.2 up it starts moving
// documents, not only the order of near-equal sections.
var pairWeight = 0.1

// skillsWeight scales the score of a skill's reference files. A reference
// applies a rule to one skill's procedure; the rule itself lives in knowledge/
// and sdk/, and a question about the rule should land on the canonical text.
// The reference still answers a question about the procedure. Measured on
// bench/: 0.8 puts the canonical rule back on top while every question about a
// skill's own procedure still lands in that skill; 0.7 starts losing those.
var skillsWeight = 0.8

// referenceWeight scales the score of the SDK reference generated from code.
// The reference says what a symbol is; the curated knowledge says how and when
// to use it, and a question about use should land there. A search for a
// symbol still finds the reference: its name is in the heading.
var referenceWeight = 0.6

// BM25 parameters: k1 is how fast repeated terms stop adding, b how much a
// long unit is discounted. The textbook values, which bench/ confirmed; they
// are variables only so a tuning run can vary them.
var (
	k1 = 1.2
	b  = 0.75
)

// unitTerms is a unit's term counts per field.
type unitTerms struct {
	tf  [nFields]map[string]int
	len [nFields]int
}

func newUnitTerms(texts [nFields]string) unitTerms {
	var u unitTerms
	for f := field(0); f < nFields; f++ {
		u.tf[f] = map[string]int{}
		for _, t := range tokens(texts[f]) {
			u.tf[f][t]++
			u.len[f]++
		}
		// Surface forms ride along without lengthening the unit: they are a
		// second view of the same words, not more words.
		for _, t := range surfaces(texts[f]) {
			u.tf[f][t]++
		}
		for _, t := range pairs(texts[f]) {
			u.tf[f][t]++
		}
	}
	return u
}

// ranker scores units with BM25F: per-field term frequencies are weighted and
// length-normalized into one frequency, which the usual BM25 saturation and
// inverse document frequency turn into a score. Deterministic, offline and
// free — no model, no network.
type ranker struct {
	n      int
	df     map[string]int
	avgLen [nFields]float64
}

func newRanker(units []unitTerms) *ranker {
	r := &ranker{n: len(units), df: map[string]int{}}
	var total [nFields]int
	for _, u := range units {
		seen := map[string]bool{}
		for f := field(0); f < nFields; f++ {
			total[f] += u.len[f]
			for t := range u.tf[f] {
				if !seen[t] {
					seen[t] = true
					r.df[t]++
				}
			}
		}
	}
	for f := field(0); f < nFields; f++ {
		r.avgLen[f] = math.Max(float64(total[f])/math.Max(float64(r.n), 1), 1)
	}
	return r
}

func (r *ranker) idf(t string) float64 {
	df := float64(r.df[t])
	return math.Log(1 + (float64(r.n)-df+0.5)/(df+0.5))
}

// queryTerm is one term of a question and how much it counts: the words asked
// fully, their exact forms, pairs and synonyms less.
type queryTerm struct {
	t string
	w float64
}

// ordered lists a question's terms in a fixed order. Scores are sums of
// floats, and a sum taken in map order differs in its last bits from run to
// run — enough to swap two near-equal answers and make the search
// non-deterministic.
func ordered(terms map[string]float64) []queryTerm {
	out := make([]queryTerm, 0, len(terms))
	for t, w := range terms {
		out = append(out, queryTerm{t, w})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].t < out[j].t })
	return out
}

// score rates one unit for the query terms; weight lets a synonym count less
// than the word that was asked.
func (r *ranker) score(u unitTerms, terms []queryTerm) float64 {
	s := 0.0
	for _, q := range terms {
		t, weight := q.t, q.w
		tf := 0.0
		for f := field(0); f < nFields; f++ {
			if c := u.tf[f][t]; c > 0 {
				norm := 1 - b + b*float64(u.len[f])/r.avgLen[f]
				tf += weights[f] * float64(c) / norm
			}
		}
		if tf > 0 {
			s += weight * r.idf(t) * tf * (k1 + 1) / (tf + k1)
		}
		// Saturation caps a term at the same ceiling wherever it matched, so a
		// heading that names the question's subject earned nothing over a body
		// that merely mentions it often. The heading is the author's own
		// summary of the unit: a match there is added outside the cap.
		if u.tf[fTitle][t] > 0 {
			s += titleBonus * weight * r.idf(t)
		}
	}
	return s
}
