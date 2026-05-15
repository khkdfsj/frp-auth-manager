# 运行部内网管理系统

基于 FRP v0.67.0 的内网穿透鉴权管理系统。为 FRP TCP 代理提供基于 Token+IP 白名单的鉴权机制，并集成中文管理后台。

## 功能特性

- **端口鉴权控制**：精细控制每个 FRP 代理端口是否需要鉴权
- **Token 管理**：创建、编辑、删除用户 Token，支持过期时间、流量限制
- **IP 白名单**：用户通过 Token 激活 IP，5 分钟内有效（适配 SSH/TCP 连接）
- **管理后台**：集成原 FRP Dashboard 的代理监控 + 鉴权管理，全中文界面
- **单一二进制部署**：Go 编译产物，无外部依赖，SQLite 存储

## 系统架构

```
用户 SSH 连接 → FRP Server (frps) → Auth Check API → frp_auth_server
                                                    ↓
                                               SQLite (data.db)
```

## 快速开始

### 编译

```bash
cd src
go mod tidy
CGO_ENABLED=0 go build -o ../frp_auth_server .
```

### 运行

```bash
AUTH_LISTEN_ADDR=0.0.0.0:7500 \
AUTH_DB_PATH=/www/server/frp_auth/data.db \
AUTH_ADMIN_USER=admin \
AUTH_ADMIN_PASS=yourpassword \
./frp_auth_server
```

### FRP 配置

在 `frps.toml` 中添加：

```toml
proxyAuthUrl = "http://127.0.0.1:7500/api/auth/check"
```

## 文档

- [管理员手册](ADMIN_GUIDE.md)
- [用户使用指南](USER_GUIDE.md)
- [迁移部署指南](MIGRATION_GUIDE.md)

## 项目结构

```
frp_auth/
├── main.go            # 入口 + 嵌入式管理面板
├── database/          # SQLite 数据库层
├── handlers/          # API 处理器
├── middleware/        # 中间件（会话管理）
├── models/            # 数据结构
├── go.mod / go.sum   # Go 模块定义
├── ADMIN_GUIDE.md    # 管理员手册
├── USER_GUIDE.md     # 用户指南
└── MIGRATION_GUIDE.md # 迁移部署指南
```

## 技术栈

- Go 1.22+
- SQLite (modernc.org/sqlite)
- FRP v0.67.0（需配合修改版 frps 使用）
