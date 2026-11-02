package app

import "example.com/lib"

// Run uses the sibling module.
func Run() string { return lib.Name() }
