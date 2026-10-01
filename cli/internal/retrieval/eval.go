package retrieval

import "github.com/gofi-labs/gofi/cli/internal/docs"

// Evaluation is how well the engine answers a golden set.
type Evaluation struct {
	Questions int
	// DocTop1/3/5 count questions whose document is among the first 1, 3, 5
	// hits. SectionTop3 counts those whose hit also points at the right
	// section — the lines the agent will read, which is what saves tokens.
	DocTop1, DocTop3, DocTop5, SectionTop3 int
	MRR                                    float64
	Misses                                 []docs.Question // document not in the top 5
}

// Evaluate asks every golden question and scores the answers.
func (e *Engine) Evaluate(questions []docs.Question) Evaluation {
	ev := Evaluation{Questions: len(questions)}
	for _, q := range questions {
		hits := e.Find(Query{Text: q.Ask, Limit: 5})
		rank := 0
		for i, h := range hits {
			if h.Path == q.Document {
				rank = i + 1
				if i < 3 && (q.Section == "" || h.Title == q.Section) {
					ev.SectionTop3++
				}
				break
			}
		}
		switch {
		case rank == 0:
			ev.Misses = append(ev.Misses, q)
			continue
		case rank == 1:
			ev.DocTop1++
			fallthrough
		case rank <= 3:
			ev.DocTop3++
			fallthrough
		default:
			ev.DocTop5++
		}
		ev.MRR += 1 / float64(rank)
	}
	if ev.Questions > 0 {
		ev.MRR /= float64(ev.Questions)
	}
	return ev
}
