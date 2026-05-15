# 运行部内网管理系统 — 迁移部署指南

## 项目文件位置

当前服务器上所有相关文件：

```
/www/server/
├── frp_0.67.0_linux_amd64/          # FRP 穿透服务
│   ├── frps                          # 修改后的 FRP 服务端（含鉴权钩子）
│   ├── frps.toml                     # FRP 配置文件
│   └── LICENSE
│
├── frp_auth/                         # 鉴权管理系统
│   ├── frp_auth_server               # 鉴权服务二进制
│   ├── data.db                       # SQLite 数据库（所有配置/Token/权限都在这里）
│   ├── ADMIN_GUIDE.md                # 管理员手册
│   ├── USER_GUIDE.md                 # 用户使用指南
│   └── src/                          # 源码（用于二次开发）
│       ├── main.go
│       ├── go.mod / go.sum
│       ├── database/
│       ├── handlers/
│       ├── middleware/
│       └── models/
│
/etc/systemd/system/
├── frps.service                      # FRP 服务定义
└── frp_auth.service                  # 鉴权服务定义

/frpslog/frps.log                     # FRP 运行日志
```

---

## 迁移到新服务器（完整步骤）

### 前提条件

新服务器需要：
- Linux x86_64 系统（CentOS/Ubuntu/OpenCloudOS 等）
- 已安装 ssh、curl、tar
- 如果有 Go 1.22+ 环境最佳（用于二次开发），纯运行不需要

---

### 第一步：打包旧服务器数据

在旧服务器上执行：

```bash
# 打包整个项目
cd /www/server
tar -czf /tmp/frp_migration.tar.gz \
  frp_0.67.0_linux_amd64/frps \
  frp_0.67.0_linux_amd64/frps.toml \
  frp_auth/frp_auth_server \
  frp_auth/data.db \
  frp_auth/src/

# 下载到本地
scp root@旧服务器IP:/tmp/frp_migration.tar.gz ./

# 同时复制 systemd 服务文件
scp root@旧服务器IP:/etc/systemd/system/frps.service ./
scp root@旧服务器IP:/etc/systemd/system/frp_auth.service ./
```

---

### 第二步：上传到新服务器

```bash
# 上传打包文件
scp frp_migration.tar.gz root@新服务器IP:/tmp/

# 上传 systemd 服务文件
scp frps.service frp_auth.service root@新服务器IP:/tmp/
```

---

### 第三步：在新服务器上部署

```bash
# SSH 到新服务器
ssh root@新服务器IP

# 创建目录结构
mkdir -p /www/server/frp_0.67.0_linux_amd64
mkdir -p /www/server/frp_auth
mkdir -p /frpslog

# 解压项目文件
cd /www/server
tar -xzf /tmp/frp_migration.tar.gz

# 赋予执行权限
chmod +x /www/server/frp_0.67.0_linux_amd64/frps
chmod +x /www/server/frp_auth/frp_auth_server

# 安装 systemd 服务
cp /tmp/frps.service /etc/systemd/system/
cp /tmp/frp_auth.service /etc/systemd/system/

# 修改鉴权服务中的 FRP Dashboard 连接密码（如果新服务器 FRP 密码不同）
# 编辑 /etc/systemd/system/frp_auth.service，修改：
#   Environment="FRP_DASHBOARD_USER=你的用户名"
#   Environment="FRP_DASHBOARD_PASS=你的密码"

systemctl daemon-reload

# 开启开机自启
systemctl enable frps frp_auth

# 启动服务
systemctl start frp_auth   # 先启鉴权（FRP 依赖它做鉴权检查）
sleep 2
systemctl start frps

# 检查状态
systemctl status frp_auth frps
```

---

### 第四步：验证

```bash
# 检查端口监听
ss -tlnp | grep -E '7100|7500'

# 应该看到：
# 7100  -> frps（FRP 客户端连接端口）
# 7500  -> frp_auth_server（管理面板 + 鉴权 API）

# 登录管理后台
curl -X POST http://localhost:7500/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"dfsj","password":"K05912hk"}'

# 检查 FRP 代理
curl http://localhost:7500/api/frp/proxy/tcp \
  -H "Authorization: Bearer <上一步返回的token>"
```

---

### 第五步：防火墙配置

```bash
# 开放必要端口
firewall-cmd --add-port=7500/tcp --permanent   # 管理面板
firewall-cmd --add-port=7100/tcp --permanent   # FRP 客户端
firewall-cmd --add-port=6000-7000/tcp --permanent  # FRP 代理端口范围
firewall-cmd --reload

# 或使用 iptables
iptables -A INPUT -p tcp --dport 7500 -j ACCEPT
iptables -A INPUT -p tcp --dport 7100 -j ACCEPT
iptables -A INPUT -p tcp --dport 6000:7000 -j ACCEPT
```

---

## 关键说明

### 数据持久化

**所有业务数据存储在 `/www/server/frp_auth/data.db`** 这一个 SQLite 文件中，包括：
- 管理员账户
- 所有 Token 及其权限
- 端口鉴权配置
- IP 白名单历史记录

只要这个文件不丢失，所有配置都在。

### 备份建议

```bash
# 设置每日自动备份（crontab）
0 3 * * * cp /www/server/frp_auth/data.db /backup/frp_auth_$(date +\%Y\%m\%d).db
```

### 二次开发

如需修改源码：

```bash
cd /www/server/frp_auth/src
go mod tidy
go build -o ../frp_auth_server .
systemctl restart frp_auth
```

---

## 快速排查

| 问题 | 检查 |
|------|------|
| 管理面板打不开 | `systemctl status frp_auth` |
| FRP 代理不通 | `systemctl status frps` |
| 鉴权不生效 | 确认 frps.toml 中 `proxyAuthUrl = "http://127.0.0.1:7500/api/auth/check"` |
| 代理状态页无数据 | 确认 `/etc/systemd/system/frp_auth.service` 中 FRP_DASHBOARD_USER/PASS 与 frps.toml 一致 |
