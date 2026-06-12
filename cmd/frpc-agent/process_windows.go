//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func isFrpcRunning(exe string) bool {
	name := strings.ToLower(filepath.Base(exe))
	cmd := exec.Command("powershell", "-NoProfile", "-Command", fmt.Sprintf(
		`$p=Get-CimInstance Win32_Process | Where-Object { $_.Name -ieq %q -and $_.CommandLine -like '*frpc.generated.toml*' }; if ($p) { exit 0 } else { exit 1 }`, name))
	return cmd.Run() == nil
}

func stopFrpc(exe string) error {
	name := strings.ToLower(filepath.Base(exe))
	cmd := exec.Command("powershell", "-NoProfile", "-Command", fmt.Sprintf(
		`Get-CimInstance Win32_Process | Where-Object { $_.Name -ieq %q } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }`, name))
	return cmd.Run()
}

func startFrpc(exe, config string) error {
	cmd := exec.Command(exe, "-c", config)
	cmd.Dir = filepath.Dir(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
