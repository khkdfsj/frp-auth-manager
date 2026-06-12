# 运行部内网管理系统

基于 FRP v0.67.0 的内网穿透鉴权与 SSH 服务管理系统。服务端运行在 `dfsj` 公网服务器，负责 Token/IP 鉴权、FRP Dashboard 聚合展示、SSH 服务配置管理；机房 Windows 电脑运行 `frpc-agent`，负责生成并应用 `frpc` 客户端配置。

## 功能

- 端口鉴权：支持 `open`、`token`、`ip` 三种模式。
- Token 管理：创建、禁用、删除 Token，并给 Token 分配端口权限。
- IP 临时白名单：用户激活 Token 后，当前公网 IP 在 5 分钟内可访问授权端口。
- SSH 服务管理：后台新增、修改、删除 SSH 服务，目标端口固定为 `22`，公网端口池为 `6222-6299`。
- frpc-agent：Windows 服务开机自启，守护 `frpc.exe`，通过 HMAC 校验的本地 API 接收配置并执行 `verify/reload/restart`。
- 标准化发布：本地仓库、服务器源码、GitHub 私有仓库保持同步。

## 架构

```text
用户 SSH -> dfsj:6222-6299 -> frps -> frpc -> 内网服务器:22
                 |
                 +-> frp_auth_server:7500 鉴权

后台 SSH 服务管理 -> dfsj:6999 -> frpc-agent:6700 -> frpc.generated.toml
```

`6999` 是管理通道端口，仅允许 `127.0.0.1` 后台本机访问，不对公网用户开放。

## 快速构建

```bash
go test ./...
CGO_ENABLED=0 go build -o frp_auth_server .
GOOS=windows GOARCH=amd64 go build -o frpc-agent.exe ./cmd/frpc-agent
```

## 服务端环境变量

```bash
AUTH_LISTEN_ADDR=0.0.0.0:7500
AUTH_DB_PATH=/www/server/frp_auth/data.db
AUTH_ADMIN_USER=dfsj
AUTH_ADMIN_PASS=<admin-password>
FRPC_AGENT_URL=http://127.0.0.1:6999
FRPC_AGENT_SECRET=<shared-secret>
```

## 关键路径

- 本地仓库：`C:\Users\dell\frp-auth-manager`
- 服务器源码：`/www/server/frp_auth/src`
- 服务器二进制：`/www/server/frp_auth/frp_auth_server`
- 服务器数据库：`/www/server/frp_auth/data.db`
- Windows frpc 目录：`C:\frp\frp_0.67.0`
- GitHub 私有仓库：`khkdfsj/frp-auth-manager`

## 文档

- [管理员手册](ADMIN_GUIDE.md)
- [用户手册](USER_GUIDE.md)
- [迁移与发布手册](MIGRATION_GUIDE.md)
