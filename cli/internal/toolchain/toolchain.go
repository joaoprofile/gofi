// Package toolchain runs a lightweight preflight before `gofi init` creates
// the selected surfaces. It DETECTS the required toolchains (Go for a Go
// backend, Node.js LTS for web/mobile) and produces human-facing guidance —
// it never installs anything. Surfaces whose toolchain is missing are skipped
// by the caller, which keeps the rest of init flowing.
package toolchain

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Needs declares which toolchains the surfaces selected in the wizard require.
type Needs struct {
	Go   bool // a Go backend was selected
	Node bool // a web and/or mobile surface was selected
}

// Check is the outcome of probing one toolchain.
type Check struct {
	Name    string // "Go", "Node.js"
	OK      bool   // present and usable
	Version string // detected version, when present (e.g. "1.25.0", "22.3.0")
	Hint    string // install guidance, set when !OK or a soft warning applies
	Warn    bool   // present but sub-optimal (e.g. Node present but not LTS)
}

// Preflight is the result of a detection pass.
type Preflight struct {
	Checks []Check
	GoOK   bool // Go usable (or not needed)
	NodeOK bool // Node usable (or not needed)
}

const (
	nodeDownloadURL = "https://nodejs.org/en/download (LTS)"
	goDownloadURL   = "https://go.dev/dl"
	// minNodeMajor is the lowest Node major we treat as usable; current LTS
	// lines are 18/20/22 (even majors). Odd majors are "Current", not LTS.
	minNodeMajor = 18
)

// runner runs a command and returns its combined stdout. Injectable for tests.
type runner func(name string, args ...string) (string, error)

func defaultRunner(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

// Detect probes the toolchains required by needs using the real environment.
func Detect(needs Needs) Preflight { return detect(needs, defaultRunner) }

func detect(needs Needs, run runner) Preflight {
	p := Preflight{GoOK: !needs.Go, NodeOK: !needs.Node}

	if needs.Go {
		c := Check{Name: "Go"}
		if v, ok := probeGo(run); ok {
			c.OK, c.Version = true, v
			p.GoOK = true
		} else {
			c.Hint = "instale o Go antes de criar o backend Go — " + goDownloadURL
		}
		p.Checks = append(p.Checks, c)
	}

	if needs.Node {
		c := Check{Name: "Node.js"}
		if v, major, ok := probeNode(run); ok {
			c.Version = v
			if major >= minNodeMajor {
				c.OK = true
				p.NodeOK = true
				if major%2 != 0 {
					c.Warn = true
					c.Hint = "versão Current (não-LTS) — recomendado o Node.js LTS: " + nodeDownloadURL
				}
			} else {
				c.Hint = "Node.js " + v + " é muito antigo; instale o Node.js LTS — " + nodeDownloadURL
			}
		} else {
			c.Hint = "instale o Node.js LTS antes de criar web/mobile — " + nodeDownloadURL
		}
		p.Checks = append(p.Checks, c)
	}

	return p
}

var (
	goVersionRe   = regexp.MustCompile(`go([0-9]+(?:\.[0-9]+){1,2})`)
	nodeVersionRe = regexp.MustCompile(`v?([0-9]+)\.([0-9]+)\.([0-9]+)`)
)

// probeGo returns the Go toolchain version from `go version`, or ok=false.
func probeGo(run runner) (string, bool) {
	out, err := run("go", "version")
	if err != nil {
		return "", false
	}
	if m := goVersionRe.FindStringSubmatch(out); m != nil {
		return m[1], true
	}
	return "", false
}

// probeNode returns the Node version string and major from `node --version`,
// or ok=false when node is absent or unparseable.
func probeNode(run runner) (version string, major int, ok bool) {
	out, err := run("node", "--version")
	if err != nil {
		return "", 0, false
	}
	m := nodeVersionRe.FindStringSubmatch(strings.TrimSpace(out))
	if m == nil {
		return "", 0, false
	}
	maj, err := strconv.Atoi(m[1])
	if err != nil {
		return "", 0, false
	}
	return m[1] + "." + m[2] + "." + m[3], maj, true
}
