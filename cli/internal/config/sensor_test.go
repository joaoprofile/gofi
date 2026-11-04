package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A stage of loop.sensors is its command alone, or {run, format, report};
// one written as a command is written back as it.
func TestASensorIsACommandOrAReport(t *testing.T) {
	var l Loop
	src := "sensors:\n  build: make build\n  security: {run: horusec start, format: sarif, report: out.sarif}\n"
	if err := yaml.Unmarshal([]byte(src), &l); err != nil {
		t.Fatal(err)
	}
	if l.Sensors["build"] != (Sensor{Run: "make build"}) {
		t.Errorf("build = %+v", l.Sensors["build"])
	}
	if l.Sensors["security"] != (Sensor{Run: "horusec start", Format: "sarif", Report: "out.sarif"}) {
		t.Errorf("security = %+v", l.Sensors["security"])
	}
	out, err := yaml.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "build: make build\n") || !strings.Contains(string(out), "report: out.sarif") {
		t.Errorf("marshalled:\n%s", out)
	}
}
