//gofi:context storage
package store

// Repo is the data output port.
type Repo interface {
	Get(id string) (string, error)
	Put(id, v string) error
}

// Memory keeps everything in memory.
type Memory struct{ m map[string]string }

func NewMemory() *Memory { return &Memory{m: map[string]string{}} }

func (s *Memory) Get(id string) (string, error) { return s.m[id], nil }
func (s *Memory) Put(id, v string) error        { s.m[id] = v; return nil }

// Audit wraps another Repo. It also satisfies Repo.
//
//gofi:context audit
type Audit struct{ inner Repo }

func NewAudit(inner Repo) *Audit               { return &Audit{inner: inner} }
func (a *Audit) Get(id string) (string, error) { return a.inner.Get(id) }
func (a *Audit) Put(id, v string) error        { return a.inner.Put(id, v) }
