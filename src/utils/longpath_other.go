//go:build !windows

package utils

func LongPath(p string) string { return p }
