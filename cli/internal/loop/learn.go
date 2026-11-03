package loop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/learn"
)

// keepLessons reads the lessons a learn turn wrote and keeps them at the
// project root, where the person accepts or rejects them: none written is
// a turn that found nothing to teach.
func (r *Runner) keepLessons(s *State) error {
	data, err := os.ReadFile(filepath.Join(r.work(), filepath.FromSlash(LessonsFile(s.Run))))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	l, err := learn.Parse(data, s.Run)
	if err != nil {
		return fmt.Errorf("gravou lições que o gofi não lê: %w", err)
	}
	if len(l.Lessons) == 0 {
		return nil
	}
	if err := learn.Save(r.Root, l); err != nil {
		return err
	}
	r.emit(s, Event{Ev: "lessons", Node: s.Agent, Why: fmt.Sprintf("%d lições propostas — gofi learn", len(l.Lessons))})
	return nil
}

// learnBrief is what a run had to fix, for the lessons proposed of it.
func learnBrief(s *State) string {
	var b strings.Builder
	fmt.Fprintf(&b, "A mudança foi aceita depois de %d voltas de retrabalho. O que a execução teve de corrigir:\n\n", s.Rounds)
	if len(s.SensorFailures) > 0 {
		var stages []string
		for st := range s.SensorFailures {
			stages = append(stages, st)
		}
		sort.Strings(stages)
		for _, st := range stages {
			fmt.Fprintf(&b, "- o sensor %s recusou o trabalho %d vez(es)\n", st, s.SensorFailures[st])
		}
	}
	for _, f := range s.Seen {
		fmt.Fprintf(&b, "- [%s] %s — %s (`%s`)\n", f.Severity, f.Rule, f.Message, f.File)
	}
	fmt.Fprintf(&b, "\nProponha o que teria poupado essas voltas, para a pessoa aceitar no knowledge do projeto: grave em `%s`, exatamente neste formato — `scope` uma de %s; `name` em kebab-case; `body` a regra e o porquê, sem nada do domínio do projeto (serviria, sem mudar uma palavra, a outro projeto do mesmo SDK); `overrides` quando corrige uma regra do gofi; `evidence` o achado ou o estágio que a ensinou:\n\n```json\n%s\n```\n\nSó o que se generaliza: sem lição que valha, `lessons` vazio. É a única escrita deste turno.\n\n",
		LessonsFile(s.Run), strings.Join(learn.Scopes, ", "), learn.Example)
	return b.String()
}
