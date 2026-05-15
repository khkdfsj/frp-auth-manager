# FRP 端口使用指南（用户版）

## 概述

出于安全考虑，服务器 FRP 内网穿透端口已启用访问鉴权。连接前需要先激活您的 IP 地址。

---

## 快速开始

### 1. 获取您的 Token

联系管理员索取您的专属 Token（格式：`frp_xxxx...`）。

### 2. 激活您的 IP

在终端执行以下命令（**每次连接前都需要执行**）：

**Linux / macOS：**
```bash
curl -s -X POST http://140.143.209.222:7500/api/user/activate \
  -H "Content-Type: application/json" \
  -d '{"token":"你的token","port":端口号}'
```

**Windows PowerShell：**
```powershell
$response = Invoke-RestMethod -Uri "http://140.143.209.222:7500/api/user/activate" -Method POST -ContentType "application/json" -Body '{"token":"你的token","port":端口号}'
$response
```

### 3. 连接服务器

激活成功后，在 **5 分钟内** 正常连接即可：

```bash
ssh -p 端口号 用户名@140.143.209.222
```

---

## 端口对照表

| 端口 | 目标服务器 |
|------|-----------|
| 6222 | 210.47.163.114 |
| 6223 | 210.47.163.113 |
| 6224 | 210.47.163.118 |
| 6225 | 210.47.163.181 |
| 6121 | 210.47.163.181 (phpMyAdmin) |
| 6122 | 210.47.163.118 (phpMyAdmin) |
| 6123 | 210.47.163.113 (phpMyAdmin) |

> 具体端口对应的服务器可能调整，以管理员通知为准。

---

## 一键连接脚本

将以下内容保存为 `connect.sh`，赋予执行权限后使用：

```bash
#!/bin/bash
# 用法: ./connect.sh <端口号>
# 示例: ./connect.sh 6223

TOKEN="将这里替换为你的token"
SERVER="将这里替换为服务器IP"

if [ -z "$1" ]; then
    echo "用法: $0 <端口号>"
    echo "示例: $0 6223"
    exit 1
fi

echo "正在激活端口 $1 的访问权限..."
RESP=$(curl -s -X POST "http://$SERVER:7500/api/user/activate" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$TOKEN\",\"port\":$1}")

SUCCESS=$(echo "$RESP" | grep -o '"success":true')

if [ -z "$SUCCESS" ]; then
    MSG=$(echo "$RESP" | grep -o '"message":"[^"]*"')
    echo "激活失败: $MSG"
    exit 1
fi

echo "激活成功，正在连接端口 $1 ..."
ssh -p "$1" root@"$SERVER"
```

使用方法：
```bash
chmod +x connect.sh
./connect.sh 6223
```

---

## 常见问题

### Q: 激活时提示 "token does not have permission for this port"
**A**：您的 Token 没有被授权访问该端口，请联系管理员。

### Q: 激活时提示 "token is disabled"
**A**：您的 Token 已被管理员禁用，请联系管理员。

### Q: 激活时提示 "token has expired"
**A**：您的 Token 已过期，请联系管理员获取新 Token。

### Q: 为什么连接超过 5 分钟就断了？
**A**：IP 授权有效期为 5 分钟。连接建立后不会中断（只检查连接建立时），但如果您断开后想重新连接，需要重新激活 IP。

### Q: 换个网络/Wi-Fi 后需要重新激活吗？
**A**：需要。授权绑定的是您的公网 IP，切换网络后 IP 会变化，必须重新激活。

### Q: 如何在手机/平板上使用？
**A**：先用手机浏览器访问以下地址激活：
```
http://140.143.209.222:7500/api/user/activate
```
然后用 POST 请求工具（如 HTTP Request Shortcuts 等 App）发送同样的 JSON 请求体。

---

## 技术支持

如遇到问题，请提供以下信息联系管理员：
1. 您的 Token（部分即可，如 `frp_41407438...`）
2. 目标端口号
3. 激活时的完整错误信息
4. 您的当前公网 IP（访问 https://ip.sb 获取）
