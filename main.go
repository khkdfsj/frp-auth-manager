package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"frp_auth/database"
	"frp_auth/handlers"
	"frp_auth/middleware"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	flag.Parse()
	_ = configPath

	listenAddr := getEnv("AUTH_LISTEN_ADDR", "0.0.0.0:7500")
	dbPath := getEnv("AUTH_DB_PATH", "/www/server/frp_auth/data.db")
	adminUser := getEnv("AUTH_ADMIN_USER", "admin")
	adminPass := getEnv("AUTH_ADMIN_PASS", "admin123")

	if err := database.Init(dbPath); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	hash := sha256.Sum256([]byte(adminPass))
	hashStr := hex.EncodeToString(hash[:])
	if err := database.CreateAdminUser(adminUser, hashStr); err != nil {
		log.Printf("管理员账户可能已存在: %v", err)
	} else {
		log.Printf("默认管理员已创建: %s", adminUser)
	}
	if err := database.SeedDefaultSSHServices(); err != nil {
		log.Printf("seed default ssh services failed: %v", err)
	}

	go func() {
		for {
			time.Sleep(1 * time.Minute)
			database.CleanExpiredWhitelist()
		}
	}()

	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("POST /api/login", handlers.Login)
	mux.HandleFunc("GET /api/check-session", handlers.CheckSession)
	mux.HandleFunc("POST /api/user/activate", handlers.UserActivate)
	mux.HandleFunc("GET /api/auth/check", handlers.AuthCheck)

	// FRP dashboard proxy (admin-only)
	mux.HandleFunc("GET /api/frp/", middleware.AdminAuth(handlers.ProxyFRPAPI))
	mux.HandleFunc("GET /api/serverinfo", middleware.AdminAuth(handlers.FRPServerInfo))

	// Admin auth endpoints
	mux.HandleFunc("POST /api/logout", middleware.AdminAuth(handlers.Logout))
	mux.HandleFunc("POST /api/tokens", middleware.AdminAuth(handlers.CreateToken))
	mux.HandleFunc("GET /api/tokens", middleware.AdminAuth(handlers.ListTokens))
	mux.HandleFunc("PUT /api/tokens/{id}", middleware.AdminAuth(handlers.UpdateToken))
	mux.HandleFunc("DELETE /api/tokens/{id}", middleware.AdminAuth(handlers.DeleteToken))
	mux.HandleFunc("POST /api/permissions", middleware.AdminAuth(handlers.AddPortPermission))
	mux.HandleFunc("DELETE /api/permissions/{id}", middleware.AdminAuth(handlers.RemovePortPermission))
	mux.HandleFunc("DELETE /api/permissions", middleware.AdminAuth(handlers.RemovePortPermissionByTokenPort))
	mux.HandleFunc("GET /api/permissions", middleware.AdminAuth(handlers.ListPortPermissions))
	mux.HandleFunc("POST /api/ports/config", middleware.AdminAuth(handlers.SetPortConfig))
	mux.HandleFunc("GET /api/ports/config", middleware.AdminAuth(handlers.ListPortConfigs))
	mux.HandleFunc("DELETE /api/ports/config/{id}", middleware.AdminAuth(handlers.DeletePortConfig))

	// Port IP allowlist
	mux.HandleFunc("POST /api/ports/ip-allow", middleware.AdminAuth(handlers.AddPortIPAllow))
	mux.HandleFunc("POST /api/ports/ip-batch", middleware.AdminAuth(handlers.BatchAddPortIP))
	mux.HandleFunc("DELETE /api/ports/ip-allow/{id}", middleware.AdminAuth(handlers.RemovePortIPAllow))
	mux.HandleFunc("GET /api/ports/ip-allow", middleware.AdminAuth(handlers.ListPortIPAllowlist))
	mux.HandleFunc("PUT /api/ports/ip-mode", middleware.AdminAuth(handlers.SetPortIPMode))
	mux.HandleFunc("GET /api/ssh-services", middleware.AdminAuth(handlers.ListSSHServices))
	mux.HandleFunc("POST /api/ssh-services", middleware.AdminAuth(handlers.CreateSSHService))
	mux.HandleFunc("PUT /api/ssh-services/{id}", middleware.AdminAuth(handlers.UpdateSSHService))
	mux.HandleFunc("DELETE /api/ssh-services/{id}", middleware.AdminAuth(handlers.DeleteSSHService))
	mux.HandleFunc("POST /api/ssh-services/apply", middleware.AdminAuth(handlers.ApplySSHServices))
	mux.HandleFunc("GET /api/frpc-agent/status", middleware.AdminAuth(handlers.FrpcAgentStatus))

	mux.HandleFunc("GET /", serveStatic)

	log.Printf("运行部内网管理系统启动: %s", listenAddr)

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(adminHTML))
		return
	}
	http.NotFound(w, r)
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

const adminHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>运行部内网管理系统</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Microsoft YaHei','PingFang SC',sans-serif;background:#f0f2f5;color:#333;min-height:100vh}
.login-wrapper{display:flex;justify-content:center;align-items:center;min-height:100vh;background:linear-gradient(135deg,#1a1a2e 0%,#16213e 50%,#0f3460 100%)}
.login-box{width:400px;background:#fff;padding:40px;border-radius:12px;box-shadow:0 10px 40px rgba(0,0,0,0.3)}
.login-box h1{text-align:center;font-size:22px;margin-bottom:4px;color:#1a1a2e}
.login-box .sub{text-align:center;color:#999;font-size:13px;margin-bottom:24px}
.login-box input{width:100%;padding:12px 16px;margin-bottom:14px;border:1px solid #ddd;border-radius:6px;font-size:14px;outline:none;transition:border-color .2s}
.login-box input:focus{border-color:#3498db}
.login-box button{width:100%;padding:12px;background:#1a1a2e;color:#fff;border:none;border-radius:6px;font-size:15px;cursor:pointer;transition:background .2s}
.login-box button:hover{background:#16213e}
.app{display:none}
.header{background:#1a1a2e;color:#fff;padding:0 24px;display:flex;justify-content:space-between;align-items:center;height:54px;position:sticky;top:0;z-index:100}
.header h1{font-size:17px;letter-spacing:1px}
.header .user-info{display:flex;align-items:center;gap:12px;font-size:13px}
.header button{background:rgba(255,255,255,0.12);color:#fff;border:1px solid rgba(255,255,255,0.25);padding:5px 14px;border-radius:4px;cursor:pointer;font-size:12px}
.header button:hover{background:rgba(255,255,255,0.22)}
.main{max-width:1400px;margin:0 auto;padding:20px}
.stats-row{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:14px;margin-bottom:20px}
.stat-card{background:#fff;border-radius:8px;padding:14px 18px;box-shadow:0 1px 3px rgba(0,0,0,0.08);display:flex;align-items:center;gap:12px}
.stat-card .icon{width:42px;height:42px;border-radius:10px;display:flex;align-items:center;justify-content:center;font-size:18px;flex-shrink:0}
.stat-card .icon.blue{background:#dbeafe;color:#2563eb}
.stat-card .icon.green{background:#d1fae5;color:#059669}
.stat-card .icon.orange{background:#ffedd5;color:#ea580c}
.stat-card .icon.red{background:#fee2e2;color:#dc2626}
.stat-card .icon.purple{background:#ede9fe;color:#7c3aed}
.stat-card .val{font-size:20px;font-weight:700}
.stat-card .lbl{font-size:12px;color:#888;margin-top:1px}
.tabs{display:flex;gap:0;margin-bottom:20px;background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,0.08)}
.tab{padding:11px 22px;background:#fff;border:none;cursor:pointer;font-size:13px;color:#666;transition:all .2s;border-bottom:2px solid transparent}
.tab:hover{color:#1a1a2e;background:#f8f9fa}
.tab.active{color:#1a1a2e;border-bottom-color:#1a1a2e;font-weight:600}
.tab-content{display:none}
.tab-content.active{display:block}
.card{background:#fff;border-radius:8px;padding:20px;margin-bottom:20px;box-shadow:0 1px 3px rgba(0,0,0,0.08)}
.card h2{font-size:15px;margin-bottom:14px;padding-bottom:10px;border-bottom:1px solid #eee;display:flex;align-items:center;gap:6px}
.card h2 .tip{font-weight:400;font-size:12px;color:#999;margin-left:auto}
table{width:100%;border-collapse:collapse;font-size:13px}
th,td{text-align:left;padding:9px 12px;border-bottom:1px solid #f0f0f0}
th{background:#fafbfc;font-weight:600;color:#555;white-space:nowrap;font-size:12px}
tr:hover td{background:#f8f9fa}
.btn{padding:5px 13px;border:none;border-radius:4px;cursor:pointer;font-size:12px;margin:1px;white-space:nowrap;transition:opacity .2s}
.btn:hover{opacity:0.85}
.btn-primary{background:#3498db;color:#fff}
.btn-danger{background:#e74c3c;color:#fff}
.btn-success{background:#27ae60;color:#fff}
.btn-warning{background:#f39c12;color:#fff}
.btn-sm{padding:3px 9px;font-size:11px}
.tag{display:inline-block;padding:2px 8px;border-radius:3px;font-size:11px;margin:1px;font-weight:500}
.tag-green{background:#d1fae5;color:#065f46}
.tag-red{background:#fee2e2;color:#991b1b}
.tag-gray{background:#f3f4f6;color:#6b7280}
.tag-blue{background:#dbeafe;color:#1e40af}
.tag-orange{background:#ffedd5;color:#9a3412}
.form-row{display:flex;gap:10px;align-items:flex-end;margin-bottom:12px;flex-wrap:wrap}
.form-group{display:flex;flex-direction:column;min-width:130px}
.form-group label{font-size:12px;margin-bottom:3px;color:#666;font-weight:500}
.form-group input,.form-group select,.form-group textarea{padding:8px 12px;border:1px solid #ddd;border-radius:6px;font-size:13px;outline:none;transition:border-color .2s}
.form-group textarea{resize:vertical;min-height:50px}
.form-group input:focus,.form-group select:focus,.form-group textarea:focus{border-color:#3498db}
.token-value{font-family:'SF Mono',Monaco,Consolas,monospace;background:#f1f3f4;padding:2px 8px;border-radius:4px;font-size:11px;word-break:break-all;max-width:200px;display:inline-block}
.toast{position:fixed;top:20px;right:20px;padding:12px 24px;border-radius:8px;color:#fff;z-index:9999;animation:slideIn .3s;font-size:14px;box-shadow:0 4px 12px rgba(0,0,0,0.15);max-width:400px}
.toast-success{background:#059669}
.toast-error{background:#dc2626}
.toast-info{background:#2563eb}
@keyframes slideIn{from{opacity:0;transform:translateX(40px)}to{opacity:1;transform:translateX(0)}}
.empty{text-align:center;padding:30px;color:#999;font-size:13px}
.traffic{font-family:monospace;font-size:12px}
.modal-overlay{display:none;position:fixed;inset:0;background:rgba(0,0,0,0.45);z-index:200;align-items:flex-start;justify-content:center;padding-top:60px;overflow-y:auto}
.modal-overlay.show{display:flex}
.modal{background:#fff;border-radius:12px;padding:28px;max-width:600px;width:95%;box-shadow:0 12px 48px rgba(0,0,0,0.25);margin-bottom:40px}
.modal h3{margin-bottom:20px;font-size:16px;display:flex;align-items:center;justify-content:space-between}
.modal .mclose{background:none;border:none;font-size:22px;cursor:pointer;color:#999;padding:0 4px}
.modal .mclose:hover{color:#333}
.modal .form-row{margin-bottom:10px}
.perms-inline{display:flex;flex-wrap:wrap;gap:4px;margin:8px 0}
.perm-chip{display:inline-flex;align-items:center;gap:4px;background:#dbeafe;color:#1e40af;padding:2px 8px;border-radius:12px;font-size:12px}
.perm-chip button{background:none;border:none;color:#1e40af;cursor:pointer;font-size:14px;line-height:1;padding:0 2px}
.perm-chip button:hover{color:#dc2626}.tag-purple{display:inline-block;padding:2px 8px;border-radius:3px;font-size:11px;margin:1px;font-weight:500;background:#ede9fe;color:#6d28d9}
</style>
</head>
<body>

<div class="login-wrapper" id="loginPage">
<div class="login-box">
<h1>运行部内网管理系统</h1>
<div class="sub">内网穿透 · 访问控制 · 代理监控</div>
<input type="text" id="loginUser" placeholder="用户名" autocomplete="username" />
<input type="password" id="loginPass" placeholder="密码" autocomplete="current-password" />
<button onclick="doLogin()">登 录</button>
</div>
</div>

<div class="app" id="appPage">
<div class="header">
<h1>运行部内网管理系统</h1>
<div class="user-info"><span id="currentUser"></span><button onclick="doLogout()">退出登录</button></div>
</div>
<div class="main">
<div class="stats-row" id="statsRow"></div>
<div class="tabs">
<button class="tab active" data-tab="proxy">代理状态</button>
<button class="tab" data-tab="tokens">Token 管理</button>
<button class="tab" data-tab="ports">端口配置</button>
<button class="tab" data-tab="ssh">SSH 服务</button>
<button class="tab" data-tab="perms">权限列表</button>
</div>

<!-- 代理状态 -->
<div id="tab-proxy" class="tab-content active">
<div class="card">
<h2>TCP 代理列表<span class="tip" id="proxyRefresh"></span></h2>
<table>
<thead><tr><th>代理名称</th><th>远程端口</th><th>版本</th><th>状态</th><th>当前连接</th><th>今日入站</th><th>今日出站</th><th>最后上线</th><th>鉴权状态</th></tr></thead>
<tbody id="proxyList"></tbody>
</table>
</div>
</div>

<!-- Token 管理 -->
<div id="tab-tokens" class="tab-content">
<div class="card">
<h2>创建 Token</h2>
<div class="form-row">
<div class="form-group"><label>用户名称</label><input type="text" id="newTokenName" placeholder="例如：张三" /></div>
<div class="form-group"><label>备注说明</label><input type="text" id="newTokenNotes" placeholder="如：开发部-远程运维" /></div>
<div class="form-group"><label>过期时间</label><input type="datetime-local" id="newTokenExpiry" /></div>
<div class="form-group"><label>流量限额(MB)</label><input type="number" id="newTokenLimit" placeholder="0=不限制" value="0" min="0" /></div>
<button class="btn btn-primary" onclick="createToken()">创建 Token</button>
</div>
</div>
<div class="card">
<h2>Token 列表</h2>
<table>
<thead><tr><th>ID</th><th>用户</th><th>Token</th><th>备注</th><th>状态</th><th>过期时间</th><th>激活次数</th><th>授权端口</th><th>流量限额</th><th>操作</th></tr></thead>
<tbody id="tokenList"></tbody>
</table>
</div>
</div>

<!-- 端口配置 -->
<div id="tab-ports" class="tab-content">
<div class="card">
<h2>端口鉴权设置</h2>
<div class="form-row">
<div class="form-group"><label>端口号</label><input type="number" id="portConfigPort" placeholder="例如：6223" min="1" /></div>
<div class="form-group"><label>鉴权模式</label>
<select id="portConfigAuth">
<option value="token">Token 鉴权</option>
<option value="open">开放访问</option>
	<option value="ip">IP 限制</option>
	</select>
	<select id="portConfigIPMode" style="display:none;width:100%;padding:8px 12px;border:1px solid #ddd;border-radius:6px;font-size:13px;margin-top:8px"><option value="whitelist">白名单（仅允许列表IP）</option><option value="blacklist">黑名单（禁止列表IP）</option></select>
</select></div>
<button class="btn btn-primary" onclick="setPortConfig()">保存配置</button>
</div>
</div>
<div class="card">
<h2>所有端口状态</h2>
<table>
<thead><tr><th>端口</th><th>代理名称</th><th>鉴权状态</th><th>操作</th></tr></thead>
<tbody id="portConfigList"></tbody>
</table>
</div>
</div>

<!-- SSH 服务管理 -->
<div id="tab-ssh" class="tab-content">
<div class="card">
<h2>SSH 服务管理<span class="tip" id="agentStatus">Agent: 检查中</span></h2>
<div class="form-row">
<input type="hidden" id="sshServiceId" />
<div class="form-group"><label>服务名称</label><input type="text" id="sshName" placeholder="例如：210.47.163.120" /></div>
<div class="form-group"><label>内网 IP</label><input type="text" id="sshTargetIP" placeholder="例如：210.47.163.120" /></div>
<div class="form-group"><label>公网端口</label><input type="number" id="sshRemotePort" placeholder="留空自动分配" min="6222" max="6299" /></div>
<div class="form-group"><label>状态</label><select id="sshActive"><option value="1">启用</option><option value="0">停用</option></select></div>
<div class="form-group"><label>备注</label><input type="text" id="sshNotes" placeholder="可选" /></div>
<button class="btn btn-primary" onclick="saveSSHService()">保存服务</button>
<button class="btn" style="background:#e5e7eb;color:#333" onclick="resetSSHForm()">清空</button>
<button class="btn btn-success" onclick="applySSHServices()">应用配置</button>
</div>
</div>
<div class="card">
<h2>SSH 服务列表</h2>
<table>
<thead><tr><th>ID</th><th>名称</th><th>公网端口</th><th>目标</th><th>状态</th><th>应用状态</th><th>备注</th><th>操作</th></tr></thead>
<tbody id="sshServiceList"></tbody>
</table>
</div>
</div>

<!-- 权限列表 -->
<div id="tab-perms" class="tab-content">
<div class="card">
<h2>所有端口权限分配</h2>
<table>
<thead><tr><th>ID</th><th>Token</th><th>端口</th><th>创建时间</th><th>操作</th></tr></thead>
<tbody id="permList"></tbody>
</table>
</div>
</div>
</div>
</div>

<!-- 编辑 Token 弹窗 -->
<div class="modal-overlay" id="editModal">
<div class="modal">
<h3>编辑 Token #<span id="editTokenId"></span><button class="mclose" onclick="closeEditModal()">&times;</button></h3>
<div class="form-row">
<div class="form-group"><label>用户名称</label><input type="text" id="editName" /></div>
<div class="form-group"><label>备注说明</label><input type="text" id="editNotes" /></div>
</div>
<div class="form-row">
<div class="form-group"><label>启用状态</label><select id="editActive"><option value="1">启用</option><option value="0">禁用</option></select></div>
<div class="form-group"><label>过期时间</label><input type="datetime-local" id="editExpiry" /></div>
<div class="form-group"><label>流量限额(MB) 0=不限</label><input type="number" id="editLimit" value="0" min="0" /></div>
</div>
<div style="margin-top:12px">
<label style="font-size:12px;color:#666;font-weight:500">授权端口</label>
<div class="perms-inline" id="editPerms"></div>
<div class="form-row" style="margin-top:8px">
<select id="editAddPort" style="width:220px;padding:6px 10px;border:1px solid #ddd;border-radius:4px;font-size:12px"><option value="">-- 选择端口 --</option></select>
<button class="btn btn-success btn-sm" onclick="editAddPerm()">添加端口</button>
</div>
</div>
<div style="margin-top:16px;text-align:right">
<button class="btn btn-primary" onclick="saveEdit()">保存修改</button>
<button class="btn" style="background:#e5e7eb;color:#333;margin-left:8px" onclick="closeEditModal()">取消</button>
</div>
</div>
</div>

<!-- 创建 Token 成功弹窗 -->
<div class="modal-overlay" id="tokenModal">
<div class="modal">
<h3>Token 创建成功<button class="mclose" onclick="closeModal()">&times;</button></h3>
<p style="color:#666;margin-bottom:12px;font-size:13px">请立即复制并安全保存此 Token，关闭后将<strong>无法再次查看</strong>：</p>
<div style="background:#f0fdf4;border:1px solid #86efac;border-radius:6px;padding:12px;font-family:monospace;font-size:13px;word-break:break-all;margin-bottom:12px" id="modalTokenValue"></div>
<button class="btn btn-primary" onclick="copyToken()">复制 Token</button>
<button class="btn" style="background:#e5e7eb;color:#333;margin-left:8px" onclick="closeModal()">关闭</button>
</div>
</div>

<script>
var sessionToken = '';
var refreshTimer = null;

function startRefreshInterval() {
  if (refreshTimer) clearInterval(refreshTimer);
  refreshTimer = setInterval(refreshProxy, 10000);
}
var proxyDataCache = [];
var proxyPorts = [];
var tokenEditData = null;

// === 登录保持 ===
function initSession() {
  var saved = localStorage.getItem('frp_auth_session');
  if (saved) {
    sessionToken = saved;
    fetch('/api/check-session', {headers: {'Authorization': 'Bearer ' + sessionToken}})
      .then(function(r) { return r.json(); })
      .then(function(d) {
        if (d.valid) {
          document.getElementById('loginPage').style.display = 'none';
          document.getElementById('appPage').style.display = 'block';
          if (d.username) document.getElementById('currentUser').textContent = d.username;
          loadAll();
          startRefreshInterval();
        } else {
          // Only clear if another login hasn't updated the session in the meantime
          if (sessionToken === saved) {
            localStorage.removeItem('frp_auth_session');
            sessionToken = '';
          }
        }
      }).catch(function() {});
  }
}

function doLogin() {
  var body = {username: document.getElementById('loginUser').value, password: document.getElementById('loginPass').value};
  fetch('/api/login', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(body)})
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (data.token) {
        sessionToken = data.token;
        localStorage.setItem('frp_auth_session', data.token);
        document.getElementById('loginPage').style.display = 'none';
        document.getElementById('appPage').style.display = 'block';
        document.getElementById('currentUser').textContent = data.username || document.getElementById('loginUser').value;
        loadAll();
        startRefreshInterval();
      } else { toast(data.error || '用户名或密码错误', 'error'); }
    }).catch(function() { toast('登录失败，请检查网络连接', 'error'); });
}

function doLogout() {
  if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null; }
  api('POST', '/api/logout').catch(function(){});
  sessionToken = '';
  localStorage.removeItem('frp_auth_session');
  document.getElementById('loginPage').style.display = 'flex';
  document.getElementById('appPage').style.display = 'none';
  proxyPorts = [];
}

function api(method, path, body) {
  var headers = {'Content-Type': 'application/json'};
  if (sessionToken) headers['Authorization'] = 'Bearer ' + sessionToken;
  var opts = {method: method, headers: headers};
  if (body) opts.body = JSON.stringify(body);
  return fetch(path, opts).then(function(r) {
    if (r.status === 401) { doLogout(); throw new Error('未登录'); }
    if (!r.ok) throw new Error('请求失败: ' + r.status);
    return r.json();
  });
}

function loadAll() { loadProxies(); loadTokens(); loadPortConfigs(); loadPermissions(); loadSSHServices(); loadAgentStatus(); }

// === 代理状态 ===
function refreshProxy() { loadProxies(true); }

function loadProxies(silent) {
  api('GET', '/api/frp/proxy/tcp').then(function(data) {
    proxyDataCache = (data && data.proxies) ? data.proxies : [];
    proxyPorts = proxyDataCache.map(function(p) { return p.conf ? p.conf.remotePort : 0; }).filter(function(v) { return v > 0; });
    renderProxies();
    if (!silent) { updateStats(); loadPortConfigs(true); }
    document.getElementById('proxyRefresh').textContent = '上次刷新: ' + new Date().toLocaleTimeString('zh-CN');
  }).catch(function(e) {
    if (!silent) document.getElementById('proxyList').innerHTML = '<tr><td colspan="9" class="empty">无法获取 FRP 代理数据，请检查 FRP 服务</td></tr>';
  });
}

function renderProxies() {
  var html = '';
  if (proxyDataCache.length === 0) {
    html = '<tr><td colspan="9" class="empty">暂无代理连接</td></tr>';
  } else {
    proxyDataCache.forEach(function(p) {
      var port = p.conf ? p.conf.remotePort : 0;
      var statusTag = p.status === 'online' ? '<span class="tag tag-green">在线</span>' : '<span class="tag tag-red">离线</span>';
      var trafficIn = formatBytes(p.todayTrafficIn || 0);
      var trafficOut = formatBytes(p.todayTrafficOut || 0);
      var lastStart = p.lastStartTime || '-';
      var authStatus = getAuthStatusHtml(port);
      html += '<tr><td><strong>' + esc(p.name) + '</strong></td>' +
        '<td>' + port + '</td>' +
        '<td>' + esc(p.clientVersion || '-') + '</td>' +
        '<td>' + statusTag + '</td>' +
        '<td>' + (p.curConns || 0) + '</td>' +
        '<td class="traffic">' + trafficIn + '</td>' +
        '<td class="traffic">' + trafficOut + '</td>' +
        '<td>' + lastStart + '</td>' +
        '<td>' + authStatus + '</td></tr>';
    });
  }
  document.getElementById('proxyList').innerHTML = html;
}

var authConfigCache = {};
function getAuthStatusHtml(port) {
  if (!port) return '<span class="tag tag-gray">-</span>';
  if (authConfigCache[port] === undefined) return '<span class="tag tag-gray">开放访问</span>';
  var mode = authConfigCache[port];
  if (mode === 'ip') return '<span class="tag tag-purple">IP 限制</span>';
  if (mode === 'token') return '<span class="tag tag-green">Token 鉴权</span>';
  return '<span class="tag tag-gray">开放访问</span>';
}

function updateStats() {
  var online = 0, totalConns = 0, totalIn = 0, totalOut = 0;
  proxyDataCache.forEach(function(p) {
    if (p.status === 'online') online++;
    totalConns += (p.curConns || 0);
    totalIn += (p.todayTrafficIn || 0);
    totalOut += (p.todayTrafficOut || 0);
  });
  document.getElementById('statsRow').innerHTML =
    '<div class="stat-card"><div class="icon blue">#</div><div><div class="val">' + proxyDataCache.length + '</div><div class="lbl">代理总数</div></div></div>' +
    '<div class="stat-card"><div class="icon green">V</div><div><div class="val">' + online + '</div><div class="lbl">在线代理</div></div></div>' +
    '<div class="stat-card"><div class="icon orange">U</div><div><div class="val">' + totalConns + '</div><div class="lbl">当前连接数</div></div></div>' +
    '<div class="stat-card"><div class="icon red">T</div><div><div class="val">' + formatBytes(totalIn + totalOut) + '</div><div class="lbl">今日总流量</div></div></div>';
}

function formatBytes(b) {
  if (!b || b === 0) return '0 B';
  var k = 1024, sizes = ['B','KB','MB','GB'];
  var i = Math.floor(Math.log(b) / Math.log(k));
  return parseFloat((b / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// === Token 管理 ===
function loadTokens() {
  api('GET', '/api/tokens').then(function(data) {
    var html = '';
    (data || []).forEach(function(t) {
      var permsHtml = (t.permissions || []).map(function(p) {
        return '<span class="tag tag-green">' + p.port + '</span>';
      }).join(' ') || '<span class="tag tag-gray">无</span>';
      var statusTag = t.is_active ? '<span class="tag tag-green">启用</span>' : '<span class="tag tag-red">禁用</span>';
      var expiresStr = t.expires_at ? new Date(t.expires_at).toLocaleString('zh-CN') : '<span class="tag tag-gray">永不过期</span>';
      var limitStr = t.traffic_limit > 0 ? formatBytes(t.traffic_limit * 1024 * 1024) : '<span class="tag tag-gray">不限制</span>';
      html += '<tr>' +
        '<td>' + t.id + '</td>' +
        '<td>' + esc(t.name) + '</td>' +
        '<td><span class="token-value">' + esc(t.token.substring(0,16)) + '...</span></td>' +
        '<td>' + esc(t.notes || '-') + '</td>' +
        '<td>' + statusTag + '</td>' +
        '<td>' + expiresStr + '</td>' +
        '<td>' + (t.activation_count || 0) + ' 次</td>' +
        '<td>' + permsHtml + '</td>' +
        '<td>' + limitStr + '</td>' +
        '<td><button class="btn btn-sm btn-primary" onclick="openEditModal(' + t.id + ')">编辑</button> ' +
        '<button class="btn btn-sm btn-danger" onclick="deleteToken(' + t.id + ')">删除</button></td></tr>';
    });
    document.getElementById('tokenList').innerHTML = html || '<tr><td colspan="10" class="empty">暂无 Token</td></tr>';
  });
}

function createToken() {
  var n = document.getElementById('newTokenName').value.trim();
  var notes = document.getElementById('newTokenNotes').value.trim();
  var expiry = document.getElementById('newTokenExpiry').value;
  var limit = parseInt(document.getElementById('newTokenLimit').value) || 0;
  if (!n) { toast('请输入用户名称', 'error'); return; }
  var body = {name: n, notes: notes, traffic_limit: limit};
  if (expiry) body.expires_at = new Date(expiry).toISOString();
  api('POST', '/api/tokens', body).then(function(data) {
    document.getElementById('newTokenName').value = '';
    document.getElementById('newTokenNotes').value = '';
    document.getElementById('newTokenExpiry').value = '';
    document.getElementById('newTokenLimit').value = '0';
    document.getElementById('modalTokenValue').textContent = data.token;
    document.getElementById('tokenModal').classList.add('show');
    loadTokens();
  }).catch(function(e) { toast('创建失败', 'error'); });
}

function deleteToken(id) {
  if (!confirm('确认删除 Token #' + id + '？此操作不可恢复！')) return;
  api('DELETE', '/api/tokens/' + id).then(function() { toast('已删除', 'success'); loadTokens(); loadPermissions(); });
}

// === 编辑 Token 弹窗 ===
function openEditModal(id) {
  var token = null;
  api('GET', '/api/tokens').then(function(data) {
    token = (data || []).find(function(t) { return t.id === id; });
    if (!token) { toast('Token 不存在', 'error'); return; }
    tokenEditData = token;
    document.getElementById('editTokenId').textContent = id;
    document.getElementById('editName').value = token.name || '';
    document.getElementById('editNotes').value = token.notes || '';
    document.getElementById('editActive').value = token.is_active ? '1' : '0';
    document.getElementById('editExpiry').value = token.expires_at ? new Date(token.expires_at).toISOString().slice(0,16) : '';
    document.getElementById('editLimit').value = token.traffic_limit || 0;
    // Populate port dropdown with all known proxy ports
    var sel = document.getElementById('editAddPort');
    sel.innerHTML = '<option value="">-- 选择端口 --</option>';
    proxyPorts.forEach(function(p) {
      var name = '';
      proxyDataCache.forEach(function(px) { if (px.conf && px.conf.remotePort === p) name = px.name; });
      sel.innerHTML += '<option value="' + p + '">' + p + ' - ' + esc(name || '未知') + '</option>';
    });
    // Also add ports that have port_config but aren't in proxy list
    Object.keys(authConfigCache).forEach(function(p) {
      if (proxyPorts.indexOf(parseInt(p)) === -1) {
        sel.innerHTML += '<option value="' + p + '">' + p + ' - (已配置)</option>';
      }
    });
    renderEditPerms(token.permissions || []);
    document.getElementById('editModal').classList.add('show');
  });
}

function renderEditPerms(perms) {
  var html = '';
  (perms || []).forEach(function(p) {
    html += '<span class="perm-chip">端口 ' + p.port + ' <button onclick="editRemovePerm(' + p.id + ')" title="移除">&times;</button></span>';
  });
  document.getElementById('editPerms').innerHTML = html || '<span style="font-size:12px;color:#999">暂未授权任何端口</span>';
}

function editAddPerm() {
  var sel = document.getElementById('editAddPort');
  var tokenId = parseInt(document.getElementById('editTokenId').textContent);
  var port = parseInt(sel.value);
  if (!port) { toast('请选择端口', 'error'); return; }
  api('POST', '/api/permissions', {token_id: tokenId, port: port}).then(function() {
    sel.value = '';
    openEditModal(tokenId);
  }).catch(function(e) { toast('添加失败', 'error'); });
}

function editRemovePerm(permId) {
  var tokenId = parseInt(document.getElementById('editTokenId').textContent);
  if (!confirm('移除该端口权限？')) return;
  api('DELETE', '/api/permissions/' + permId).then(function() {
    openEditModal(tokenId);
  }).catch(function(e) { toast('移除失败', 'error'); });
}

function saveEdit() {
  var id = parseInt(document.getElementById('editTokenId').textContent);
  var name = document.getElementById('editName').value.trim();
  var notes = document.getElementById('editNotes').value.trim();
  var active = document.getElementById('editActive').value === '1';
  var expiry = document.getElementById('editExpiry').value;
  var limit = parseInt(document.getElementById('editLimit').value) || 0;
  if (!name) { toast('用户名称不能为空', 'error'); return; }

  var promises = [];
  promises.push(api('PUT', '/api/tokens/' + id, {
    name: name, notes: notes, is_active: active,
    expires_at: expiry ? new Date(expiry).toISOString() : null,
    traffic_limit: limit
  }));
  var sel = document.getElementById('editAddPort');
  var port = parseInt(sel.value);
  if (port) {
    promises.push(api('POST', '/api/permissions', {token_id: id, port: port}));
  }
  Promise.all(promises).then(function() {
    closeEditModal();
    toast('Token 已更新', 'success');
    loadTokens(); loadPermissions();
  }).catch(function() { toast('保存失败', 'error'); });
}

function closeEditModal() {
  document.getElementById('editModal').classList.remove('show');
  tokenEditData = null;
}

// === 端口配置 ===
function setPortConfig() {
  var port = parseInt(document.getElementById('portConfigPort').value);
  var authMode = document.getElementById('portConfigAuth').value;
  if (!port) { toast('请输入端口号', 'error'); return; }
  var modeLabels = {open: '开放访问', token: 'Token 鉴权', ip: 'IP 限制'};
  var body = {port: port, auth_mode: authMode};
  if (authMode === 'ip') {
    var ipModeEl = document.getElementById('portConfigIPMode');
    if (ipModeEl) body.ip_list_mode = ipModeEl.value;
  }
  api('POST', '/api/ports/config', body).then(function() {
    document.getElementById('portConfigPort').value = '';
    toast('端口 ' + port + ' 已设为：' + (modeLabels[authMode] || authMode), 'success');
    loadPortConfigs();
    loadProxies(true);
  });
}

var portConfigData = {};

function loadPortConfigs(silent) {
  api('GET', '/api/ports/config').then(function(data) {
    authConfigCache = {};
    portConfigData = {};
    (data || []).forEach(function(c) { authConfigCache[c.port] = c.auth_mode || 'token'; portConfigData[c.port] = c; });
    // Mark all known proxy ports not in config as open
    proxyPorts.forEach(function(p) {
      if (authConfigCache[p] === undefined) authConfigCache[p] = 'open';
    });
    if (silent) { renderProxies(); return; }
    renderPortConfigTable(data || []);
  }).catch(function() {});
}

function renderPortConfigTable(configs) {
  var configMap = {};
  configs.forEach(function(c) { configMap[c.port] = c; });

  // Collect all ports: configured + from proxies
  var allPorts = {};
  configs.forEach(function(c) { allPorts[c.port] = c; });
  proxyPorts.forEach(function(p) {
    if (!allPorts[p]) allPorts[p] = {port: p, auth_mode: 'open', id: 0, created_at: null};
  });
  var sortedPorts = Object.values(allPorts).sort(function(a,b) { return a.port - b.port; });

  // Build proxy name map
  var proxyNameMap = {};
  proxyDataCache.forEach(function(p) {
    if (p.conf && p.conf.remotePort) proxyNameMap[p.conf.remotePort] = p.name;
  });

  var modeLabels = {open: '开放访问', token: 'Token 鉴权', ip: 'IP 限制'};
  var modeTags = {open: '<span class="tag tag-gray">开放访问</span>', token: '<span class="tag tag-green">Token 鉴权</span>', ip: '<span class="tag tag-purple">IP 限制</span>'};

  var html = '';
  sortedPorts.forEach(function(c) {
    var mode = c.auth_mode || 'token';
    var tag = modeTags[mode] || modeTags['token'];
    var proxyName = proxyNameMap[c.port] || '-';
    var action = c.id > 0
      ? '<button class="btn btn-sm btn-warning" onclick="changeAuthMode(' + c.port + ')">切换模式</button> <button class="btn btn-sm btn-danger" onclick="deletePortConfigById(' + c.id + ')">清除配置</button>'
      : '<button class="btn btn-sm btn-primary" onclick="quickAddAuth(' + c.port + ')">添加配置</button>';
    html += '<tr><td><strong>' + c.port + '</strong></td><td>' + esc(proxyName) + '</td><td>' + tag + '</td><td>' + action + '</td></tr>';
  });
  document.getElementById('portConfigList').innerHTML = html || '<tr><td colspan="4" class="empty">暂未检测到端口</td></tr>';
  
  // After rendering port table, load IP allowlist
  loadIPAllowlists();
}

function quickAddAuth(port) {
  api('POST', '/api/ports/config', {port: port, auth_mode: 'token'}).then(function() {
    toast('端口 ' + port + ' 已设为 Token 鉴权', 'success');
    loadPortConfigs();
    loadProxies(true);
  });
}

function changeAuthMode(port) {
  var current = authConfigCache[port] || 'token';
  var modes = ['open', 'token', 'ip'];
  var labels = {open: '开放访问', token: 'Token 鉴权', ip: 'IP 限制'};
  var idx = modes.indexOf(current);
  var next = modes[(idx + 1) % 3];
  if (!confirm('将端口 ' + port + ' 从 ' + (labels[current] || current) + ' 切换为 ' + (labels[next] || next) + '？')) return;
  api('POST', '/api/ports/config', {port: port, auth_mode: next}).then(function() {
    toast('端口 ' + port + ' 已切换为 ' + (labels[next] || next), 'success');
    loadPortConfigs();
    loadProxies(true);
  });
}

function deletePortConfigById(id) {
  if (!confirm('删除后该端口将恢复开放访问，确认？')) return;
  api('DELETE', '/api/ports/config/' + id).then(function() { toast('配置已删除', 'success'); loadPortConfigs(); loadProxies(true); });
}

// === 权限列表 ===
function loadPermissions() {
  api('GET', '/api/permissions').then(function(data) {
    var html = '';
    (data || []).forEach(function(p) {
      html += '<tr><td>' + p.id + '</td><td>#' + p.token_id + '</td>' +
        '<td><span class="tag tag-green">端口 ' + p.port + '</span></td>' +
        '<td>' + new Date(p.created_at).toLocaleString('zh-CN') + '</td>' +
        '<td><button class="btn btn-sm btn-danger" onclick="removePermById(' + p.id + ')">移除</button></td></tr>';
    });
    document.getElementById('permList').innerHTML = html || '<tr><td colspan="5" class="empty">暂无权限记录</td></tr>';
  });
}

function removePermById(id) {
  if (!confirm('确认移除？')) return;
  api('DELETE', '/api/permissions/' + id).then(function() { toast('已移除', 'success'); loadPermissions(); loadTokens(); });
}

// === SSH 服务管理 ===
var sshServicesCache = [];

function loadSSHServices() {
  api('GET', '/api/ssh-services').then(function(data) {
    sshServicesCache = data || [];
    renderSSHServices();
  }).catch(function(e) {
    document.getElementById('sshServiceList').innerHTML = '<tr><td colspan="8" class="empty">无法加载 SSH 服务</td></tr>';
  });
}

function renderSSHServices() {
  var html = '';
  sshServicesCache.forEach(function(s) {
    var activeTag = s.is_active ? '<span class="tag tag-green">启用</span>' : '<span class="tag tag-gray">停用</span>';
    var statusTag = s.apply_status === 'applied'
      ? '<span class="tag tag-green">已应用</span>'
      : '<span class="tag tag-orange">待应用</span>';
    var err = s.last_error ? '<div style="color:#dc2626;font-size:11px;margin-top:3px">' + esc(s.last_error) + '</div>' : '';
    html += '<tr><td>' + s.id + '</td>' +
      '<td><strong>' + esc(s.name) + '</strong></td>' +
      '<td>' + s.remote_port + '</td>' +
      '<td><code>' + esc(s.target_ip) + ':22</code></td>' +
      '<td>' + activeTag + '</td>' +
      '<td>' + statusTag + err + '</td>' +
      '<td>' + esc(s.notes || '-') + '</td>' +
      '<td><button class="btn btn-sm btn-primary" onclick="editSSHService(' + s.id + ')">编辑</button> ' +
      '<button class="btn btn-sm btn-danger" onclick="deleteSSHService(' + s.id + ')">删除</button></td></tr>';
  });
  document.getElementById('sshServiceList').innerHTML = html || '<tr><td colspan="8" class="empty">暂无 SSH 服务</td></tr>';
}

function saveSSHService() {
  var id = parseInt(document.getElementById('sshServiceId').value) || 0;
  var body = {
    name: document.getElementById('sshName').value.trim(),
    target_ip: document.getElementById('sshTargetIP').value.trim(),
    remote_port: parseInt(document.getElementById('sshRemotePort').value) || 0,
    is_active: document.getElementById('sshActive').value === '1',
    notes: document.getElementById('sshNotes').value.trim()
  };
  if (!body.name || !body.target_ip) { toast('服务名称和内网 IP 必填', 'error'); return; }
  var method = id ? 'PUT' : 'POST';
  var path = id ? '/api/ssh-services/' + id : '/api/ssh-services';
  api(method, path, body).then(function(data) {
    resetSSHForm();
    toast(data.apply_error ? '已保存，等待应用：' + data.apply_error : '已保存并应用', data.apply_error ? 'info' : 'success');
    loadSSHServices(); loadPortConfigs(); loadProxies(true); loadAgentStatus();
  }).catch(function(e) { toast('保存 SSH 服务失败: ' + e.message, 'error'); });
}

function editSSHService(id) {
  var s = sshServicesCache.find(function(x) { return x.id === id; });
  if (!s) return;
  document.getElementById('sshServiceId').value = s.id;
  document.getElementById('sshName').value = s.name || '';
  document.getElementById('sshTargetIP').value = s.target_ip || '';
  document.getElementById('sshRemotePort').value = s.remote_port || '';
  document.getElementById('sshActive').value = s.is_active ? '1' : '0';
  document.getElementById('sshNotes').value = s.notes || '';
}

function resetSSHForm() {
  document.getElementById('sshServiceId').value = '';
  document.getElementById('sshName').value = '';
  document.getElementById('sshTargetIP').value = '';
  document.getElementById('sshRemotePort').value = '';
  document.getElementById('sshActive').value = '1';
  document.getElementById('sshNotes').value = '';
}

function deleteSSHService(id) {
  if (!confirm('确认删除这个 SSH 服务？对应公网端口会下线。')) return;
  api('DELETE', '/api/ssh-services/' + id).then(function(data) {
    toast(data.apply_error ? '已删除，等待应用：' + data.apply_error : '已删除并应用', data.apply_error ? 'info' : 'success');
    loadSSHServices(); loadPortConfigs(); loadProxies(true); loadAgentStatus();
  }).catch(function(e) { toast('删除失败: ' + e.message, 'error'); });
}

function applySSHServices() {
  api('POST', '/api/ssh-services/apply').then(function() {
    toast('SSH 服务配置已应用', 'success');
    loadSSHServices(); loadProxies(true); loadAgentStatus();
  }).catch(function(e) {
    toast('应用失败: ' + e.message, 'error');
    loadSSHServices(); loadAgentStatus();
  });
}

function loadAgentStatus() {
  api('GET', '/api/frpc-agent/status').then(function(s) {
    var el = document.getElementById('agentStatus');
    if (!el) return;
    if (s.online === false) {
      el.textContent = 'Agent: 离线 - ' + (s.last_error || '');
      el.style.color = '#dc2626';
    } else {
      el.textContent = 'Agent: 在线, frpc ' + (s.frpc_running ? '运行中' : '未运行') + ', v' + (s.config_version || 0);
      el.style.color = '#059669';
    }
  }).catch(function() {
    var el = document.getElementById('agentStatus');
    if (el) { el.textContent = 'Agent: 离线'; el.style.color = '#dc2626'; }
  });
}

// === IP 限制管理 ===
var ipAllowData = [];

function loadIPAllowlists() {
  api('GET', '/api/ports/ip-allow').then(function(data) {
    ipAllowData = data || [];
    renderIPAllowlist(ipAllowData);
  }).catch(function() {});
}

function renderIPAllowlist(entries) {
  var html = '';
  if (entries.length === 0) {
    html = '<tr><td colspan="6" class="empty">暂无 IP 记录。先在"端口配置"中将端口设为"IP 限制"模式</td></tr>';
  } else {
    entries.forEach(function(e) {
      var mode = (authConfigCache[e.port] === 'ip' && portConfigData[e.port] && portConfigData[e.port].ip_list_mode === 'blacklist') ? '黑名单' : '白名单';
      var modeTag = mode === '黑名单' ? '<span class="tag tag-red">黑名单</span>' : '<span class="tag tag-green">白名单</span>';
      html += '<tr><td><strong>' + e.port + '</strong> <button class="btn btn-sm btn-warning" onclick="toggleIPMode(' + e.port + ')" title="切换白名单/黑名单">切换</button></td>' +
        '<td><code>' + esc(e.ip) + '</code></td>' +
        '<td>' + modeTag + '</td>' +
        '<td>' + esc(e.notes || '-') + '</td>' +
        '<td>' + new Date(e.created_at).toLocaleString('zh-CN') + '</td>' +
        '<td><button class="btn btn-sm btn-danger" onclick="removeIPAllow(' + e.id + ')">移除</button></td></tr>';
    });
  }
  document.getElementById('ipAllowList').innerHTML = html;
}

function batchAddIPs() {
  var port = parseInt(document.getElementById('ipAllowPort').value);
  var ipText = document.getElementById('ipBatchInput').value.trim();
  var notes = document.getElementById('ipAllowNotes').value.trim();
  if (!port) { toast('请输入端口号', 'error'); return; }
  if (!ipText) { toast('请输入 IP 地址', 'error'); return; }
  var ips = ipText.split('\n').map(function(s) { return s.trim(); }).filter(function(s) { return s; });
  if (ips.length === 0) { toast('请输入有效的 IP 地址', 'error'); return; }
  api('POST', '/api/ports/ip-batch', {port: port, ips: ips, notes: notes}).then(function(data) {
    document.getElementById('ipAllowPort').value = '';
    document.getElementById('ipBatchInput').value = '';
    document.getElementById('ipAllowNotes').value = '';
    toast(data.message || '已添加', 'success');
    loadIPAllowlists();
    loadPortConfigs();
  });
}

function toggleIPMode(port) {
  var config = null;
  api('GET', '/api/ports/config').then(function(data) {
    var found = (data || []).find(function(c) { return c.port === port; });
    if (found) config = found;
    var current = (config && config.ip_list_mode) || 'whitelist';
    var next = current === 'whitelist' ? 'blacklist' : 'whitelist';
    var labels = {whitelist: '白名单（仅允许列表IP）', blacklist: '黑名单（禁止列表IP）'};
    if (!confirm('将端口 ' + port + ' 从"' + (labels[current] || current) + '"切换为"' + (labels[next] || next) + '"？')) return;
    api('PUT', '/api/ports/ip-mode', {port: port, ip_list_mode: next}).then(function() {
      toast('端口 ' + port + ' 已切换为 ' + (labels[next] || next), 'success');
      loadIPAllowlists();
      loadPortConfigs();
    });
  });
}

function removeIPAllow(id) {
  if (!confirm('确认移除该 IP 限制？')) return;
  api('DELETE', '/api/ports/ip-allow/' + id).then(function() {
    toast('IP 限制已移除', 'success');
    loadIPAllowlists();
  });
}

// === UI ===
function switchTab(tab) {
  document.querySelectorAll('.tab').forEach(function(t) { t.classList.remove('active'); });
  document.querySelectorAll('.tab-content').forEach(function(c) { c.classList.remove('active'); });
  var btn = document.querySelector('[data-tab="'+tab+'"]');
  if (btn) btn.classList.add('active');
  var panel = document.getElementById('tab-'+tab);
  if (panel) panel.classList.add('active');
  if (tab === 'proxy') loadProxies();
  if (tab === 'ssh') { loadSSHServices(); loadAgentStatus(); }
}
document.querySelectorAll('.tab').forEach(function(t) {
  t.addEventListener('click', function() { switchTab(this.dataset.tab); });
});

// Show/hide IP mode selector when auth mode changes
document.getElementById('portConfigAuth').addEventListener('change', function() {
  var ipMode = document.getElementById('portConfigIPMode');
  if (this.value === 'ip') { ipMode.style.display = 'block'; }
  else { ipMode.style.display = 'none'; }
});

function closeModal() { document.getElementById('tokenModal').classList.remove('show'); }
function copyToken() {
  var val = document.getElementById('modalTokenValue').textContent;
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(val).then(function() { toast('已复制到剪贴板', 'success'); }).catch(function() { fallbackCopy(val); });
  } else { fallbackCopy(val); }
}
function fallbackCopy(text) {
  var ta = document.createElement('textarea');
  ta.value = text; ta.style.position = 'fixed'; ta.style.left = '-9999px'; ta.style.top = '0';
  document.body.appendChild(ta); ta.focus(); ta.select();
  try { document.execCommand('copy'); toast('已复制到剪贴板', 'success'); } catch(e) { toast('复制失败，请手动选中复制', 'error'); }
  document.body.removeChild(ta);
}

function toast(msg, type) {
  var el = document.createElement('div');
  el.className = 'toast toast-' + (type || 'info');
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(function() { el.remove(); }, 3000);
}

function esc(s) { return (s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;'); }

document.getElementById('loginPass').addEventListener('keydown', function(e) { if (e.key === 'Enter') doLogin(); });
if (typeof initSession === 'function') initSession();
initSession();
</script>
</body>
</html>`
