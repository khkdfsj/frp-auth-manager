@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%USERPROFILE%\bin\frp-ssh.ps1" 10.2.0.3 %*
