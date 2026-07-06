package internal

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

type AdminServer struct {
	Pool   *KeyPoolConfig
	Models *ModelManager
	Proxy  *Proxy
	Config *ServerConfig
}

type ServerConfig struct {
	ThinkingLang string `json:"thinking_lang"`
	Debug        bool   `json:"debug"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
}

func NewAdminServer(pool *KeyPoolConfig, models *ModelManager, proxy *Proxy, config *ServerConfig) *AdminServer {
	return &AdminServer{Pool: pool, Models: models, Proxy: proxy, Config: config}
}

func (as *AdminServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/", as.handlePage)
	mux.HandleFunc("/admin/api/login", as.handleLogin)
	mux.HandleFunc("/admin/api/logout", as.handleLogout)
	mux.HandleFunc("/admin/api/keys", as.handleKeysAPI)
	mux.HandleFunc("/admin/api/keys/test", as.handleKeysTest)
	mux.HandleFunc("/admin/api/models", as.handleModelsAPI)
	mux.HandleFunc("/admin/api/settings", as.handleSettingsAPI)
}

func (as *AdminServer) checkAuth(r *http.Request) bool {
	cookie, err := r.Cookie("cp_admin_token")
	if err != nil {
		return false
	}
	return as.Pool.ValidateSessionToken(cookie.Value)
}

func (as *AdminServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		http.Error(w, `{"error":"method not allowed"}`, 405)
		return
	}
	var body map[string]string
	json.NewDecoder(r.Body).Decode(&body)
	password := body["password"]
	if as.Pool.CheckPassword(password) {
		token := as.Pool.GenerateSessionToken()
		http.SetCookie(w, &http.Cookie{
			Name: "cp_admin_token", Value: token,
			Path: "/admin/", HttpOnly: false,
			MaxAge: 86400 * 7,
		})
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "default_password": as.Pool.IsPasswordDefault()})
	} else {
		http.Error(w, `{"error":"密码错误"}`, 401)
	}
}

func (as *AdminServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	http.SetCookie(w, &http.Cookie{Name: "cp_admin_token", Value: "", Path: "/admin/", MaxAge: -1})
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (as *AdminServer) handlePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, _ := template.New("admin").Parse(adminPageHTML)
	tmpl.Execute(w, map[string]interface{}{
		"ThinkingLang":    as.Config.ThinkingLang,
		"CooldownHour":    as.Pool.CooldownHour,
		"MaxRetries":      as.Pool.MaxCooldownRetries,
		"Authenticated":   as.checkAuth(r),
		"PasswordDefault": as.Pool.IsPasswordDefault(),
	})
}

func (as *AdminServer) handleKeysAPI(w http.ResponseWriter, r *http.Request) {
	if !as.checkAuth(r) {
		http.Error(w, `{"error":"unauthorized"}`, 401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		as.Pool.mu.Lock()
		res := make([]map[string]interface{}, len(as.Pool.Keys))
		for i, k := range as.Pool.Keys {
			res[i] = map[string]interface{}{
				"id": k.ID, "key_prefix": safePrefix(k.Key),
				"status": k.Status, "status_label": k.StatusLabel(),
				"note": k.Note, "last_used": fmtTime(k.LastUsed),
				"last_failed": fmtTime(k.LastFailed), "fail_count": k.FailCount,
				"cooldown_until": fmtTime(k.CooldownUntil),
				"weekly_reset":   fmtTime(k.WeeklyReset),
				"created_at":     fmtTime(k.CreatedAt),
			}
		}
		as.Pool.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{"keys": res, "total": len(res)})
	case "POST":
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		key, note := body["key"], body["note"]
		if key == "" {
			http.Error(w, `{"error":"key required"}`, 400)
			return
		}
		e := as.Pool.AddKey(key, note)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": e.ID})
	case "DELETE":
		idStr := r.URL.Query().Get("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, 400)
			return
		}
		if as.Pool.RemoveKey(id) {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		} else {
			http.Error(w, `{"error":"not found"}`, 404)
		}
	default:
		http.Error(w, `{"error":"method not allowed"}`, 405)
	}
}

func (as *AdminServer) handleKeysTest(w http.ResponseWriter, r *http.Request) {
	if !as.checkAuth(r) {
		http.Error(w, `{"error":"unauthorized"}`, 401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"results": as.Pool.TestAllKeys()})
}

func (as *AdminServer) handleModelsAPI(w http.ResponseWriter, r *http.Request) {
	if !as.checkAuth(r) {
		http.Error(w, `{"error":"unauthorized"}`, 401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		all := as.Models.AllModels()
		type modelX struct {
			Model
			Custom bool `json:"custom"`
			Hidden bool `json:"hidden"`
			Edited bool `json:"edited"`
		}
		res := make([]modelX, len(all))
		for i, m := range all {
			res[i] = modelX{Model: m, Custom: as.Models.IsCustom(m.ID), Hidden: as.Pool.IsModelHidden(m.ID), Edited: as.Models.IsOverridden(m.ID)}
		}
		for _, hm := range as.Models.AllModelsWithHidden() {
			if !as.Pool.IsModelHidden(hm.ID) {
				continue
			}
			res = append(res, modelX{Model: hm, Custom: as.Models.IsCustom(hm.ID), Hidden: true, Edited: as.Models.IsOverridden(hm.ID)})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"models": res, "total": len(res)})
	case "POST":
		var m Model
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			http.Error(w, `{"error":"invalid json"}`, 400)
			return
		}
		as.Models.AddCustom(m)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	case "PUT":
		var m Model
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil || m.ID == "" {
			http.Error(w, `{"error":"invalid json or missing id"}`, 400)
			return
		}
		as.Models.UpdateModel(m)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	case "DELETE":
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			http.Error(w, `{"error":"id required"}`, 400)
			return
		}
		if as.Models.RemoveModel(idStr) {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "hidden": true})
		}
	default:
		http.Error(w, `{"error":"method not allowed"}`, 405)
	}
}

func (as *AdminServer) handleSettingsAPI(w http.ResponseWriter, r *http.Request) {
	if !as.checkAuth(r) {
		http.Error(w, `{"error":"unauthorized"}`, 401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		json.NewEncoder(w).Encode(map[string]interface{}{
			"thinking_lang":        as.Config.ThinkingLang,
			"cooldown_hour":        as.Pool.CooldownHour,
			"max_cooldown_retries": as.Pool.MaxCooldownRetries,
			"password_status":      as.Pool.PasswordStatusLabel(),
		})
	case "PUT":
		var s map[string]interface{}
		json.NewDecoder(r.Body).Decode(&s)
		if v, ok := s["thinking_lang"].(string); ok {
			as.Config.ThinkingLang = v
			as.Proxy.ThinkingLang = v
		}
		if v, ok := s["cooldown_hour"].(float64); ok {
			as.Pool.CooldownHour = int(v)
			as.Pool.dirty = true
		}
		if v, ok := s["max_cooldown_retries"].(float64); ok {
			as.Pool.MaxCooldownRetries = int(v)
			as.Pool.dirty = true
		}
		if oldPw, ok := s["old_password"].(string); ok {
			if newPw, ok2 := s["new_password"].(string); ok2 && len(newPw) >= 4 {
				if as.Pool.CheckPassword(oldPw) {
					as.Pool.SetAdminPassword(newPw)
					token := as.Pool.GenerateSessionToken()
					http.SetCookie(w, &http.Cookie{Name: "cp_admin_token", Value: token, Path: "/admin/", MaxAge: 86400 * 7})
					json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "password_changed": true, "token": token})
					return
				} else {
					http.Error(w, `{"error":"旧密码错误"}`, 403)
					return
				}
			}
		}
		as.Pool.saveIfDirty()
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	default:
		http.Error(w, `{"error":"method not allowed"}`, 405)
	}
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

// ModelManager
type ModelManager struct {
	custom map[string]Model
	pool   *KeyPoolConfig
}

func NewModelManager(pool *KeyPoolConfig) *ModelManager {
	mm := &ModelManager{custom: make(map[string]Model), pool: pool}
	for id, m := range pool.ModelOverrides {
		if !isBuiltinModel(id) {
			mm.custom[id] = m
		}
	}
	return mm
}

func isBuiltinModel(id string) bool {
	for _, m := range ClinePassModels {
		if m.ID == id {
			return true
		}
	}
	return false
}

func (mm *ModelManager) AllModels() []Model {
	var r []Model
	for _, m := range ClinePassModels {
		if mm.pool.IsModelHidden(m.ID) {
			continue
		}
		r = append(r, mm.applyOverrides(m))
	}
	for _, m := range mm.custom {
		if !mm.pool.IsModelHidden(m.ID) {
			r = append(r, m)
		}
	}
	return r
}

func (mm *ModelManager) AllModelsWithHidden() []Model {
	var r []Model
	for _, m := range ClinePassModels {
		r = append(r, mm.applyOverrides(m))
	}
	for _, m := range mm.custom {
		r = append(r, m)
	}
	return r
}

func (mm *ModelManager) applyOverrides(m Model) Model {
	if ov, ok := mm.pool.GetModelOverride(m.ID); ok {
		if ov.Name != "" {
			m.Name = ov.Name
		}
		if ov.ContextLength != 0 {
			m.ContextLength = ov.ContextLength
		}
		if ov.MaxOutput != 0 {
			m.MaxOutput = ov.MaxOutput
		}
		if ov.InputPrice != 0 {
			m.InputPrice = ov.InputPrice
		}
		if ov.OutputPrice != 0 {
			m.OutputPrice = ov.OutputPrice
		}
		if ov.SupportsVision {
			m.SupportsVision = ov.SupportsVision
		}
		if ov.SupportsVideo {
			m.SupportsVideo = ov.SupportsVideo
		}
		if ov.SupportsAudio {
			m.SupportsAudio = ov.SupportsAudio
		}
		if ov.SupportsReason {
			m.SupportsReason = ov.SupportsReason
		}
		if ov.SupportsTools {
			m.SupportsTools = ov.SupportsTools
		}
	}
	return m
}

func (mm *ModelManager) IsCustom(id string) bool { _, ok := mm.custom[id]; return ok }
func (mm *ModelManager) IsOverridden(id string) bool { _, ok := mm.pool.ModelOverrides[id]; return ok }

func (mm *ModelManager) AddCustom(m Model) {
	mm.custom[m.ID] = m
	mm.pool.SetModelOverride(m.ID, m)
	mm.pool.UnhideModel(m.ID)
}

func (mm *ModelManager) RemoveModel(id string) bool {
	if _, ok := mm.custom[id]; ok {
		delete(mm.custom, id)
		mm.pool.RemoveModelOverride(id)
		return true
	}
	mm.pool.HideModel(id)
	return false
}

func (mm *ModelManager) UpdateModel(m Model) {
	if _, ok := mm.custom[m.ID]; ok {
		mm.custom[m.ID] = m
	}
	mm.pool.SetModelOverride(m.ID, m)
	mm.pool.UnhideModel(m.ID)
}

const adminPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>ClinePass Proxy 管理</title>
<style id="theme-style">
:root{--bg:#f5efe4;--card:#fbf7ee;--border:#d8cfbe;--text:#2a2622;--text2:#4a433c;--text3:#6b6158;--acc:#537d96;--acc2:#3f6179;--err:#8b2c1f;--warn:#9d5f4d;--ok:#4a6b4a;--pre:#efe8db;--input:#fffdf7;--shade:rgba(0,0,0,0.3);--okbg:rgba(74,107,74,0.1);--warnbg:rgba(157,95,77,0.1);--errbg:rgba(139,44,31,0.08);--accbg:rgba(83,125,150,0.08)}
.dark{--bg:#1a1a22;--card:#25252e;--border:#353540;--text:#ddd9d2;--text2:#bbb7ae;--text3:#8a867e;--acc:#5f96b8;--acc2:#427a9a;--err:#c96a55;--warn:#b8945a;--ok:#6a9a6a;--pre:#2e2e38;--input:#2e2e38;--shade:rgba(0,0,0,0.6);--okbg:rgba(106,154,106,0.12);--warnbg:rgba(184,148,90,0.12);--errbg:rgba(201,106,85,0.12);--accbg:rgba(95,150,184,0.12)}
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,'Segoe UI',sans-serif;background:var(--bg);color:var(--text);font-size:14px;padding:20px;transition:background .2s,color .2s}
h1{font-size:1.3rem;font-weight:500;margin-bottom:16px;display:flex;align-items:center;gap:10px}
h1 .sub{font-size:12px;color:var(--text3);font-weight:400}
.nav{display:flex;gap:8px;margin-bottom:20px;border-bottom:1px solid var(--border);flex-wrap:wrap}
.nav button,.nav .sp{padding:8px 18px;border:none;background:transparent;color:var(--text3);cursor:pointer;font-size:14px;border-bottom:2px solid transparent;margin-bottom:-1px}
.nav button.active{color:var(--acc);border-bottom-color:var(--acc);font-weight:500}
.nav .sp{flex:1;cursor:default}
.panel{display:none}.panel.active{display:block}
.card{background:var(--card);border:1px solid var(--border);border-radius:4px;padding:16px 20px;margin-bottom:16px}
.card h3{font-size:1rem;font-weight:500;margin-bottom:12px}
.flex{display:flex;flex-wrap:wrap;gap:12px;align-items:center}
.btn{border:1px solid var(--border);border-radius:4px;background:transparent;color:var(--acc);padding:6px 14px;font-size:13px;cursor:pointer}
.btn:hover{background:var(--accbg)}
.btn-primary{background:var(--acc);color:#fff;border-color:var(--acc)}
.btn-primary:hover{background:var(--acc2)}
.btn-danger{color:var(--err);border-color:var(--err)}
.btn-danger:hover{background:var(--errbg)}
.btn-sm{padding:3px 10px;font-size:12px}
input,select{font-size:13px;padding:6px 10px;border:1px solid var(--border);border-radius:4px;background:var(--input);color:var(--text)}
input:focus,select:focus{outline:none;border-color:var(--acc)}
table{width:100%;border-collapse:collapse;font-size:13px}
th{text-align:left;font-weight:500;color:var(--text2);padding:8px 6px;border-bottom:1px solid var(--border)}
td{padding:6px;border-bottom:1px solid rgba(0,0,0,0.06);color:var(--text)}
tr:last-child td{border-bottom:none}
.tag{display:inline-flex;align-items:center;padding:2px 8px;font-size:12px;border-radius:4px;font-weight:500}
.tag-ok{background:var(--okbg);color:var(--ok)}
.tag-warn{background:var(--warnbg);color:var(--warn)}
.tag-err{background:var(--errbg);color:var(--err)}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:10px}
.grid-item{border:1px solid var(--border);border-radius:4px;padding:10px 12px;font-size:12px;position:relative}
.grid-item .mn{font-weight:500;color:var(--text);padding-right:50px}
.grid-item .mi{color:var(--text3);margin-top:2px;font-size:11px;word-break:break-all}
.grid-item .mm{color:var(--text3);margin-top:4px;font-size:11px}
.grid-item .mbtns{position:absolute;top:6px;right:6px;display:none;gap:4px}
.grid-item:hover .mbtns{display:flex}
.grid-item .mbtns button{padding:1px 7px;font-size:11px;border:1px solid var(--border);border-radius:3px;background:transparent;cursor:pointer}
.grid-item .mbtns .e{color:var(--acc)}
.grid-item .mbtns .e:hover{background:var(--accbg)}
.grid-item .mbtns .d{color:var(--err)}
.grid-item .mbtns .d:hover{background:var(--errbg)}
.grid-item.hidden{opacity:0.4}
.grid-item.hidden .mn::after{content:" (已隐藏)";font-size:11px;color:var(--err);font-weight:400}
.search{margin-bottom:10px}
.search input{width:100%;padding:8px 12px;font-size:13px}
pre{background:var(--pre);padding:8px;border-radius:4px;font-size:12px;overflow-x:auto}
.waiting{opacity:0.5;pointer-events:none}
.mt-8{margin-top:8px}.gap-sm{gap:6px}.text-muted{color:var(--text3)}.text-sm{font-size:12px}
.sdot{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:5px}
.sg{background:var(--ok)}.sy{background:var(--warn)}.sr{background:var(--err)}
.modal{display:none;position:fixed;top:0;left:0;right:0;bottom:0;background:var(--shade);z-index:100;align-items:center;justify-content:center}
.modal.show{display:flex}
.modal-box{background:var(--card);border:1px solid var(--border);border-radius:6px;padding:20px;width:480px;max-width:95vw;max-height:90vh;overflow-y:auto}
.modal-box h3{margin-bottom:14px}
.modal-box label{display:block;font-size:12px;color:var(--text3);margin-top:8px;margin-bottom:2px}
.modal-box input{width:100%}
.modal-box .row{display:flex;gap:10px}
.modal-box .row>div{flex:1}
.modal-box .fe{display:flex;justify-content:flex-end;gap:8px;margin-top:16px}
.modal-box .cg{display:flex;flex-wrap:wrap;gap:10px;margin-top:6px}
.modal-box .cg label{display:inline-flex;align-items:center;gap:4px;font-size:13px;color:var(--text);margin:0}
#lo{position:fixed;top:0;left:0;right:0;bottom:0;background:var(--bg);z-index:200;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:20px}
#lo.hide{display:none}
#lo .b{background:var(--card);border:1px solid var(--border);border-radius:6px;padding:30px;width:340px;max-width:90vw;text-align:center}
#lo .b h2{margin-bottom:8px;font-weight:500}
#lo .b p{color:var(--text3);font-size:13px;margin-bottom:16px}
#lo .b input{width:100%;text-align:center;margin-bottom:10px}
#lo .b .er{color:var(--err);font-size:12px;margin-bottom:8px;display:none}
#lo .v{color:var(--text3);font-size:12px}
</style>
</head>
<body>
<div id="lo"{{if .Authenticated}} class="hide"{{end}}>
<div class="b">
<h2>ClinePass Proxy 管理</h2>
<p>请输入管理密码登录</p>
<div class="er" id="le"></div>
<input id="lp" type="password" placeholder="密码" onkeydown="if(event.key==='Enter')lg()">
<button class="btn btn-primary" onclick="lg()" style="width:100%" id="lb">登录</button>
</div>
<div class="v">v3.0.0</div>
</div>

<h1>ClinePass Proxy<span class="sub">v3.0.0</span></h1>
<div class="nav">
<button class="active" onclick="st('keys')">Key 管理 ({{.CooldownHour}}h轮换)</button>
<button onclick="st('models')">模型管理</button>
<button onclick="st('settings')">设置</button>
<span class="sp"></span>
<button onclick="td()" title="切换深色模式" id="db">\u{1F319}</button>
<button onclick="lo2()" title="退出登录">\u{1F6AA}</button>
</div>

<div id="pk" class="panel active">
<div class="card">
<h3>添加 Key</h3>
<div class="flex">
<input id="nk" type="text" placeholder="sk_..." style="flex:1;min-width:200px">
<input id="nn" type="text" placeholder="备注(可选)" style="width:120px">
<button class="btn btn-primary" onclick="addKey()">添加</button>
<button class="btn" onclick="testAll()">一键检测</button>
</div>
</div>
<div class="card">
<h3>Key 列表 <span id="kc" class="text-muted text-sm"></span></h3>
<table><thead><tr><th>#</th><th>Key</th><th>状态</th><th>备注</th><th>上次使用</th><th>上次失败</th><th>失败</th><th></th></tr></thead>
<tbody id="kl"></tbody></table>
</div>
</div>

<div id="pm" class="panel">
<div class="card">
<h3>添加自定义模型</h3>
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(130px,1fr));gap:10px">
<input id="mi" placeholder="模型 ID" style="width:100%">
<input id="mn2" placeholder="显示名称" style="width:100%">
<input id="mc" type="number" placeholder="上下文" value="1000000" style="width:100%">
<input id="mo" type="number" placeholder="最大输出" value="131072" style="width:100%">
</div>
<div class="flex mt-8 gap-sm">
<label><input type="checkbox" id="mv"> 视觉</label>
<label><input type="checkbox" id="mvid"> 视频</label>
<label><input type="checkbox" id="ma"> 音频</label>
<label><input type="checkbox" id="mr" checked> 推理</label>
<label><input type="checkbox" id="mt" checked> 工具</label>
<button class="btn btn-primary" onclick="addModel()">添加</button>
</div>
</div>
<div class="card">
<h3>所有模型 <span id="mc2" class="text-muted text-sm"></span></h3>
<div class="search"><input id="ms" type="text" placeholder="搜索模型名称或 ID..." oninput="fm()"></div>
<div id="ml" class="grid"></div>
</div>
</div>

<div id="ps" class="panel">
<div class="card">
<h3>号池设置</h3>
<div class="flex">
<label>冷却(小时): <input id="sc" type="number" value="{{.CooldownHour}}" style="width:60px"></label>
<label>最大重试: <input id="sr" type="number" value="{{.MaxRetries}}" style="width:60px"></label>
<label>思考语言: <select id="sl"><option value="zh" {{if eq .ThinkingLang "zh"}}selected{{end}}>中文</option><option value="en" {{if eq .ThinkingLang "en"}}selected{{end}}>English</option></select></label>
<button class="btn btn-primary" onclick="saveSet()">保存</button>
</div>
</div>
<div class="card">
<h3>修改密码{{if .PasswordDefault}} <span class="tag tag-warn">默认密码未修改</span>{{end}}</h3>
<div class="flex">
<input id="opw" type="password" placeholder="当前密码" style="width:130px">
<input id="npw" type="password" placeholder="新密码(4位+)" style="width:150px">
<input id="cpw" type="password" placeholder="确认新密码" style="width:130px">
<button class="btn btn-primary" onclick="cp()">修改</button>
</div>
</div>
<div class="card">
<h3>API 端点</h3>
<pre>/v1/chat/completions (OpenAI 兼容)
/v1/models (模型列表)
/admin/ (管理面板)
/health (健康检查)</pre>
</div>
</div>

<div id="em" class="modal">
<div class="modal-box">
<h3>编辑模型</h3>
<input type="hidden" id="eid">
<label>显示名称</label><input id="en" placeholder="名称">
<div class="row"><div><label>上下文长度</label><input id="ec" type="number"></div><div><label>最大输出</label><input id="eo" type="number"></div></div>
<div class="row"><div><label>输入价格 (/M)</label><input id="epi" type="number" step="0.01"></div><div><label>输出价格 (/M)</label><input id="epo" type="number" step="0.01"></div></div>
<label>能力</label>
<div class="cg">
<label><input type="checkbox" id="ev"> 视觉</label>
<label><input type="checkbox" id="evid"> 视频</label>
<label><input type="checkbox" id="ea"> 音频</label>
<label><input type="checkbox" id="er"> 推理</label>
<label><input type="checkbox" id="et"> 工具</label>
</div>
<div class="fe"><button class="btn" onclick="cm()">取消</button><button class="btn btn-primary" onclick="sme()">保存</button></div>
</div>
</div>

<script>
async function api(u,o){try{const r=await fetch(u,o||{});if(r.status===401){document.getElementById('lo').classList.remove('hide');document.getElementById('le').textContent='登录已过期，请重新登录';document.getElementById('le').style.display='block';return{error:'unauth'}}return await r.json()}catch(e){return{error:e.message}}}
async function lg(){const p=document.getElementById('lp').value;if(!p)return;const b=document.getElementById('lb');b.textContent='登录中...';b.disabled=true;const r=await(await fetch('/admin/api/login',{method:'POST',body:JSON.stringify({password:p})})).json();b.textContent='登录';b.disabled=false;if(r.ok){document.getElementById('lo').classList.add('hide');if(r.default_password)setTimeout(()=>alert('建议立即修改默认密码'),300);lk();lm()}else{document.getElementById('le').textContent=r.error||'密码错误';document.getElementById('le').style.display='block'}}
async function lo2(){await fetch('/admin/api/logout',{method:'POST'});location.reload()}
function td(){const d=document.body.classList.toggle('dark');localStorage.setItem('cpd',d?'1':'0');document.getElementById('db').textContent=d?'\u{2600}\u{FE0F}':'\u{1F319}'}
(function(){if(localStorage.getItem('cpd')==='1'){document.body.classList.add('dark');document.getElementById('db').textContent='\u{2600}\u{FE0F}'}})()
function st(n){document.querySelectorAll('.panel').forEach(p=>p.classList.remove('active'));document.getElementById('p'+n[0]).classList.add('active');document.querySelectorAll('.nav button:not(.sp)').forEach(b=>b.classList.remove('active'));[['keys','models','settings'].indexOf(n)].forEach(i=>{if(i>=0)document.querySelectorAll('.nav button:not(.sp)')[i].classList.add('active')})}
function sd(s){return '<span class="sdot '+(s==='active'?'sg':s==='cooldown'?'sy':'sr')+'"></span>'}
async function lk(){const r=await api('/admin/api/keys');if(!r||!r.keys)return;document.getElementById('kc').textContent='('+r.total+')';document.getElementById('kl').innerHTML=r.keys.map(k=>'<tr><td>'+k.id+'</td><td><code>'+k.key_prefix+'</code></td><td>'+sd(k.status)+k.status_label+'</td><td>'+(k.note||'-')+'</td><td class="text-muted">'+(k.last_used||'-')+'</td><td class="text-muted">'+(k.last_failed||'-')+'</td><td>'+k.fail_count+'</td><td><button class="btn btn-danger btn-sm" onclick="dk('+k.id+')">删除</button></td></tr>').join('')}
async function addKey(){const k=document.getElementById('nk').value.trim();if(!k){alert('请输入 Key');return}const n=document.getElementById('nn').value.trim();const r=await api('/admin/api/keys',{method:'POST',body:JSON.stringify({key:k,note:n})});if(r&&r.ok){document.getElementById('nk').value='';document.getElementById('nn').value='';lk()}else alert(r.error||'添加失败')}
async function dk(id){if(!confirm('删除 Key '+id+'?'))return;const r=await api('/admin/api/keys?id='+id,{method:'DELETE'});if(r&&r.ok)lk()}
async function testAll(){const btn=document.querySelector('.nav button');btn.classList.add('waiting');const r=await api('/admin/api/keys/test');btn.classList.remove('waiting');if(r&&r.results){let h='<div style="display:flex;flex-direction:column;gap:6px">'+r.results.map(x=>'<div class="flex gap-sm"><code>'+x.key+'</code>'+(x.ok?'<span class="tag tag-ok"> 可用</span>':'<span class="tag tag-err">  '+x.msg+'</span>')+'</div>').join('')+'</div>';document.querySelector('#pk .card:first-child').innerHTML='<h3>检测结果</h3>'+h+'<button class="btn mt-8" onclick="location.reload()">返回</button>'}lk()}
let _am=[]
async function lm(){const r=await api('/admin/api/models');if(!r||!r.models)return;_am=r.models;rm()}
function rm(){const q=(document.getElementById('ms').value||'').toLowerCase();const el=document.getElementById('ml');const fl=q?_am.filter(m=>m.name.toLowerCase().includes(q)||m.id.toLowerCase().includes(q)):_am;document.getElementById('mc2').textContent='(共 '+_am.length+' 个'+(q?'，筛选 '+fl.length:'')+')';el.innerHTML=fl.map(m=>'<div class="grid-item'+(m.hidden?' hidden':'')+'"><div class="mn">'+m.name+'</div><div class="mi">'+m.id+'</div><div class="mm">上下文 '+fmt(m.context_length)+' | 最大输出 '+fmt(m.max_output||Math.round(m.context_length/4))+'</div><div class="flex gap-sm" style="margin-top:6px">'+(m.supports_vision?'<span class="tag tag-ok">视觉</span>':'')+(m.supports_video?'<span class="tag tag-ok">视频</span>':'')+(m.supports_audio?'<span class="tag tag-ok">音频</span>':'')+(m.supports_reasoning?'<span class="tag tag-warn">推理</span>':'')+(m.supports_tools?'<span class="tag tag-ok">工具</span>':'')+(m.edited?'<span class="tag tag-warn">已编辑</span>':'')+'</div><div class="mbtns"><button class="e" onclick="oe(\''+m.id+'\')">编辑</button><button class="d" onclick="dm(\''+m.id+'\')">'+(m.hidden?'恢复':'隐藏')+'</button></div></div>').join('')}
function fm(){rm()}
function fmt(n){if(n>=1000000)return (n/1000000).toFixed(1)+'M';if(n>=1000)return (n/1000).toFixed(0)+'K';return n.toLocaleString()}
async function dm(id){if(!confirm('确定隐藏模型 '+id+'？你可以随时在列表中恢复。'))return;const r=await api('/admin/api/models?id='+encodeURIComponent(id),{method:'DELETE'});if(r&&r.ok)lm()}
async function addModel(){const id=document.getElementById('mi').value.trim();if(!id){alert('请输入模型 ID');return}const m={id:id,name:document.getElementById('mn2').value||id,context_length:parseInt(document.getElementById('mc').value)||1000000,max_output:parseInt(document.getElementById('mo').value)||131072,supports_vision:document.getElementById('mv').checked,supports_video:document.getElementById('mvid').checked,supports_audio:document.getElementById('ma').checked,supports_reasoning:document.getElementById('mr').checked,supports_tools:document.getElementById('mt').checked};const r=await api('/admin/api/models',{method:'POST',body:JSON.stringify(m)});if(r&&r.ok){document.getElementById('mi').value='';document.getElementById('mn2').value='';lm()}else alert(r.error||'添加失败')}
let _eid=''
function oe(id){_eid=id;const m=_am.find(x=>x.id===id);if(!m){alert('找不到模型');return}
document.getElementById('eid').value=m.id;document.getElementById('en').value=m.name||''
document.getElementById('ec').value=m.context_length||1000000;document.getElementById('eo').value=m.max_output||Math.round((m.context_length||1000000)/4)
document.getElementById('epi').value=m.input_price_per_million||0;document.getElementById('epo').value=m.output_price_per_million||0
document.getElementById('ev').checked=m.supports_vision||false;document.getElementById('evid').checked=m.supports_video||false
document.getElementById('ea').checked=m.supports_audio||false;document.getElementById('er').checked=m.supports_reasoning||false
document.getElementById('et').checked=m.supports_tools||false;document.getElementById('em').classList.add('show')}
function cm(){document.getElementById('em').classList.remove('show')}
async function sme(){const m={id:_eid,name:document.getElementById('en').value.trim()||_eid,context_length:parseInt(document.getElementById('ec').value)||1000000,max_output:parseInt(document.getElementById('eo').value)||0,input_price_per_million:parseFloat(document.getElementById('epi').value)||0,output_price_per_million:parseFloat(document.getElementById('epo').value)||0,supports_vision:document.getElementById('ev').checked,supports_video:document.getElementById('evid').checked,supports_audio:document.getElementById('ea').checked,supports_reasoning:document.getElementById('er').checked,supports_tools:document.getElementById('et').checked};const r=await api('/admin/api/models',{method:'PUT',body:JSON.stringify(m)});if(r&&r.ok){cm();lm()}else alert(r.error||'保存失败')}
async function saveSet(){const s={thinking_lang:document.getElementById('sl').value,cooldown_hour:parseInt(document.getElementById('sc').value)||5,max_cooldown_retries:parseInt(document.getElementById('sr').value)||3};const r=await api('/admin/api/settings',{method:'PUT',body:JSON.stringify(s)});if(r&&r.ok)alert('已保存')}
async function cp(){const op=document.getElementById('opw').value;const np=document.getElementById('npw').value;const cp=document.getElementById('cpw').value;if(!op||!np||!cp){alert('请填写所有密码字段');return}
if(np!==cp){alert('两次新密码不一致');return}
if(np.length<4){alert('新密码至少4位');return}
const r=await api('/admin/api/settings',{method:'PUT',body:JSON.stringify({old_password:op,new_password:np})});if(r&&r.ok){alert('密码已修改');document.getElementById('opw').value='';document.getElementById('npw').value='';document.getElementById('cpw').value=''}else alert(r.error||'修改失败')}
if(!document.getElementById('lo').classList.contains('hide')){document.getElementById('lp').focus()}else{lk();lm()}
</script>
</body>
</html>`
