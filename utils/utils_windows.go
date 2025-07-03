//go:build windows
// +build windows

package utils

func IsExecutable(absPath string) *bool {
	return nil
}

func SetExecutable(absPath string, executable bool) error {
	return nil
}
