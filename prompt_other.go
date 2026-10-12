//go:build !windows

package main

import "errors"

func promptInput(title, message string) (string, error) {
	return "", errors.New("not supported on this platform")
}
