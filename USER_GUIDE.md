# FRP SSH 用户使用指南

本文说明如何使用 FRP SSH 端口访问内网服务器，以及如何配置成类似 `ssh114`、`ssh3`、`ssh102` 这种“自动敲门 + 免密登录”的便捷方式。

文档中的 token、密码、服务器账号都使用占位符。不要把真实 token、服务器密码、私钥或 `authorized_keys` 内容提交到 GitHub。

## 基本概念

访问受保护的 SSH 端口需要两层条件：

1. FRP 访问放行：先调用 `/api/user/activate`，把当前公网 IP 临时加入目标端口白名单。
2. SSH 登录认证：目标服务器仍然需要密码或 SSH 公钥认证。

也就是说，FRP token 只负责“能不能连到端口”，不能替代目标服务器的 SSH 密码。要做到免密登录，需要把本机公钥写入目标服务器的 `~/.ssh/authorized_keys`。

## 当前 SSH 服务端口

当前生产环境的公网入口是 `140.143.209.222`。

| 快捷名 | 公网端口 | 目标服务器 | 目标端口 |
| --- | --- | --- | --- |
| `114` | `6222` | `210.47.163.114` | `22` |
| `113` | `6223` | `210.47.163.113` | `22` |
| `118` | `6224` | `210.47.163.118` | `22` |
| `181` | `6225` | `210.47.163.181` | `22` |
| `3` | `6226` | `10.2.0.3` | `22` |
| `102` | `6227` | `10.2.0.102` | `22` |

`6500/6501` 是独立 AI 服务代理，不属于 SSH 服务管理范围。

## 最基础的使用方式

### Windows PowerShell

```powershell
$response = Invoke-RestMethod `
  -Uri "http://140.143.209.222:7500/api/user/activate" `
  -Method POST `
  -ContentType "application/json" `
  -Body '{"token":"<YOUR_FRP_TOKEN>","port":6222}'

$response
ssh -p 6222 root@140.143.209.222
```

### macOS / Linux

```bash
curl -sS -X POST "http://140.143.209.222:7500/api/user/activate" \
  -H "Content-Type: application/json" \
  -d '{"token":"<YOUR_FRP_TOKEN>","port":6222}'

ssh -p 6222 root@140.143.209.222
```

如果切换网络、代理、Wi-Fi、运营商出口，公网 IP 会变化，需要重新 activate。

## 配置免密登录

以下步骤只需要做一次。每台目标服务器都要安装一次公钥。

### 1. 生成本机 SSH 密钥

Windows PowerShell、macOS、Linux 都可以使用同一条命令：

```bash
ssh-keygen -t ed25519 -C "your-name@your-device"
```

一路回车会生成默认密钥：

```text
~/.ssh/id_ed25519
~/.ssh/id_ed25519.pub
```

私钥 `id_ed25519` 必须保密；公钥 `id_ed25519.pub` 可以安装到服务器。

### 2. 先敲门放行端口

以 `114 -> 6222` 为例：

```bash
curl -sS -X POST "http://140.143.209.222:7500/api/user/activate" \
  -H "Content-Type: application/json" \
  -d '{"token":"<YOUR_FRP_TOKEN>","port":6222}'
```

Windows PowerShell：

```powershell
Invoke-RestMethod `
  -Uri "http://140.143.209.222:7500/api/user/activate" `
  -Method POST `
  -ContentType "application/json" `
  -Body '{"token":"<YOUR_FRP_TOKEN>","port":6222}'
```

### 3. 安装公钥到服务器

macOS / Linux 推荐使用 `ssh-copy-id`：

```bash
ssh-copy-id -i ~/.ssh/id_ed25519.pub -p 6222 root@140.143.209.222
```

Windows 没有 `ssh-copy-id` 时，可以用 PowerShell：

```powershell
$pub = (Get-Content "$env:USERPROFILE\.ssh\id_ed25519.pub" -Raw).Trim()
$pub64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($pub))
ssh -p 6222 root@140.143.209.222 "mkdir -p ~/.ssh && chmod 700 ~/.ssh && touch ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys && (echo $pub64 | base64 -d | grep -qxF - ~/.ssh/authorized_keys || echo $pub64 | base64 -d >> ~/.ssh/authorized_keys)"
```

安装后测试：

```bash
ssh -o BatchMode=yes -p 6222 root@140.143.209.222 "hostname; whoami"
```

如果能输出主机名和 `root`，说明免密登录成功。

## 配置标准 SSH 别名

编辑 `~/.ssh/config`。Windows 路径通常是：

```text
C:\Users\<你的用户名>\.ssh\config
```

macOS / Linux 路径通常是：

```text
~/.ssh/config
```

示例：

```sshconfig
Host 114
  HostName 140.143.209.222
  Port 6222
  User root
  IdentityFile ~/.ssh/id_ed25519
  StrictHostKeyChecking accept-new
  ServerAliveInterval 30
  ServerAliveCountMax 3

Host 3
  HostName 140.143.209.222
  Port 6226
  User root
  IdentityFile ~/.ssh/id_ed25519
  StrictHostKeyChecking accept-new
  ServerAliveInterval 30
  ServerAliveCountMax 3

Host 102
  HostName 140.143.209.222
  Port 6227
  User root
  IdentityFile ~/.ssh/id_ed25519
  StrictHostKeyChecking accept-new
  ServerAliveInterval 30
  ServerAliveCountMax 3
```

这样可以使用：

```bash
ssh 114
ssh 3
ssh 102
scp ./file.txt 102:/root/
```

注意：`ssh 114` 这种标准 SSH 别名只负责 SSH 连接，不会自动调用 activate。如果白名单过期，需要先敲门，或者使用下一节的自动敲门脚本。

## Windows：配置自动敲门快捷命令

当前仓库已提供 Windows 工具脚本：

```text
tools/windows/frp-ssh.ps1
tools/windows/install-frp-ssh-key.py
tools/windows/ssh114.cmd
tools/windows/ssh113.cmd
tools/windows/ssh118.cmd
tools/windows/ssh181.cmd
tools/windows/ssh3.cmd
tools/windows/ssh102.cmd
```

### 1. 安装脚本

把 `tools/windows` 里的脚本复制到：

```text
C:\Users\<你的用户名>\bin
```

把这个目录加入用户 PATH：

```powershell
$bin = "$env:USERPROFILE\bin"
New-Item -ItemType Directory -Force -Path $bin | Out-Null
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ";") -notcontains $bin) {
  [Environment]::SetEnvironmentVariable("Path", ($userPath.TrimEnd(";") + ";" + $bin), "User")
}
```

重新打开终端后生效。

### 2. 加密保存 FRP token

Windows 推荐用 DPAPI 加密保存 token，仅当前 Windows 用户可解密：

```powershell
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.frp-ssh" | Out-Null
ConvertTo-SecureString "<YOUR_FRP_TOKEN>" -AsPlainText -Force |
  Export-Clixml -Path "$env:USERPROFILE\.frp-ssh\token.xml"
```

不要把 `token.xml` 放进 Git 仓库。

### 3. 使用快捷命令

```powershell
ssh114
ssh113
ssh118
ssh181
ssh3
ssh102
```

这些无空格命令会自动执行：

```text
activate 当前公网 IP -> SSH 免密登录
```

标准命令 `ssh 114` 仍然可以用，但它不会自动敲门。

### 4. 批量安装公钥

如果还没有给目标服务器安装公钥，可以使用仓库里的 `install-frp-ssh-key.py`。它会自动敲门、用服务器密码登录、追加本机公钥、修正权限，并验证免密登录。

如果所有目标服务器密码一样：

```powershell
install-frp-ssh-key --same-password 114 113 118 181 3 102
```

如果不同服务器密码不同：

```powershell
install-frp-ssh-key 114 113 118 181 3 102
```

脚本会逐台提示输入密码。不要把密码写进脚本或提交到 GitHub。

## macOS：配置自动敲门快捷命令

### 1. 保存 token

```bash
mkdir -p ~/.frp-ssh
printf '%s' '<YOUR_FRP_TOKEN>' > ~/.frp-ssh/token
chmod 600 ~/.frp-ssh/token
```

### 2. 创建通用脚本

```bash
mkdir -p ~/.local/bin
cat > ~/.local/bin/frp-ssh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

PUBLIC_HOST="140.143.209.222"
TOKEN_FILE="$HOME/.frp-ssh/token"

case "${1:-}" in
  114) PORT=6222 ;;
  113) PORT=6223 ;;
  118) PORT=6224 ;;
  181) PORT=6225 ;;
  3) PORT=6226 ;;
  102) PORT=6227 ;;
  list)
    cat <<LIST
114 -> ${PUBLIC_HOST}:6222
113 -> ${PUBLIC_HOST}:6223
118 -> ${PUBLIC_HOST}:6224
181 -> ${PUBLIC_HOST}:6225
3   -> ${PUBLIC_HOST}:6226
102 -> ${PUBLIC_HOST}:6227
LIST
    exit 0
    ;;
  *) echo "Usage: frp-ssh {114|113|118|181|3|102} [ssh args...]" >&2; exit 2 ;;
esac

shift || true
TOKEN="$(cat "$TOKEN_FILE")"

curl -sS -X POST "http://${PUBLIC_HOST}:7500/api/user/activate" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"${TOKEN}\",\"port\":${PORT}}" >/dev/null

exec ssh -p "$PORT" -i "$HOME/.ssh/id_ed25519" \
  -o StrictHostKeyChecking=accept-new \
  -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3 \
  "root@${PUBLIC_HOST}" "$@"
EOF

chmod +x ~/.local/bin/frp-ssh
```

确保 `~/.local/bin` 在 PATH 中：

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

### 3. 创建短命令

```bash
for name in 114 113 118 181 3 102; do
  cat > ~/.local/bin/ssh${name} <<EOF
#!/usr/bin/env bash
exec "\$HOME/.local/bin/frp-ssh" ${name} "\$@"
EOF
  chmod +x ~/.local/bin/ssh${name}
done
```

使用：

```bash
ssh114
ssh3
ssh102
```

## Linux：配置自动敲门快捷命令

Linux 配置方式与 macOS 基本一致。差异通常只有 shell 配置文件：

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

如果系统没有 `curl`，先安装：

```bash
# Debian/Ubuntu
sudo apt-get update
sudo apt-get install -y curl openssh-client

# RHEL/CentOS/Rocky
sudo dnf install -y curl openssh-clients
```

然后复用 macOS 一节里的 `~/.frp-ssh/token`、`~/.local/bin/frp-ssh` 和 `ssh114/ssh3/ssh102` 创建方式。

## 推荐命名规则

快捷名建议使用目标 IP 的最后一段：

| 目标服务器 | 推荐快捷命令 |
| --- | --- |
| `210.47.163.114` | `ssh114` |
| `210.47.163.113` | `ssh113` |
| `210.47.163.118` | `ssh118` |
| `210.47.163.181` | `ssh181` |
| `10.2.0.3` | `ssh3` |
| `10.2.0.102` | `ssh102` |

避免把 `10.2.0.3` 命名为 `ssh103`，因为它容易被误解为 `10.2.0.103`。

## 常见问题

### `token does not have permission for this port`

当前 token 没有该端口权限。联系管理员给 token 增加端口权限。

### `token is disabled`

token 已被禁用或过期。联系管理员重新启用或换 token。

### `Permission denied (publickey,password)`

FRP 端口可能已经放行，但目标服务器没有接受你的 SSH 公钥，或密码不正确。先确认公钥已经写入目标服务器的 `~/.ssh/authorized_keys`。

### 换网络后无法连接

白名单绑定的是当前公网 IP。换 Wi-Fi、代理、运营商出口后需要重新 activate。使用 `ssh114` 这类自动敲门命令可以避免手工操作。

### activate 超过 5 分钟后 SSH 会断开吗

不会。白名单只影响新连接。已经建立的 SSH 连接不会因为白名单过期主动断开；重新连接时需要再次 activate。

### 可以让 `ssh 114` 也自动敲门吗

标准 OpenSSH `Host 114` 配置只负责 SSH 参数，不能稳定、跨平台地执行“连接前 HTTP activate”。推荐使用无空格包装命令，例如 `ssh114`、`ssh3`、`ssh102`。

## 给管理员排查时提供的信息

排查问题时请提供：

- 目标快捷名或公网端口，例如 `ssh102` 或 `6227`。
- activate 接口返回的错误信息。
- 当前公网 IP。
- SSH 报错信息。

不要发送完整 token、服务器密码或私钥。
