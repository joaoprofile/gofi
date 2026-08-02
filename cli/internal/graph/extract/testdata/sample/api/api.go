package api

import "sample/store"

// Server exposes the API.
type Server struct{ repo store.Repo }

func NewServer(r store.Repo) *Server { return &Server{repo: r} }

// Start brings the server up.
func (s *Server) Start() error { _, err := s.repo.Get("hello"); return err }

// Worker also has a Start: the case that confuses the syntactic heuristic.
type Worker struct{ n int }

func (w *Worker) Start() error { return nil }

// par and impar are mutually recursive.
func par(n int) bool {
	if n == 0 {
		return true
	}
	return impar(n - 1)
}

func impar(n int) bool {
	if n == 0 {
		return false
	}
	return par(n - 1)
}

func Bootstrap(r store.Repo) error {
	s := NewServer(r)
	w := &Worker{n: 1}
	if err := w.Start(); err != nil {
		return err
	}
	_ = par(4)
	return s.Start()
}
