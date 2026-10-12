//go:build !windows

package main

func isAdmin() bool           { return true }
func relaunchElevated() error { return nil }
