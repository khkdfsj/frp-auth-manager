# FRP 鉴权管理系统 — 管理员手册

## 系统概述

FRP 鉴权管理系统为 FRP 内网穿透服务增加了基于 Token 的端口访问控制。未经授权的用户连接会被 FRP 服务端直接拒绝，无法建立 TCP 连接。

### 架构

```
用户 → 激活 IP (curl API) → 临时白名单 → SSH 连接 → FRP → 鉴权检查 → 放行/拒绝
                                                                    ↕
                                                            Auth Manager (7500)
```

---

## 管理后台

**地址**：`http://<服务器IP>:7500`
**账号**：`dfsj`
**密码**：`K05912hk`

后台提供三个功能标签页：

### 1. Token 管理

- **创建 Token**：填写用户名称（如 `developer-zhang`），可选设置过期时间
- **Token 列表**：查看所有 Token、状态、权限，支持启用/禁用/删除

### 2. 端口配置

控制哪些 FRP 代理端口需要鉴权：

| 设置 | 效果 |
|------|------|
| 开启鉴权 | 用户必须激活 IP 才能使用该端口 |
| 关闭鉴权 | 任何 IP 可直接连接（向后兼容） |
| 未配置 | 默认放行，等同于关闭鉴权 |

### 3. 权限列表

查看所有 Token 对端口的访问权限，可单独移除某条权限。

---

## 常用操作

### 为新用户开通权限

1. 登录后台 → **Token 管理** → **Create Token**
2. 输入用户名，点击创建，复制生成的 Token（格式：`frp_xxxx...`）
3. 在 **Add Port Permission** 区域：
   - Token ID 填刚创建的 ID
   - Port 填用户需要的端口（如 6223）
   - 点击 **Add Permission**

### 禁用某个用户

1. Token 列表中找到对应用户的 Token
2. 点击 **Disable** — 用户将无法激活 IP，现有白名单也会在 5 分钟后失效

### 删除用户

1. Token 列表中找到对应用户
2. 点击 **Delete** — Token 和所有关联权限一并删除

### 开放某个端口（不需要鉴权）

1. 进入 **Port Config** 标签页
2. 找到对应端口，点击 **Delete** 删除鉴权配置
3. 或创建时选择 `No (Open Access)`

---

## 关键参数

| 参数 | 值 | 说明 |
|------|-----|------|
| IP 白名单有效期 | 5 分钟 | 用户激活后 5 分钟内可连接 |
| 过期清理间隔 | 1 分钟 | 系统每分钟清理过期白名单 |
| FRP 鉴权超时 | 3 秒 | 鉴权 API 调用超时，超时则拒绝连接 |
| 数据存储 | SQLite | `/www/server/frp_auth/data.db` |

---

## 服务管理

```bash
# 查看鉴权服务状态
systemctl status frp_auth

# 重启鉴权服务
systemctl restart frp_auth

# 查看 FRP 服务状态
systemctl status frps

# 重启 FRP 服务
systemctl restart frps

# 查看鉴权拒绝日志
tail -f /frpslog/frps.log | grep rejected
```

## 备份与恢复

备份数据库即可保存所有配置：
```bash
cp /www/server/frp_auth/data.db /backup/frp_auth_$(date +%Y%m%d).db
```

恢复：
```bash
systemctl stop frp_auth
cp /backup/frp_auth_20260515.db /www/server/frp_auth/data.db
systemctl start frp_auth
```

---

## 安全建议

1. **修改默认密码**：首次使用后立即在后台修改管理员密码
2. **限制管理面板访问**：建议配置防火墙，仅允许信任 IP 访问 7500 端口
   ```bash
   # 示例：只允许 1.2.3.4 访问管理后台
   firewall-cmd --add-rich-rule='rule family="ipv4" source address="1.2.3.4" port protocol="tcp" port="7500" accept'
   ```
3. **Token 定期轮换**：建议为 Token 设置过期时间，定期创建新 Token
4. **开启 HTTPS**：生产环境建议使用 Nginx 反向代理管理面板并配置 SSL

---

## 文件位置

| 文件 | 路径 |
|------|------|
| FRP 程序 | `/www/server/frp_0.67.0_linux_amd64/frps` |
| FRP 配置 | `/www/server/frp_0.67.0_linux_amd64/frps.toml` |
| 鉴权服务 | `/www/server/frp_auth/frp_auth_server` |
| 鉴权数据库 | `/www/server/frp_auth/data.db` |
| 鉴权服务配置 | `/etc/systemd/system/frp_auth.service` |
| FRP 服务配置 | `/etc/systemd/system/frps.service` |
| FRP 日志 | `/frpslog/frps.log` |
