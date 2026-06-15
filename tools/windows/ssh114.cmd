@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%USERPROFILE%\bin\frp-ssh.ps1" 114 %*
