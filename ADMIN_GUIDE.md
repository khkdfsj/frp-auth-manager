# FRP 鉴权管理系统 管理员手册

## 后台入口

- 地址：`http://<dfsj公网IP>:7500`
- 服务：`frp_auth`
- 数据库：`/www/server/frp_auth/data.db`

后台包含 Token 管理、端口配置、权限列表、IP 白名单、SSH 服务管理等页面。

## SSH 服务管理

SSH 服务管理只用于内网服务器 SSH 转发：

- 服务名称：后台展示名称，建议写目标 IP 或业务名。
- 目标内网 IP：最终要连接的服务器 IP。
- 公网端口：默认使用 `6222-6299`。
- 目标端口：固定为 `22`，后台和 agent 都会校验。
- 启用状态：关闭后 agent 会从 `frpc.generated.toml` 中移除该代理。
- 备注：记录用途、负责人或变更原因。

默认迁移的 SSH 服务：

| 公网端口 | 目标 |
| --- | --- |
| `6222` | `210.47.163.114:22` |
| `6223` | `210.47.163.113:22` |
| `6224` | `210.47.163.118:22` |
| `6225` | `210.47.163.181:22` |

`6500/6501` 是独立 AI 服务代理，不属于 SSH 服务管理范围。

## 操作流程

1. 进入后台的“SSH 服务”页面。
2. 填写服务名称、目标 IP、公网端口、启用状态和备注。
3. 点击保存。后台会同步创建或更新该端口的 Token 鉴权配置。
4. 后台自动调用 `frpc-agent` 应用配置。
5. 如状态显示 `pending`，点击“应用到 frpc”重试。

新增或修改服务时，端口默认启用 Token 鉴权。删除服务时，后台会删除该端口的鉴权配置和已有端口授权，避免端口复用时继承旧权限。

## 管理通道

管理通道固定为：

```text
dfsj:6999 -> frpc电脑 127.0.0.1:6700
```

后台启动时会保证 `6999` 的端口配置为 IP 白名单模式，并只允许 `127.0.0.1`。agent API 还会校验 `FRPC_AGENT_SECRET` HMAC 签名。

## frpc-agent

Windows 电脑上的 agent 负责：

- 作为 Windows 服务开机自启。
- 守护 `frpc.exe`。
- 读取 `frpc.base.toml`。
- 生成 `frpc.generated.toml`。
- 执行 `frpc verify`。
- 备份旧配置。
- 尝试 `frpc reload`，失败时重启 `frpc`。

agent 配置示例：

```json
{
  "listen_addr": "127.0.0.1:6700",
  "shared_secret": "<shared-secret>",
  "frpc_exe": "C:\\frp\\frp_0.67.0\\frpc.exe",
  "base_config": "frpc.base.toml",
  "generated_config": "frpc.generated.toml",
  "backup_dir": "backups",
  "frpc_admin_addr": "127.0.0.1:7400",
  "frpc_admin_user": "agent",
  "frpc_admin_password": "<frpc-admin-password>",
  "management_remote_port": 6999
}
```

## 常用命令

```bash
systemctl status frp_auth
systemctl restart frp_auth
systemctl status frps
systemctl restart frps
tail -f /frpslog/frps.log
```

Windows frpc 电脑：

```powershell
Get-Service frpc-agent
Restart-Service frpc-agent
Get-Process frpc -ErrorAction SilentlyContinue
```

## 回滚

服务端回滚：

```bash
systemctl stop frp_auth
cp /backup/frp_auth/<timestamp>/frp_auth_server /www/server/frp_auth/frp_auth_server
cp /backup/frp_auth/<timestamp>/data.db /www/server/frp_auth/data.db
systemctl start frp_auth
```

Windows frpc 回滚：

1. 停止 `frpc-agent` 服务。
2. 从 `backups` 目录恢复上一份 `frpc.generated.toml`。
3. 启动 `frpc-agent`。
4. 在后台确认 agent 状态和代理状态。
