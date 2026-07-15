# ClinePass Proxy

ClinePass 多Key轮询代理，让 ClinePass 订阅能在其他软件（CherryStudio、Kelivo等）中通过标准 OpenAI API 使用。

## 功能特性

- **多Key轮询**：支持添加多个 API Key，自动轮询调度
- **智能切换**：429限流自动切换到可用Key，冷却后自动恢复
- **管理面板**：Web界面动态增删Key、一键健康检测
- **模型信息**：展示可用模型的能力、上下文、最大输出等信息
- **思考过程**：保留 `reasoning` 和 `reasoning_details` 字段，支持 CherryStudio 显示思考过程
- **连接池优化**：全局连接池复用，减少延迟

## 部署

### 环境要求

- Python 3.10+
- aiohttp

### 安装

```bash
# 创建虚拟环境
python3 -m venv venv
./venv/bin/pip install aiohttp

# 配置
cp config.json.example config.json
# 编辑 config.json 设置端口等参数

# 启动
./venv/bin/python app.py
```

### Systemd 服务

```bash
cp clinepass-proxy.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable clinepass-proxy
systemctl start clinepass-proxy
```

## 使用

### 管理面板

访问 http://your-server:55991/

- 添加/删除 API Key
- 一键健康检测
- 查看可用模型信息

### API 端点

```
POST http://your-server:55991/v1/chat/completions
Authorization: Bearer any-key-or-empty
Content-Type: application/json

{
  "model": "deepseek-v4-flash",
  "messages": [{"role": "user", "content": "hello"}],
  "stream": false
}
```

模型名称会自动添加 `cline-pass/` 前缀。

### 支持的模型

| 模型 | 上下文 | 最大输出 | 速度 | 质量 |
|------|--------|----------|------|------|
| deepseek-v4-flash | 1000K | 16K | ⚡极快 | ⭐⭐⭐ |
| deepseek-v4-pro | 1000K | 16K | 🚀快速 | ⭐⭐⭐⭐⭐ |
| glm-5.2 | 200K | 8K | 🚀快速 | ⭐⭐⭐⭐ |
| kimi-k2.7-code | 262K | 8K | 🚀快速 | ⭐⭐⭐⭐⭐ |
| kimi-k2.6 | 262K | 8K | 🚀快速 | ⭐⭐⭐⭐ |
| qwen3.7-max | 262K | 8K | 🚀快速 | ⭐⭐⭐⭐⭐ |
| qwen3.7-plus | 1000K | 8K | ⚡极快 | ⭐⭐⭐⭐ |
| minimax-m3 | 1000K | 8K | 🚀快速 | ⭐⭐⭐⭐ |
| mimo-v2.5 | 262K | 8K | ⚡极快 | ⭐⭐⭐ |
| mimo-v2.5-pro | 262K | 8K | 🚀快速 | ⭐⭐⭐⭐ |

## 配置

config.json:

```json
{
  "host": "0.0.0.0",
  "port": 55991,
  "cline_api_base": "https://api.cline.bot",
  "cooldown_seconds": 300,
  "debug": false
}
```

## 文件结构

```
clinepass-proxy/
├── app.py                    # 主程序
├── config.json               # 配置文件
├── requirements.txt          # 依赖
├── clinepass-proxy.service   # Systemd服务文件
├── data/
│   └── keys.json            # Key存储（自动生成）
└── proxy.log                # 日志
```

## 注意事项

- ClinePass 响应会被解包为标准 OpenAI 格式（移除 `data` 包装层）
- 流式响应直接透传，保留 `reasoning` 字段
- Key 达到周上限会自动冷却 300 秒
