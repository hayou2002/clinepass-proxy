# ClinePass Proxy 管理技能

管理 ClinePass Proxy 服务的启停、配置和日志查看。

## 功能

- 启动/停止/重启代理服务
- 配置 ClinePass API Key
- 查看服务状态和日志
- 监控代理运行状态

## 使用方法

### 启动代理
```powershell
.\clinepass-proxy.ps1 start
```

### 停止代理
```powershell
.\clinepass-proxy.ps1 stop
```

### 查看状态
```powershell
.\clinepass-proxy.ps1 status
```

### 配置 API Key
```powershell
.\clinepass-proxy.ps1 config
```

### 查看日志
```powershell
.\clinepass-proxy.ps1 logs
```

## CherryStudio 配置

1. 打开 CherryStudio 设置
2. 选择 OpenAI 兼容 API
3. 填写配置：
   - API 地址: `http://127.0.0.1:55991/v1`
   - API Key: 任意值（服务端已配置时）
   - 模型: `glm-5.2` / `kimi-k2.7-code` / `deepseek-v4-pro` 等

## 可用模型

- DeepSeek: `deepseek-v4-pro`, `deepseek-v4-flash`
- Kimi: `kimi-k2.7-code`, `kimi-k2.7-code-highspeed`, `kimi-k2.6`, `kimi-k2.5`
- GLM: `glm-5.2`, `glm-5.2-fast`, `glm-5.1`, `glm-5`
- MiniMax: `minimax-m3`, `minimax-m2.7`, `minimax-m2.5`
- MiMo: `mimo-v2.5-pro`, `mimo-v2.5`
- Qwen: `qwen-3.7-max`, `qwen-3.7-plus`, `qwen-3.6-max-preview`, `qwen-3.6-plus`
- StepFun: `step-3.7-flash`, `step-3.5-flash`
- NVIDIA: `nemotron-3-ultra`

## 工作原理

代理将 ClinePass API 返回的 `reasoning` 字段转换为 OpenAI 标准的 `reasoning_content`，使 CherryStudio 能正确显示思考过程。

## 部署到 HK 精品服务器

1. 将 `clinepass-proxy.exe` 上传到服务器
2. 创建 systemd 服务或使用 Docker
3. 配置 API Key 和端口
4. 使用 frpc 或 cftunnel 暴露端口

## 故障排查

### 代理无法启动
- 检查端口是否被占用
- 检查 API Key 是否正确

### 思考过程不显示
- 确认模型支持思考模式（如 DeepSeek V4 Pro、GLM-5.2 等）
- 检查 CherryStudio 是否支持 `reasoning_content` 字段

### 请求超时
- 检查网络连接到 ClinePass API
- 增加超时时间（默认 300 秒）
