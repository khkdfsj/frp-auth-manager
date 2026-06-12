# FRP SSH service runbook

Last updated: 2026-06-12 21:05 CST

This document records the current production layout for the dfsj FRP SSH management system. Secrets are intentionally omitted. Do not commit real tokens, HMAC secrets, admin passwords, or generated auth tokens.

## Current topology

- Public server: `dfsj`, public IP `140.143.209.222`.
- FRP server path: `/www/server/frp_0.67.0_linux_amd64`.
- FRP auth backend path: `/www/server/frp_auth`.
- FRP auth source path: `/www/server/frp_auth/src`.
- FRPC Windows host: `DESKTOP-BLVH1GO`.
- Managed FRPC path: `C:\frp\frp_0.67.0`.
- Managed service: `frpc-agent`.

The backend manages only SSH mappings. Target port is always `22`. Public SSH ports are currently in the `6222-6299` pool. Ports `6500` and `6501` are independent AI service proxies and are not part of SSH service management.

## Runtime ports

| Port | Side | Purpose |
| --- | --- | --- |
| `7100` | dfsj | `frps` bind port for FRP clients |
| `7500` | dfsj | FRP auth/admin backend |
| `6999` | dfsj -> frpc host | Management tunnel to `127.0.0.1:6700` on the Windows FRPC host |
| `6700` | frpc host local only | `frpc-agent` private API |
| `6222` | dfsj | SSH to `210.47.163.114:22` |
| `6223` | dfsj | SSH to `210.47.163.113:22` |
| `6224` | dfsj | SSH to `210.47.163.118:22` |
| `6225` | dfsj | SSH to `210.47.163.181:22` |
| `6226` | dfsj | SSH to `10.2.0.3:22` |
| `6500`, `6501` | dfsj | Independent AI service proxies; do not manage through SSH service UI |

## Current versions and hashes

Server side:

- `frps` version: `0.67.0`
- `/www/server/frp_0.67.0_linux_amd64/frps` SHA256: `07ffd771b2db965a329c50e9a9e6ca41a35d4b06266720574b831a1c256fa494`
- `/www/server/frp_auth/frp_auth_server` SHA256: `6d6de8458330abc51666a95e3e54e8a320fafe85f4da20b3eba49540f2232b29`
- `/www/server/frp_auth/data.db` SHA256 at documentation time: `6c86e381ed20f47e654e40739446227a84161eec554f6e7ace52025e41a0c618`

Client side:

- `frpc` version: `0.67.0`
- `C:\frp\frp_0.67.0\frpc.exe` SHA256: `4606ce1567074e102a703db6a662d23d6a13f2cbffbc054faa4e110ff7a75582`
- `C:\frp\frp_0.67.0\frpc-agent.exe` SHA256: `1d94f5c27029141413fa8d4b05b8a689389416ecb7d114abd9cbaab58acbd8e7`
- `C:\frp\frp_0.67.0\frpc.generated.toml` SHA256 at latest verification: `b8df0ae5464b976041b7629a8fbcc62a1f55417e5d9ac68e48fb57de5571b2b8`
- `frpc.generated.toml` is generated runtime state and can change after every successful apply.

## Important files

Server:

- `/etc/systemd/system/frps.service`: `frps` systemd unit.
- `/etc/systemd/system/frp_auth.service`: backend systemd unit.
- `/etc/systemd/system/frp_auth.service.d/frpc-agent.conf`: backend environment for agent URL and HMAC secret.
- `/www/server/frp_0.67.0_linux_amd64/frps.toml`: `frps` config.
- `/www/server/frp_auth/frp_auth_server`: active backend binary.
- `/www/server/frp_auth/data.db`: active SQLite database.
- `/www/server/frp_auth/src`: deployed backend and agent source.

FRPC Windows host:

- `C:\frp\frp_0.67.0\frpc-agent.exe`: Windows agent binary.
- `C:\frp\frp_0.67.0\agent.json`: agent config; contains secrets and must not be committed.
- `C:\frp\frp_0.67.0\frpc.base.toml`: base FRPC connection config; contains secrets and must not be committed.
- `C:\frp\frp_0.67.0\frpc.generated.toml`: generated active FRPC config.
- `C:\frp\frp_0.67.0\backups`: latest generated config backup.

The old `C:\frp\frp_0.67.0\frpc.toml` and the old desktop `运行部\frp` copy were removed on 2026-06-12 to prevent accidental startup of stale mappings.

## Normal service control

On dfsj:

```bash
systemctl status frps frp_auth
systemctl restart frps
systemctl restart frp_auth
ss -lntp | grep -e :7100 -e :7500 -e :6999 -e :6222 -e :6223 -e :6224 -e :6225 -e :6226
```

On the FRPC Windows host:

```powershell
Get-Service frpc-agent
Restart-Service frpc-agent -Force
Get-CimInstance Win32_Process -Filter "Name='frpc.exe' or Name='frpc-agent.exe'" |
  Select-Object ProcessId,ExecutablePath,CommandLine
Test-NetConnection 140.143.209.222 -Port 7100
Test-NetConnection 127.0.0.1 -Port 6700
Test-NetConnection 10.2.0.3 -Port 22
```

The expected FRPC process command line is:

```text
C:\frp\frp_0.67.0\frpc.exe -c C:\frp\frp_0.67.0\frpc.generated.toml
```

If the command line is `frpc.exe -c frpc.toml`, it is the old configuration and must be stopped before restarting `frpc-agent`.

## Adding or changing SSH services

Use the web admin backend on dfsj:

1. Open the admin backend at `http://140.143.209.222:7500`.
2. Add or edit an SSH service.
3. Use target IP only; target port is fixed to `22`.
4. Use public ports in `6222-6299`.
5. Apply the config.
6. Confirm the service status is `applied`.

Backend APIs:

- `GET /api/ssh-services`
- `POST /api/ssh-services`
- `PUT /api/ssh-services/{id}`
- `DELETE /api/ssh-services/{id}`
- `POST /api/ssh-services/apply`
- `GET /api/frpc-agent/status`

Agent private APIs are reachable only through the management tunnel and require HMAC signature:

- `GET /v1/status`
- `POST /v1/apply`
- `POST /v1/restart-frpc`

## Apply flow

1. Backend reads active SSH services from SQLite.
2. Backend sends signed apply request to `FRPC_AGENT_URL`, currently `http://127.0.0.1:6999`.
3. `frps` forwards `6999` to the Windows host agent at `127.0.0.1:6700`.
4. Agent validates HMAC signature.
5. Agent writes temporary generated config.
6. Agent runs `frpc verify`.
7. Agent backs up previous generated config.
8. Agent replaces `frpc.generated.toml`.
9. Agent calls `frpc reload` or restarts `frpc` if needed.
10. Backend records `applied` or `failed`.

## Health checks

Server port check:

```bash
ss -lntp | grep -e :6999 -e :6222 -e :6223 -e :6224 -e :6225 -e :6226
```

Agent tunnel check from dfsj:

```bash
curl -i -m 3 http://127.0.0.1:6999/v1/status
```

Expected unauthenticated result is `401` with `missing signature`. That means the tunnel reaches the agent.

Public port check from any external machine:

```powershell
Test-NetConnection 140.143.209.222 -Port 6226
```

## Backup and release procedure

Before deployment:

```bash
backup_dir=/www/server/frp_auth/backups/$(date +%Y%m%d-%H%M%S)
mkdir -p "$backup_dir"
cp -a /www/server/frp_auth/frp_auth_server /www/server/frp_auth/data.db /www/server/frp_auth/src "$backup_dir"/
```

Build and test should be run from the GitHub source repository when Go is available:

```bash
go test ./...
go build -o frp_auth_server .
GOOS=windows GOARCH=amd64 go build -o frpc-agent.exe ./cmd/frpc-agent
```

Deploy backend:

```bash
systemctl stop frp_auth
cp frp_auth_server /www/server/frp_auth/frp_auth_server
chmod +x /www/server/frp_auth/frp_auth_server
systemctl start frp_auth
systemctl status frp_auth
```

Deploy agent:

```powershell
Stop-Service frpc-agent
Copy-Item .\frpc-agent.exe C:\frp\frp_0.67.0\frpc-agent.exe -Force
Start-Service frpc-agent
Get-Service frpc-agent
```

After deployment:

1. Verify backend login.
2. Verify `GET /api/frpc-agent/status` shows `online: true`.
3. Apply SSH services once.
4. Verify ports `6999` and `6222-6226` are listening on dfsj.
5. Verify an external TCP connection to the expected public SSH port.
6. Commit and push GitHub repository `khkdfsj/frp-auth-manager`.

## Rollback

Backend rollback:

```bash
systemctl stop frp_auth
cp /path/to/backup/frp_auth_server /www/server/frp_auth/frp_auth_server
cp /path/to/backup/data.db /www/server/frp_auth/data.db
systemctl start frp_auth
```

Agent config rollback:

```powershell
Copy-Item C:\frp\frp_0.67.0\backups\frpc.generated.20260612-210329.toml C:\frp\frp_0.67.0\frpc.generated.toml -Force
Restart-Service frpc-agent -Force
```

## Current cleanup state

As of 2026-06-12 21:01 CST, the FRPC Windows host keeps only:

- `agent.json`
- `frpc-agent.exe`
- `frpc.base.toml`
- `frpc.exe`
- `frpc.generated.toml`
- `frpc.log`
- `backups\frpc.generated.20260612-210329.toml`

The old `frpc.toml`, old desktop copy, and old migration/debug backups were removed.
