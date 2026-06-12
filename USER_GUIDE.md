# FRP 端口使用指南

## 使用方式

管理员会提供一个 Token 和一个公网端口。连接前先激活当前公网 IP，然后在 5 分钟内连接 SSH。

### 激活 IP

Linux/macOS：

```bash
curl -s -X POST http://<dfsj公网IP>:7500/api/user/activate \
  -H "Content-Type: application/json" \
  -d '{"token":"你的token","port":6222}'
```

Windows PowerShell：

```powershell
Invoke-RestMethod -Uri "http://<dfsj公网IP>:7500/api/user/activate" `
  -Method POST `
  -ContentType "application/json" `
  -Body '{"token":"你的token","port":6222}'
```

### 连接 SSH

```bash
ssh -p 6222 <用户名>@<dfsj公网IP>
```

## 当前默认 SSH 端口

| 公网端口 | 目标服务器 |
| --- | --- |
| `6222` | `210.47.163.114` |
| `6223` | `210.47.163.113` |
| `6224` | `210.47.163.118` |
| `6225` | `210.47.163.181` |

具体端口可能由管理员调整，以后台配置为准。

## 常见问题

### 提示 `token does not have permission for this port`

Token 没有该端口权限，请联系管理员添加端口授权。

### 提示 `token is disabled`

Token 已被禁用，请联系管理员。

### 换网络后无法连接

授权绑定的是当前公网 IP。更换 Wi-Fi、运营商网络或代理后，需要重新激活。

### 激活超过 5 分钟后会断开吗

不会。鉴权只发生在建立连接时。已经建立的 SSH 连接不会因为白名单过期而主动断开；重新连接需要再次激活。

## 提供给管理员的信息

排查问题时请提供：

- Token 前缀，不要发送完整 Token。
- 公网端口。
- 激活接口返回的错误信息。
- 当前公网 IP。
