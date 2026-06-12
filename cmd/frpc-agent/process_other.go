//go:build !windows

package main

import (
	"os/exec"
	"path/filepath"
)

func isFrpcRunning(exe string) bool {
	return exec.Command("pgrep", "-f", filepath.Base(exe)+".*frpc.generated.toml").Run() == nil
}

func stopFrpc(exe string) error {
	return exec.Command("pkill", "-f", filepath.Base(exe)).Run()
}

func startFrpc(exe, config string) error {
	cmd := exec.Command(exe, "-c", config)
	cmd.Dir = filepath.Dir(exe)
	return cmd.Start()
}
