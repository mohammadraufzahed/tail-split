//go:build !windows

package main

func (r *Router) RunCheck() string { return "Check: unsupported platform" }
