# 迁移、发布与备份手册

## 发布原则

所有变更按同一条链路执行：

1. 本地仓库 `C:\Users\dell\frp-auth-manager` 修改代码。
2. 运行 `go test ./...`。
3. 构建 Linux 服务端和 Windows agent。
4. 部署前备份服务器源码、二进制和 `data.db`。
5. 部署到 `dfsj` 和 Windows frpc 电脑。
6. 验证后台、agent、FRP 代理和端口鉴权。
7. 提交 Git 并推送到 GitHub 私有仓库 `khkdfsj/frp-auth-manager`。

## 构建

```bash
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o frp_auth_server .
GOOS=windows GOARCH=amd64 go build -o frpc-agent.exe ./cmd/frpc-agent
```

## dfsj 服务端部署

关键路径：

```text
/www/server/frp_auth/src
/www/server/frp_auth/frp_auth_server
/www/server/frp_auth/data.db
/etc/systemd/system/frp_auth.service
```

部署前备份：

```bash
ts=$(date +%Y%m%d-%H%M%S)
mkdir -p /backup/frp_auth/$ts
cp -a /www/server/frp_auth/src /backup/frp_auth/$ts/src
cp -a /www/server/frp_auth/frp_auth_server /backup/frp_auth/$ts/frp_auth_server
cp -a /www/server/frp_auth/data.db /backup/frp_auth/$ts/data.db
```

部署：

```bash
rsync -a --delete ./ /www/server/frp_auth/src/
cd /www/server/frp_auth/src
go test ./...
CGO_ENABLED=0 go build -o ../frp_auth_server .
systemctl daemon-reload
systemctl restart frp_auth
systemctl status frp_auth --no-pager
```

`frp_auth.service` 需要包含：

```ini
Environment="FRPC_AGENT_URL=http://127.0.0.1:6999"
Environment="FRPC_AGENT_SECRET=<shared-secret>"
```

## Windows frpc-agent 部署

frpc 目录：

```text
C:\frp\frp_0.67.0
```

文件职责：

- `frpc.exe`：FRP 客户端。
- `frpc.base.toml`：手工维护的基础连接信息和不属于 SSH 管理的代理。
- `frpc.generated.toml`：agent 生成配置，不手工修改。
- `agent.json`：agent 配置。
- `backups\`：agent 自动备份旧生成配置。

安装服务：

```powershell
sc.exe create frpc-agent binPath= '"C:\frp\frp_0.67.0\frpc-agent.exe" -config "C:\frp\frp_0.67.0\agent.json"' start= auto
sc.exe failure frpc-agent reset= 60 actions= restart/5000/restart/5000/restart/5000
sc.exe start frpc-agent
```

更新服务：

```powershell
Stop-Service frpc-agent
Copy-Item .\frpc-agent.exe "C:\frp\frp_0.67.0\frpc-agent.exe" -Force
Start-Service frpc-agent
```

## 初次迁移

1. 备份原 `frpc.toml`。
2. 将基础连接信息和非 SSH 代理保留到 `frpc.base.toml`。
3. 将原有 `6222-6225` 迁移为后台 SSH 服务：
   - `6222 -> 210.47.163.114:22`
   - `6223 -> 210.47.163.113:22`
   - `6224 -> 210.47.163.118:22`
   - `6225 -> 210.47.163.181:22`
4. 创建初始 `frpc.generated.toml`，包含管理通道 `6999 -> 127.0.0.1:6700`。
5. 启动 `frpc-agent`，确认后台 `/api/frpc-agent/status` 在线。
6. 在后台点击“应用到 frpc”，让后续配置进入标准流程。

## 验证

服务端：

```bash
systemctl status frp_auth --no-pager
curl -s http://127.0.0.1:7500/api/frpc-agent/status
ss -lntp | grep -E ':6999|:6222|:6223|:6224|:6225'
```

Windows：

```powershell
Get-Service frpc-agent
Get-Process frpc -ErrorAction SilentlyContinue
```

业务验证：

- 未激活 Token 时，SSH 连接被拒绝。
- 激活 Token 后，对应公网端口可连接。
- 修改 IP 或端口后，旧映射下线，新映射上线。
- 删除服务后，对应公网端口停止监听。

## 回滚

dfsj：

```bash
systemctl stop frp_auth
cp /backup/frp_auth/<timestamp>/frp_auth_server /www/server/frp_auth/frp_auth_server
cp /backup/frp_auth/<timestamp>/data.db /www/server/frp_auth/data.db
rm -rf /www/server/frp_auth/src
cp -a /backup/frp_auth/<timestamp>/src /www/server/frp_auth/src
systemctl start frp_auth
```

Windows：

```powershell
Stop-Service frpc-agent
Copy-Item .\backups\<previous-generated>.toml .\frpc.generated.toml -Force
Start-Service frpc-agent
```
