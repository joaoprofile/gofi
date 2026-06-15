package toolchain

import (
	"errors"
	"testing"
)

func mkRunner(outputs map[string]string) runner {
	return func(name string, args ...string) (string, error) {
		if out, ok := outputs[name]; ok {
			return out, nil
		}
		return "", errors.New("not found")
	}
}

func TestDetect_BothPresent(t *testing.T) {
	p := detect(Needs{Go: true, Node: true}, mkRunner(map[string]string{
		"go":   "go version go1.25.0 linux/amd64",
		"node": "v22.3.0\n",
	}))
	if !p.GoOK || !p.NodeOK {
		t.Fatalf("expected both ok: %+v", p)
	}
	if len(p.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(p.Checks))
	}
}

func TestDetect_NodeMissing(t *testing.T) {
	p := detect(Needs{Go: true, Node: true}, mkRunner(map[string]string{
		"go": "go version go1.25.0 linux/amd64",
	}))
	if !p.GoOK {
		t.Errorf("Go should be ok")
	}
	if p.NodeOK {
		t.Errorf("Node should be missing")
	}
	var nodeCheck *Check
	for i := range p.Checks {
		if p.Checks[i].Name == "Node.js" {
			nodeCheck = &p.Checks[i]
		}
	}
	if nodeCheck == nil || nodeCheck.OK || nodeCheck.Hint == "" {
		t.Errorf("expected a failing Node check with a hint, got %+v", nodeCheck)
	}
}

func TestDetect_NodeNonLTSWarns(t *testing.T) {
	p := detect(Needs{Node: true}, mkRunner(map[string]string{"node": "v21.1.0"}))
	if !p.NodeOK {
		t.Fatalf("odd-major Node >= 18 is still usable")
	}
	if !p.Checks[0].Warn || p.Checks[0].Hint == "" {
		t.Errorf("expected a non-LTS warning: %+v", p.Checks[0])
	}
}

func TestDetect_NoNeedsNoChecks(t *testing.T) {
	p := detect(Needs{}, mkRunner(nil))
	if len(p.Checks) != 0 {
		t.Errorf("expected no checks when nothing is needed")
	}
	if !p.GoOK || !p.NodeOK {
		t.Errorf("unneeded toolchains default to ok")
	}
}
