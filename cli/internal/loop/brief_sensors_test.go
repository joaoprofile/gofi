package loop

import (
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/role"
)

// An auditor is told the sensors passed on the tree it reads, so it does not
// run them again; a turn that fixes, or a run whose sensors failed, is not.
func TestAnAuditorIsToldTheSensorsPassed(t *testing.T) {
	audit := role.Role{Name: "gofi-qa", Contract: role.Contract{Stage: "audit", Tools: []string{"read", "verify", "verdict"}}}
	s := &State{Run: "r-1", Change: "c", Context: "x", Iter: 2, Sensor: &SensorResult{Stages: []string{"integrity", "build", "static", "test", "trace", "security"}}}
	got := DefaultBrief(audit, s, "e.json")
	if !strings.Contains(got, "Os sensores build, static, test passaram") || !strings.Contains(got, "`trace`") {
		t.Errorf("the auditor is not told:\n%s", got)
	}
	if strings.Contains(got, "security passaram") || strings.Contains(got, "integrity,") {
		t.Errorf("only what an auditor would run again is named:\n%s", got)
	}
	eng := role.Role{Name: "gofi-eng", Contract: role.Contract{Stage: "implement", Tools: []string{"read", "edit", "verify"}}}
	if strings.Contains(DefaultBrief(eng, s, "e.json"), "passaram há pouco") {
		t.Error("a turn that changes the code must check it")
	}
	s.Sensor.Failed = []string{"test"}
	if strings.Contains(DefaultBrief(audit, s, "e.json"), "passaram há pouco") {
		t.Error("sensors that failed did not pass")
	}
}
