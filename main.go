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
.perm-chip button:hover{color:#dc2626}
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
<option value="true">需要鉴权</option>
<option value="false">开放访问</option>
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
          setInterval(refreshProxy, 10000);
        } else {
          localStorage.removeItem('frp_auth_session');
          sessionToken = '';
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
        setInterval(refreshProxy, 10000);
      } else { toast(data.error || '用户名或密码错误', 'error'); }
    }).catch(function() { toast('登录失败，请检查网络连接', 'error'); });
}

function doLogout() {
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

function loadAll() { loadProxies(); loadTokens(); loadPortConfigs(); loadPermissions(); }

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
  return authConfigCache[port]
    ? '<span class="tag tag-green">需要鉴权</span>'
    : '<span class="tag tag-gray">开放访问</span>';
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
  var requireAuth = document.getElementById('portConfigAuth').value === 'true';
  if (!port) { toast('请输入端口号', 'error'); return; }
  api('POST', '/api/ports/config', {port: port, require_auth: requireAuth}).then(function() {
    document.getElementById('portConfigPort').value = '';
    toast('端口 ' + port + ' 已设为：' + (requireAuth ? '需要鉴权' : '开放访问'), 'success');
    loadPortConfigs();
    loadProxies(true);
  });
}

function loadPortConfigs(silent) {
  api('GET', '/api/ports/config').then(function(data) {
    authConfigCache = {};
    (data || []).forEach(function(c) { authConfigCache[c.port] = c.require_auth; });
    // Mark all known proxy ports not in config as open
    proxyPorts.forEach(function(p) {
      if (authConfigCache[p] === undefined) authConfigCache[p] = false;
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
    if (!allPorts[p]) allPorts[p] = {port: p, require_auth: false, id: 0, created_at: null};
  });
  // Also include 6500,6501 if they showed up in any proxy
  var sortedPorts = Object.values(allPorts).sort(function(a,b) { return a.port - b.port; });

  // Build proxy name map
  var proxyNameMap = {};
  proxyDataCache.forEach(function(p) {
    if (p.conf && p.conf.remotePort) proxyNameMap[p.conf.remotePort] = p.name;
  });

  var html = '';
  sortedPorts.forEach(function(c) {
    var tag = c.require_auth ? '<span class="tag tag-green">需要鉴权</span>' : '<span class="tag tag-gray">开放访问</span>';
    var proxyName = proxyNameMap[c.port] || '-';
    var action = c.id > 0
      ? '<button class="btn btn-sm btn-danger" onclick="deletePortConfigById(' + c.id + ')">删除鉴权配置</button>'
      : '<button class="btn btn-sm btn-warning" onclick="quickAddAuth(' + c.port + ')">添加鉴权</button>';
    html += '<tr><td><strong>' + c.port + '</strong></td><td>' + esc(proxyName) + '</td><td>' + tag + '</td><td>' + action + '</td></tr>';
  });
  document.getElementById('portConfigList').innerHTML = html || '<tr><td colspan="4" class="empty">暂未检测到端口</td></tr>';
}

function quickAddAuth(port) {
  api('POST', '/api/ports/config', {port: port, require_auth: true}).then(function() {
    toast('端口 ' + port + ' 已设为需要鉴权', 'success');
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

// === UI ===
function switchTab(tab) {
  document.querySelectorAll('.tab').forEach(function(t) { t.classList.remove('active'); });
  document.querySelectorAll('.tab-content').forEach(function(c) { c.classList.remove('active'); });
  var btn = document.querySelector('[data-tab="'+tab+'"]');
  if (btn) btn.classList.add('active');
  var panel = document.getElementById('tab-'+tab);
  if (panel) panel.classList.add('active');
  if (tab === 'proxy') loadProxies();
}
document.querySelectorAll('.tab').forEach(function(t) {
  t.addEventListener('click', function() { switchTab(this.dataset.tab); });
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
