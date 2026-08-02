package main

import (
	"sample/api"
	"sample/store"
)

func main() {
	repo := store.NewAudit(store.NewMemory())
	_ = api.Bootstrap(repo)
}
