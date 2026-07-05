# ClinePass Proxy

> 将 ClinePass API 转换为标准 OpenAI API 的轻量代理，支持思考过程（reasoning）透传、多模态、工具调用等全部能力。

ClinePass 是 Cline 的 $9.99/月订阅服务，提供 10 个精选开源编码模型（GLM-5.2、Kimi K2.7 Code、DeepSeek V4 Pro 等）。但 ClinePass 的 API Key 只能在 Cline 官方软件中使用，无法直接在 CherryStudio、OpenCat、LobeChat 等第三方客户端调用。

**ClinePass Proxy 解决了这个问题**——它在中间做一层透明代理，兼容 OpenAI 格式，任何支持 OpenAI API 的客户端都可以直接使用 ClinePass 的模型。

---

## 核心功能

- ✅ **OpenAI 兼容** — 标准的 `/v1/chat/completions` 和 `/v1/models` 端点
- ✅ **思考过程透传** — 将 `reasoning` / `reasoning_details` 自动转为 `reasoning_content`
- ✅ **全能力支持** — 视觉、工具调用、视频理解等能力完整保留
- ✅ **中文思考** — 默认强制模型用中文思考（可切换英文）
- ✅ **流式+非流式** — 完整 SSE 流式支持
- ✅ **API Key 优先级** — 服务端配置优先，客户端可随意填写
- ✅ **自动推理注入** — MiniMax 自动加 `reasoning_split`，DeepSeek 自动加 `thinking`
- ✅ **完整模型目录** — `/v1/models` 返回上下文长度、能力标记、价格等元数据
- ✅ **轻量高效** — 单二进制无依赖，内存占用 ~1MB

## 支持模型

| 模型 ID | 名称 | 上下文 | 视觉 | 视频 | 音频 | 推理 |
|:--|:--|:--:|:--:|:--:|:--:|:--:|
| `cline-pass/glm-5.2` | GLM-5.2 | 1M | ✅ | ❌ | ❌ | ✅ |
| `cline-pass/kimi-k2.7-code` | Kimi K2.7 Code | 262K | ✅ | ❌ | ❌ | ✅ |
| `cline-pass/kimi-k2.6` | Kimi K2.6 | 262K | ✅ | ❌ | ❌ | ✅ |
| `cline-pass/deepseek-v4-pro` | DeepSeek V4 Pro | 1M | ❌ | ❌ | ❌ | ✅ |
| `cline-pass/deepseek-v4-flash` | DeepSeek V4 Flash | 1M | ❌ | ❌ | ❌ | ✅ |
| `cline-pass/mimo-v2.5` | MiMo V2.5 | 1M | ✅ | ✅ | ✅ | ✅ |
| `cline-pass/mimo-v2.5-pro` | MiMo V2.5 Pro | 1M | ✅ | ✅ | ✅ | ✅ |
| `cline-pass/minimax-m3` | MiniMax M3 | 1M | ✅ | ✅ | ❌ | ✅ |
| `cline-pass/qwen3.7-max` | Qwen3.7 Max | 1M | ❌ | ❌ | ❌ | ✅ |
| `cline-pass/qwen3.7-plus` | Qwen3.7 Plus | 1M | ❌ | ❌ | ❌ | ✅ |

## 快速开始

### 获取 ClinePass API Key

1. 访问 [app.cline.bot](https://app.cline.bot/) 登录你的 Cline 账号
2. 进入 **Settings → API Keys** 创建 API Key
3. 订阅 ClinePass（$9.99/月）

### Windows

```powershell
# 下载并运行（或自行编译）
clinepass-proxy.exe -api-key "sk-你的-key"

# 默认监听 127.0.0.1:55991
# 加 -host 0.0.0.0 可局域网访问
```

### Linux（推荐 systemd）

```bash
# 下载 Linux 二进制
wget https://github.com/hayou2002/clinepass-proxy/releases/latest/download/clinepass-proxy-linux
chmod +x clinepass-proxy-linux

# 运行
./clinepass-proxy -api-key "sk-你的-key" -host 0.0.0.0

# 或使用 systemd 自启
sudo ./deploy/deploy.sh "sk-你的-key"
```

### Docker

```bash
docker run -d --name clinepass-proxy -p 55991:55991 \
  ghcr.io/hayou2002/clinepass-proxy:latest \
  -api-key "sk-你的-key" -host 0.0.0.0
```

## 客户端配置

以 CherryStudio 为例：

```
API 地址: http://服务器IP:55991/v1
API Key:  任意值（服务端已有 Key）
模型选择: cline-pass/deepseek-v4-pro
```

> 模型名必须带 `cline-pass/` 前缀，这是 ClinePass API 的标准格式。

## 命令行参数

| 参数 | 默认值 | 说明 |
|:--|:--|:--|
| `-api-key` | 空 | ClinePass API Key（服务端优先，客户端可随意填） |
| `-host` | `127.0.0.1` | 监听地址，`0.0.0.0` 开放所有网卡 |
| `-port` | `55991` | 监听端口 |
| `-thinking-lang` | `zh` | 思考语言：`zh`（中文）或 `en`（英文） |
| `-debug` | `false` | 启用调试日志 |
| `-version` | `false` | 显示版本号 |

## 工作原理

```
┌──────────────┐     OpenAI 格式     ┌─────────────────┐     ClinePass API     ┌────────────┐
│ CherryStudio │ ──── POST /v1 ────▶ │ ClinePass Proxy │ ──── chat/completions ──▶ │ ClinePass  │
│  或其他客户端  │                    │  字段转换+参数注入  │                      │  API       │
│              │ ◀──── SSE/JSON ──── │                 │ ◀─── reasoning ──────── │            │
└──────────────┘     reasoning_content     └─────────────────┘     +更多字段        └────────────┘
```

### 字段转换

代理自动处理上游不同 provider 的推理字段：

| 上游字段 | 来源 | 转换后 |
|:--|:--|:--|
| `reasoning` | DeepSeek、GLM、Kimi、MiMo | `reasoning_content` |
| `reasoning_details` | MiniMax M3 | `reasoning_content` |
| `reasoning_content` | 已为标准格式 | 透传 |

### 参数注入

代理根据模型自动注入所需参数，无需客户端支持：

| 模型 | 注入参数 |
|:--|:--|
| MiniMax M3 | `reasoning_split: true` + `reasoning_effort: "high"` |
| DeepSeek V4 | `thinking: {type: "enabled"}` + `reasoning_effort: "high"` |
| 其他推理模型 | `reasoning_effort: "high"` |

### 思考语言控制

代理自动在请求中插入 system 指令，强制模型使用指定语言思考。不受用户输入语言、读取的文件内容、网页、代码等影响。

## 管理命令

### Linux（systemd）

```bash
# 状态
systemctl status clinepass-proxy

# 日志
journalctl -u clinepass-proxy -f

# 重启
systemctl restart clinepass-proxy

# 停止
systemctl stop clinepass-proxy
```

### Windows（PowerShell）

```powershell
# 使用管理脚本
.\clinepass-proxy.psl config   # 配置
.\clinepass-proxy.psl start    # 启动
.\clinepass-proxy.psl status   # 状态
.\clinepass-proxy.psl logs     # 日志
```

## 编译

需要 Go 1.21+：

```bash
# Windows
go build -o bin/clinepass-proxy.exe .

# Linux
GOOS=linux GOARCH=amd64 go build -o bin/clinepass-proxy .
```

## 注意事项

- ⚠️ **模型名必须带 `cline-pass/` 前缀**，如 `cline-pass/glm-5.2`
- ⚠️ ClinePass 订阅有配额限制（5 小时滚动窗口 + 周配额 + 月配额）
- ⚠️ MiniMax M3 的推理通过 `reasoning_details` 字段返回，代理已自动转换
- ⚠️ 服务端配置 API Key 后，客户端 Authorization 头可填任意值
- ⚠️ 部署在公网时建议用防火墙限制访问或配合 cftunnel/nginx 使用
- ⚠️ API Key 不要硬编码在脚本中，推荐使用配置文件或环境变量

## 项目结构

```
clinepass-proxy/
├── main.go              # 入口 + HTTP 路由
├── internal/
│   ├── types.go         # 类型定义 + 模型目录
│   └── proxy.go         # 核心代理逻辑
├── deploy/
│   ├── deploy.sh        # Linux 部署脚本
│   ├── clinepass-proxy.service  # systemd 服务文件
│   ├── Dockerfile       # Docker 构建
│   └── docker-compose.yml
├── clinepass-proxy.ps1  # Windows 管理脚本
├── SKILL.md             # HanaAgent 管理技能
└── README.md
```

## License

MIT
