//go:build !windows

package main

func maybeRunWindowsService(configPath string) (bool, error) {
	return false, nil
}
