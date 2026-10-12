//go:build !windows

package main

// Router stub so the project compiles on non-Windows hosts.
type Router struct{}

func NewRouter() *Router                       { return &Router{} }
func (r *Router) Apply([]string, string) error { return nil }
func (r *Router) Restore()                     {}
