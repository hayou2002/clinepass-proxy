#!/usr/bin/env python3
"""
ClinePass Proxy - 多Key轮询代理 v2
支持：动态增删Key、轮询调度、健康检测、429自动切换、管理面板
优化：连接池复用、模型信息展示、白色主题
"""

import json
import os
import sys
import time
import asyncio
import logging
from pathlib import Path
from dataclasses import dataclass, field
from typing import Optional

import aiohttp
from aiohttp import web, ClientSession, TCPConnector

# ─── 配置 ───────────────────────────────────────────────────────────

BASE_DIR = Path(__file__).parent
CONFIG_PATH = BASE_DIR / "config.json"
KEYS_PATH = BASE_DIR / "data" / "keys.json"
LOG_PATH = BASE_DIR / "proxy.log"

DEFAULT_CONFIG = {
    "host": "0.0.0.0",
    "port": 55991,
    "cline_api_base": "https://api.cline.bot",
    "cooldown_seconds": 300,
    "debug": False,
}

# ─── 模型信息 ───────────────────────────────────────────────────────

MODELS_INFO = [
    {
        "id": "deepseek-v4-flash",
        "name": "DeepSeek V4 Flash",
        "description": "快速响应的轻量级模型，适合日常对话和简单任务",
        "context_window": 1000000,
        "max_output": 16384,
        "capabilities": ["对话", "代码", "推理"],
        "speed": "⚡ 极快",
        "quality": "⭐⭐⭐"
    },
    {
        "id": "deepseek-v4-pro",
        "name": "DeepSeek V4 Pro",
        "description": "高性能专业模型，适合复杂推理和代码生成",
        "context_window": 1000000,
        "max_output": 16384,
        "capabilities": ["对话", "代码", "推理", "分析"],
        "speed": "🚀 快速",
        "quality": "⭐⭐⭐⭐⭐"
    },
    {
        "id": "glm-5.2",
        "name": "GLM 5.2",
        "description": "智谱AI最新一代模型，中文能力出色",
        "context_window": 200000,
        "max_output": 8192,
        "capabilities": ["对话", "代码", "创作", "分析"],
        "speed": "🚀 快速",
        "quality": "⭐⭐⭐⭐"
    },
    {
        "id": "kimi-k2.7-code",
        "name": "Kimi K2.7 Code",
        "description": "月之暗面代码专用模型，代码能力极强",
        "context_window": 262000,
        "max_output": 8192,
        "capabilities": ["代码", "调试", "重构"],
        "speed": "🚀 快速",
        "quality": "⭐⭐⭐⭐⭐"
    },
    {
        "id": "kimi-k2.6",
        "name": "Kimi K2.6",
        "description": "月之暗面通用模型，长文本处理能力强",
        "context_window": 262000,
        "max_output": 8192,
        "capabilities": ["对话", "阅读", "分析", "创作"],
        "speed": "🚀 快速",
        "quality": "⭐⭐⭐⭐"
    },
    {
        "id": "qwen3.7-max",
        "name": "Qwen 3.7 Max",
        "description": "通义千问旗舰模型，综合能力最强",
        "context_window": 262000,
        "max_output": 8192,
        "capabilities": ["对话", "代码", "推理", "创作", "分析"],
        "speed": "🚀 快速",
        "quality": "⭐⭐⭐⭐⭐"
    },
    {
        "id": "qwen3.7-plus",
        "name": "Qwen 3.7 Plus",
        "description": "通义千问增强版，性价比高",
        "context_window": 1000000,
        "max_output": 8192,
        "capabilities": ["对话", "代码", "推理"],
        "speed": "⚡ 极快",
        "quality": "⭐⭐⭐⭐"
    },
    {
        "id": "minimax-m3",
        "name": "MiniMax M3",
        "description": "MiniMax最新模型，多模态支持",
        "context_window": 1000000,
        "max_output": 8192,
        "capabilities": ["对话", "代码", "多模态"],
        "speed": "🚀 快速",
        "quality": "⭐⭐⭐⭐"
    },
    {
        "id": "mimo-v2.5",
        "name": "MiMo V2.5",
        "description": "小米MiMo模型，轻量高效",
        "context_window": 262000,
        "max_output": 8192,
        "capabilities": ["对话", "代码"],
        "speed": "⚡ 极快",
        "quality": "⭐⭐⭐"
    },
    {
        "id": "mimo-v2.5-pro",
        "name": "MiMo V2.5 Pro",
        "description": "小米MiMo专业版，性能更强",
        "context_window": 262000,
        "max_output": 8192,
        "capabilities": ["对话", "代码", "推理"],
        "speed": "🚀 快速",
        "quality": "⭐⭐⭐⭐"
    }
]

# ─── 日志 ───────────────────────────────────────────────────────────

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    handlers=[
        logging.FileHandler(LOG_PATH, encoding="utf-8"),
        logging.StreamHandler()
    ]
)
log = logging.getLogger("clinepass-proxy")

# ─── Key 管理 ───────────────────────────────────────────────────────

@dataclass
class ApiKey:
    key: str
    name: str = ""
    enabled: bool = True
    cooldown_until: float = 0.0
    request_count: int = 0
    error_count: int = 0
    last_used: float = 0.0
    last_error: str = ""
    added_at: float = field(default_factory=time.time)

    @property
    def short_key(self) -> str:
        return self.key[:12] + "..." + self.key[-6:]

    @property
    def is_cooling(self) -> bool:
        return time.time() < self.cooldown_until

    @property
    def is_available(self) -> bool:
        return self.enabled and not self.is_cooling

    def to_dict(self) -> dict:
        return {
            "key": self.key,
            "name": self.name,
            "enabled": self.enabled,
            "cooldown_until": self.cooldown_until,
            "request_count": self.request_count,
            "error_count": self.error_count,
            "last_used": self.last_used,
            "last_error": self.last_error,
            "added_at": self.added_at,
            "is_available": self.is_available,
            "is_cooling": self.is_cooling,
        }

    @staticmethod
    def from_dict(d: dict) -> "ApiKey":
        return ApiKey(
            key=d["key"],
            name=d.get("name", ""),
            enabled=d.get("enabled", True),
            cooldown_until=d.get("cooldown_until", 0),
            request_count=d.get("request_count", 0),
            error_count=d.get("error_count", 0),
            last_used=d.get("last_used", 0),
            last_error=d.get("last_error", ""),
            added_at=d.get("added_at", time.time()),
        )


class KeyManager:
    def __init__(self, data_path: Path, cooldown_seconds: int = 300):
        self.data_path = data_path
        self.cooldown_seconds = cooldown_seconds
        self.keys: list[ApiKey] = []
        self._index = 0
        self._lock = asyncio.Lock()
        self._load()

    def _load(self):
        if self.data_path.exists():
            try:
                with open(self.data_path, "r", encoding="utf-8") as f:
                    data = json.load(f)
                self.keys = [ApiKey.from_dict(k) for k in data]
                log.info(f"加载了 {len(self.keys)} 个 API Key")
            except Exception as e:
                log.error(f"加载 keys 失败: {e}")
                self.keys = []
        else:
            self.keys = []
            self._save()

    def _save(self):
        self.data_path.parent.mkdir(parents=True, exist_ok=True)
        with open(self.data_path, "w", encoding="utf-8") as f:
            json.dump([k.to_dict() for k in self.keys], f, indent=2, ensure_ascii=False)

    def add_key(self, key: str, name: str = "") -> ApiKey:
        for k in self.keys:
            if k.key == key:
                raise ValueError("该 Key 已存在")
        api_key = ApiKey(key=key, name=name or f"Key-{len(self.keys)+1}")
        self.keys.append(api_key)
        self._save()
        log.info(f"添加 Key: {api_key.name} ({api_key.short_key})")
        return api_key

    def remove_key(self, key: str) -> bool:
        for i, k in enumerate(self.keys):
            if k.key == key:
                removed = self.keys.pop(i)
                self._save()
                log.info(f"删除 Key: {removed.name} ({removed.short_key})")
                return True
        return False

    def toggle_key(self, key: str) -> Optional[ApiKey]:
        for k in self.keys:
            if k.key == key:
                k.enabled = not k.enabled
                self._save()
                return k
        return None

    def mark_error(self, key: str, error: str):
        for k in self.keys:
            if k.key == key:
                k.error_count += 1
                k.last_error = error
                k.cooldown_until = time.time() + self.cooldown_seconds
                self._save()
                log.warning(f"Key {k.name} 进入冷却 {self.cooldown_seconds}s: {error}")
                return

    def mark_success(self, key: str):
        for k in self.keys:
            if k.key == key:
                k.request_count += 1
                k.last_used = time.time()
                if k.is_cooling:
                    k.cooldown_until = 0
                self._save()
                return

    async def get_next_key(self) -> Optional[ApiKey]:
        async with self._lock:
            if not self.keys:
                return None
            available = [k for k in self.keys if k.is_available]
            if not available:
                cooling = [k for k in self.keys if k.enabled]
                if cooling:
                    cooling.sort(key=lambda k: k.cooldown_until)
                    earliest = cooling[0]
                    wait = earliest.cooldown_until - time.time()
                    if wait > 0:
                        log.warning(f"所有 Key 冷却中，等待 {wait:.0f}s 使用 {earliest.name}")
                        await asyncio.sleep(wait)
                    earliest.cooldown_until = 0
                    return earliest
                return None
            key = available[self._index % len(available)]
            self._index = (self._index + 1) % len(available)
            return key

    def get_all_keys(self) -> list[dict]:
        return [k.to_dict() for k in self.keys]

    def get_stats(self) -> dict:
        available = [k for k in self.keys if k.is_available]
        cooling = [k for k in self.keys if k.is_cooling]
        disabled = [k for k in self.keys if not k.enabled]
        return {
            "total": len(self.keys),
            "available": len(available),
            "cooling": len(cooling),
            "disabled": len(disabled),
            "total_requests": sum(k.request_count for k in self.keys),
            "total_errors": sum(k.error_count for k in self.keys),
        }


# ─── 全局连接池 ──────────────────────────────────────────────────────

_global_session: Optional[ClientSession] = None
_connector: Optional[TCPConnector] = None

async def get_session() -> ClientSession:
    global _global_session, _connector
    if _global_session is None or _global_session.closed:
        _connector = TCPConnector(
            limit=100,
            limit_per_host=30,
            ttl_dns_cache=300,
            enable_cleanup_closed=True,
            force_close=False,
        )
        _global_session = ClientSession(connector=_connector)
    return _global_session

async def cleanup_session():
    global _global_session
    if _global_session and not _global_session.closed:
        await _global_session.close()


# ─── 配置加载 ────────────────────────────────────────────────────────

def load_config() -> dict:
    config = dict(DEFAULT_CONFIG)
    if CONFIG_PATH.exists():
        with open(CONFIG_PATH, "r", encoding="utf-8") as f:
            user_config = json.load(f)
        config.update(user_config)
    return config


# ─── 代理转发 ────────────────────────────────────────────────────────

async def proxy_request(request: web.Request, key_manager: KeyManager, config: dict) -> web.Response:
    body = await request.read()
    is_stream = False
    try:
        body_json = json.loads(body)
        is_stream = body_json.get("stream", False)
        model = body_json.get("model", "")
        if model and not model.startswith("cline-pass/"):
            body_json["model"] = f"cline-pass/{model}"
            body = json.dumps(body_json).encode("utf-8")
    except:
        pass

    api_key = await key_manager.get_next_key()
    if not api_key:
        return web.json_response(
            {"error": {"message": "没有可用的 API Key", "type": "proxy_error"}},
            status=503
        )

    target_url = f"{config['cline_api_base']}/api/v1/chat/completions"
    headers = {
        "Authorization": f"Bearer {api_key.key}",
        "Content-Type": "application/json",
    }
    for h in ["Accept", "User-Agent"]:
        if h in request.headers:
            headers[h] = request.headers[h]

    if config.get("debug"):
        log.info(f"[{api_key.name}] -> {target_url} stream={is_stream}")

    try:
        session = await get_session()
        async with session.post(
            target_url,
            headers=headers,
            data=body,
            timeout=aiohttp.ClientTimeout(total=300)
        ) as resp:
            if resp.status == 429:
                error_text = await resp.text()
                key_manager.mark_error(api_key.key, f"429 Rate Limited: {error_text[:200]}")
                new_key = await key_manager.get_next_key()
                if new_key and new_key.key != api_key.key:
                    headers["Authorization"] = f"Bearer {new_key.key}"
                    async with session.post(
                        target_url,
                        headers=headers,
                        data=body,
                        timeout=aiohttp.ClientTimeout(total=300)
                    ) as retry_resp:
                        if retry_resp.status == 200:
                            key_manager.mark_success(new_key.key)
                        if is_stream:
                            return await _stream_response(request, retry_resp, new_key, key_manager)
                        else:
                            resp_body = await retry_resp.read()
                            return _unwrap_response(resp_body)
                return web.json_response(
                    {"error": {"message": "所有 Key 均被限流", "type": "rate_limit_error"}},
                    status=429
                )

            if resp.status != 200:
                error_text = await resp.text()
                if resp.status in (401, 403):
                    key_manager.mark_error(api_key.key, f"{resp.status} Auth Error: {error_text[:200]}")
                return web.Response(
                    body=error_text.encode("utf-8"),
                    status=resp.status,
                    content_type=resp.content_type
                )

            key_manager.mark_success(api_key.key)
            if is_stream:
                return await _stream_response(request, resp, api_key, key_manager)
            else:
                resp_body = await resp.read()
                return _unwrap_response(resp_body)

    except asyncio.TimeoutError:
        key_manager.mark_error(api_key.key, "请求超时")
        return web.json_response(
            {"error": {"message": "请求超时", "type": "timeout_error"}},
            status=504
        )
    except Exception as e:
        key_manager.mark_error(api_key.key, str(e))
        log.error(f"代理请求失败: {e}")
        return web.json_response(
            {"error": {"message": f"代理错误: {str(e)}", "type": "proxy_error"}},
            status=502
        )


def _unwrap_response(resp_body: bytes) -> web.Response:
    """解包 ClinePass 的 data 包装层"""
    try:
        resp_json = json.loads(resp_body)
        if "data" in resp_json and "success" in resp_json:
            openai_resp = resp_json["data"]
            return web.json_response(openai_resp)
    except:
        pass
    return web.Response(body=resp_body, content_type="application/json")


async def _stream_response(request: web.Request, resp, api_key: ApiKey, key_manager: KeyManager):
    response = web.StreamResponse(
        status=resp.status,
        headers={
            "Content-Type": "text/event-stream",
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
        }
    )
    await response.prepare(request)
    try:
        async for chunk in resp.content.iter_any():
            await response.write(chunk)
    except Exception as e:
        log.error(f"流式传输中断: {e}")
    finally:
        await response.write_eof()
    return response


# ─── 健康检测 ────────────────────────────────────────────────────────

async def _test_single_key(key: str, config: dict) -> dict:
    target_url = f"{config['cline_api_base']}/api/v1/chat/completions"
    headers = {
        "Authorization": f"Bearer {key}",
        "Content-Type": "application/json",
    }
    test_body = json.dumps({
        "model": "cline-pass/deepseek-v4-flash",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1,
        "stream": False,
    })

    start = time.time()
    try:
        session = await get_session()
        async with session.post(
            target_url,
            headers=headers,
            data=test_body,
            timeout=aiohttp.ClientTimeout(total=15)
        ) as resp:
            latency = round((time.time() - start) * 1000)
            if resp.status == 200:
                return {"status": "ok", "latency_ms": latency, "code": 200}
            elif resp.status == 429:
                error_text = await resp.text()
                if "weekly" in error_text.lower() or "limit" in error_text.lower():
                    return {"status": "rate_limited", "latency_ms": latency, "code": 429, "detail": "已达周上限"}
                return {"status": "rate_limited", "latency_ms": latency, "code": 429}
            elif resp.status in (401, 403):
                return {"status": "auth_error", "latency_ms": latency, "code": resp.status}
            else:
                error_text = await resp.text()
                return {"status": "error", "latency_ms": latency, "code": resp.status, "detail": error_text[:200]}
    except asyncio.TimeoutError:
        return {"status": "timeout", "latency_ms": 15000, "code": 0}
    except Exception as e:
        return {"status": "error", "latency_ms": 0, "code": 0, "detail": str(e)}


# ─── 管理面板 API ───────────────────────────────────────────────────

async def handle_panel(request: web.Request) -> web.Response:
    return web.Response(text=PANEL_HTML, content_type="text/html")


async def handle_api_keys(request: web.Request, key_manager: KeyManager) -> web.Response:
    if request.method == "GET":
        return web.json_response({
            "keys": key_manager.get_all_keys(),
            "stats": key_manager.get_stats(),
        })
    data = await request.json()
    key = data.get("key", "").strip()
    name = data.get("name", "").strip()
    if not key:
        return web.json_response({"error": "Key 不能为空"}, status=400)
    try:
        api_key = key_manager.add_key(key, name)
        return web.json_response({"ok": True, "key": api_key.to_dict()})
    except ValueError as e:
        return web.json_response({"error": str(e)}, status=400)


async def handle_api_key_action(request: web.Request, key_manager: KeyManager) -> web.Response:
    key = request.match_info["key"]
    if request.method == "DELETE":
        if key_manager.remove_key(key):
            return web.json_response({"ok": True})
        return web.json_response({"error": "Key 不存在"}, status=404)
    api_key = key_manager.toggle_key(key)
    if api_key:
        return web.json_response({"ok": True, "key": api_key.to_dict()})
    return web.json_response({"error": "Key 不存在"}, status=404)


async def handle_health_check(request: web.Request, key_manager: KeyManager, config: dict) -> web.Response:
    results = []
    for api_key in key_manager.keys:
        if not api_key.enabled:
            results.append({
                "name": api_key.name,
                "key": api_key.short_key,
                "full_key": api_key.key,
                "status": "disabled",
                "latency_ms": 0,
                "code": 0
            })
            continue
        result = await _test_single_key(api_key.key, config)
        results.append({
            "name": api_key.name,
            "key": api_key.short_key,
            "full_key": api_key.key,
            **result
        })
    return web.json_response({"results": results})


async def handle_test_key(request: web.Request, key_manager: KeyManager, config: dict) -> web.Response:
    data = await request.json()
    key = data.get("key", "").strip()
    if not key:
        return web.json_response({"error": "Key 不能为空"}, status=400)
    result = await _test_single_key(key, config)
    return web.json_response(result)


async def handle_models(request: web.Request) -> web.Response:
    """返回可用模型列表"""
    return web.json_response({
        "object": "list",
        "data": [{"id": m["id"], "object": "model", "owned_by": "cline-pass"} for m in MODELS_INFO]
    })


async def handle_models_info(request: web.Request) -> web.Response:
    """返回模型详细信息"""
    return web.json_response({"models": MODELS_INFO})


# ─── 管理面板 HTML ──────────────────────────────────────────────────

PANEL_HTML = r"""<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>ClinePass Proxy 管理面板</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #f5f5f7; color: #1d1d1f; min-height: 100vh; }
.container { max-width: 960px; margin: 0 auto; padding: 24px; }
h1 { font-size: 24px; margin-bottom: 24px; display: flex; align-items: center; gap: 10px; color: #1d1d1f; }
h1 span { color: #0071e3; }

/* 统计卡片 */
.stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 28px; }
.stat-card { background: #fff; border: 1px solid #e5e5e5; border-radius: 12px; padding: 20px; text-align: center; box-shadow: 0 1px 3px rgba(0,0,0,0.05); }
.stat-value { font-size: 32px; font-weight: 700; color: #0071e3; }
.stat-label { font-size: 13px; color: #86868b; margin-top: 6px; }
.stat-card.ok .stat-value { color: #34c759; }
.stat-card.warn .stat-value { color: #ff9500; }
.stat-card.err .stat-value { color: #ff3b30; }

/* 标签页 */
.tabs { display: flex; gap: 4px; margin-bottom: 20px; background: #fff; border-radius: 10px; padding: 4px; border: 1px solid #e5e5e5; }
.tab { padding: 10px 20px; border-radius: 8px; border: none; background: transparent; color: #86868b; cursor: pointer; font-size: 14px; font-weight: 500; transition: all .2s; }
.tab.active { background: #0071e3; color: #fff; }
.tab:hover:not(.active) { background: #f0f0f0; }

/* 操作栏 */
.actions { display: flex; gap: 10px; margin-bottom: 20px; flex-wrap: wrap; }
input[type="text"], input[type="password"] { background: #fff; border: 1px solid #d2d2d7; border-radius: 8px; padding: 10px 14px; color: #1d1d1f; font-size: 14px; flex: 1; min-width: 200px; }
input[type="text"]::placeholder, input[type="password"]::placeholder { color: #aeaeb2; }
button { padding: 10px 18px; border-radius: 8px; border: 1px solid #d2d2d7; background: #fff; color: #1d1d1f; cursor: pointer; font-size: 13px; font-weight: 500; white-space: nowrap; transition: all .15s; }
button:hover { background: #f5f5f7; }
button.primary { background: #0071e3; border-color: #0071e3; color: #fff; }
button.primary:hover { background: #0077ed; }
button.danger { background: #ff3b30; border-color: #ff3b30; color: #fff; }
button.danger:hover { background: #ff453a; }

/* Key 列表 */
.key-list { display: flex; flex-direction: column; gap: 10px; }
.key-item { background: #fff; border: 1px solid #e5e5e5; border-radius: 12px; padding: 16px 18px; display: flex; align-items: center; gap: 14px; transition: all .2s; box-shadow: 0 1px 3px rgba(0,0,0,0.05); }
.key-item:hover { border-color: #0071e3; box-shadow: 0 2px 8px rgba(0,113,227,0.1); }
.key-item.cooling { border-color: #ff9500; }
.key-item.disabled { opacity: 0.5; background: #f9f9f9; }
.key-status { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.key-status.ok { background: #34c759; }
.key-status.cooling { background: #ff9500; animation: pulse 1.5s infinite; }
.key-status.disabled { background: #d2d2d7; }
@keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }
.key-info { flex: 1; min-width: 0; }
.key-name { font-size: 15px; font-weight: 600; color: #1d1d1f; }
.key-hash { font-size: 12px; color: #86868b; font-family: 'SF Mono', Monaco, monospace; margin-top: 3px; }
.key-stats { display: flex; gap: 18px; margin-top: 6px; font-size: 12px; color: #86868b; }
.key-stats span { display: flex; align-items: center; gap: 5px; }
.key-actions { display: flex; gap: 8px; }
.key-actions button { padding: 6px 12px; font-size: 12px; }

/* Toast */
.toast { position: fixed; top: 20px; right: 20px; padding: 14px 22px; border-radius: 10px; font-size: 14px; font-weight: 500; z-index: 999; animation: slideIn .3s; box-shadow: 0 4px 12px rgba(0,0,0,0.15); }
.toast.ok { background: #34c759; color: #fff; }
.toast.err { background: #ff3b30; color: #fff; }
@keyframes slideIn { from { transform: translateX(100px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }

/* 检测结果 */
.health-results { margin-top: 20px; }
.health-item { display: flex; align-items: center; gap: 12px; padding: 12px 0; border-bottom: 1px solid #f0f0f0; font-size: 14px; }
.health-dot { width: 8px; height: 8px; border-radius: 50%; }
.health-dot.ok { background: #34c759; }
.health-dot.rate_limited { background: #ff9500; }
.health-dot.auth_error { background: #ff3b30; }
.health-dot.error { background: #ff3b30; }
.health-dot.timeout { background: #ff3b30; }
.health-dot.disabled { background: #d2d2d7; }

/* 模型列表 */
.model-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 16px; }
.model-card { background: #fff; border: 1px solid #e5e5e5; border-radius: 12px; padding: 18px; transition: all .2s; box-shadow: 0 1px 3px rgba(0,0,0,0.05); }
.model-card:hover { border-color: #0071e3; box-shadow: 0 2px 8px rgba(0,113,227,0.1); transform: translateY(-2px); }
.model-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.model-name { font-size: 16px; font-weight: 600; color: #1d1d1f; }
.model-speed { font-size: 13px; }
.model-desc { font-size: 13px; color: #86868b; margin-bottom: 12px; line-height: 1.4; }
.model-meta { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
.model-tag { background: #f0f0f0; padding: 4px 10px; border-radius: 6px; font-size: 12px; color: #1d1d1f; }
.model-stats { display: flex; justify-content: space-between; font-size: 12px; color: #86868b; padding-top: 12px; border-top: 1px solid #f0f0f0; }
.model-stats span { display: flex; flex-direction: column; gap: 2px; }
.model-stats strong { color: #1d1d1f; font-weight: 600; }
</style>
</head>
<body>
<div class="container">
  <h1>🔑 <span>ClinePass Proxy</span> 管理面板</h1>

  <div class="stats" id="stats">
    <div class="stat-card"><div class="stat-value" id="s-total">-</div><div class="stat-label">总 Key 数</div></div>
    <div class="stat-card ok"><div class="stat-value" id="s-available">-</div><div class="stat-label">可用</div></div>
    <div class="stat-card warn"><div class="stat-value" id="s-cooling">-</div><div class="stat-label">冷却中</div></div>
    <div class="stat-card err"><div class="stat-value" id="s-errors">-</div><div class="stat-label">总错误</div></div>
  </div>

  <div class="tabs">
    <button class="tab active" onclick="switchTab('keys')">🔑 Key 管理</button>
    <button class="tab" onclick="switchTab('models')">📦 可用模型</button>
  </div>

  <div id="tab-keys">
    <div class="actions">
      <input type="text" id="newKey" placeholder="输入新的 API Key (sk_...)">
      <input type="text" id="newName" placeholder="备注名（可选）" style="max-width:160px">
      <button class="primary" onclick="addKey()">添加 Key</button>
      <button onclick="healthCheck()">🩺 一键检测</button>
    </div>
    <div class="key-list" id="keyList"></div>
    <div class="health-results" id="healthResults"></div>
  </div>

  <div id="tab-models" style="display:none">
    <div class="model-grid" id="modelGrid"></div>
  </div>
</div>

<script>
const API = '';

function switchTab(tab) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  event.target.classList.add('active');
  document.getElementById('tab-keys').style.display = tab === 'keys' ? 'block' : 'none';
  document.getElementById('tab-models').style.display = tab === 'models' ? 'block' : 'none';
  if (tab === 'models') loadModels();
}

async function load() {
  try {
    const resp = await fetch(`${API}/api/keys`);
    const data = await resp.json();
    renderStats(data.stats);
    renderKeys(data.keys);
  } catch(e) { toast('加载失败: ' + e.message, true); }
}

function renderStats(s) {
  document.getElementById('s-total').textContent = s.total;
  document.getElementById('s-available').textContent = s.available;
  document.getElementById('s-cooling').textContent = s.cooling;
  document.getElementById('s-errors').textContent = s.total_errors;
}

function renderKeys(keys) {
  const list = document.getElementById('keyList');
  if (!keys.length) { list.innerHTML = '<div style="color:#86868b;text-align:center;padding:40px;background:#fff;border-radius:12px">暂无 Key，请在上方添加</div>'; return; }
  list.innerHTML = keys.map(k => {
    let cls = 'key-item';
    if (!k.enabled) cls += ' disabled';
    else if (k.is_cooling) cls += ' cooling';
    let statusCls = !k.enabled ? 'disabled' : k.is_cooling ? 'cooling' : 'ok';
    const cooldownLeft = k.is_cooling ? Math.max(0, Math.round(k.cooldown_until - Date.now()/1000)) : 0;
    return `<div class="${cls}">
      <div class="key-status ${statusCls}"></div>
      <div class="key-info">
        <div class="key-name">${k.name || '未命名'}</div>
        <div class="key-hash">${k.key.substring(0,12)}...${k.key.substring(k.key.length-6)}</div>
        <div class="key-stats">
          <span>📊 ${k.request_count} 次</span>
          <span>❌ ${k.error_count} 次</span>
          ${k.is_cooling ? `<span>⏳ 冷却 ${cooldownLeft}s</span>` : ''}
          ${k.last_error ? `<span title="${k.last_error}">⚠️ ${k.last_error.substring(0,30)}</span>` : ''}
        </div>
      </div>
      <div class="key-actions">
        <button onclick="toggleKey('${k.key}')">${k.enabled ? '禁用' : '启用'}</button>
        <button class="danger" onclick="removeKey('${k.key}','${k.name}')">删除</button>
      </div>
    </div>`;
  }).join('');
}

async function addKey() {
  const key = document.getElementById('newKey').value.trim();
  const name = document.getElementById('newName').value.trim();
  if (!key) return toast('请输入 Key', true);
  try {
    const resp = await fetch(`${API}/api/keys`, {
      method: 'POST', headers: {'Content-Type':'application/json'},
      body: JSON.stringify({key, name})
    });
    const data = await resp.json();
    if (data.ok) { toast('添加成功'); document.getElementById('newKey').value = ''; document.getElementById('newName').value = ''; load(); }
    else toast(data.error, true);
  } catch(e) { toast('添加失败: ' + e.message, true); }
}

async function removeKey(key, name) {
  if (!confirm(`确认删除 ${name}？`)) return;
  try {
    const resp = await fetch(`${API}/api/keys/${encodeURIComponent(key)}`, {method:'DELETE'});
    const data = await resp.json();
    if (data.ok) { toast('已删除'); load(); }
    else toast(data.error, true);
  } catch(e) { toast('删除失败', true); }
}

async function toggleKey(key) {
  try {
    const resp = await fetch(`${API}/api/keys/${encodeURIComponent(key)}`, {method:'PUT'});
    const data = await resp.json();
    if (data.ok) { toast('已切换'); load(); }
    else toast(data.error, true);
  } catch(e) { toast('操作失败', true); }
}

async function healthCheck() {
  const div = document.getElementById('healthResults');
  div.innerHTML = '<div style="color:#86868b;padding:16px;background:#fff;border-radius:12px">检测中...</div>';
  try {
    const resp = await fetch(`${API}/api/health-check`);
    const data = await resp.json();
    div.innerHTML = '<h3 style="margin-bottom:12px;font-size:15px;color:#1d1d1f">🩺 检测结果</h3>' +
      data.results.map(r => {
        const statusMap = {ok:'✅ 可用',rate_limited:'⚠️ 限流',auth_error:'❌ 认证失败',error:'❌ 异常',timeout:'⏱ 超时',disabled:'⏸ 已禁用'};
        const detail = r.detail ? ` (${r.detail})` : '';
        return `<div class="health-item">
          <div class="health-dot ${r.status}"></div>
          <span style="flex:1">${r.name} <span style="color:#86868b;font-size:12px">${r.key}</span></span>
          <span>${statusMap[r.status]||r.status}${detail}</span>
          <span style="color:#86868b;margin-left:10px">${r.latency_ms}ms</span>
        </div>`;
      }).join('');
    load();
  } catch(e) { div.innerHTML = '<div style="color:#ff3b30;padding:16px">检测失败: ' + e.message + '</div>'; }
}

async function loadModels() {
  try {
    const resp = await fetch(`${API}/api/models-info`);
    const data = await resp.json();
    const grid = document.getElementById('modelGrid');
    grid.innerHTML = data.models.map(m => `
      <div class="model-card">
        <div class="model-header">
          <div class="model-name">${m.name}</div>
          <div class="model-speed">${m.speed}</div>
        </div>
        <div class="model-desc">${m.description}</div>
        <div class="model-meta">
          ${m.capabilities.map(c => `<span class="model-tag">${c}</span>`).join('')}
        </div>
        <div class="model-stats">
          <span>上下文<strong>${(m.context_window/1000).toFixed(0)}K</strong></span>
          <span>最大输出<strong>${(m.max_output/1000).toFixed(1)}K</strong></span>
          <span>质量<strong>${m.quality}</strong></span>
        </div>
      </div>
    `).join('');
  } catch(e) { toast('加载模型失败', true); }
}

function toast(msg, isErr) {
  const el = document.createElement('div');
  el.className = 'toast ' + (isErr ? 'err' : 'ok');
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 3000);
}

load();
setInterval(load, 5000);
</script>
</body>
</html>"""


# ─── 路由与启动 ─────────────────────────────────────────────────────

def create_app(key_manager: KeyManager, config: dict) -> web.Application:
    app = web.Application(client_max_size=50 * 1024 * 1024)

    # 管理面板
    app.router.add_get("/", handle_panel)

    # API 管理接口
    app.router.add_get("/api/keys", lambda r: handle_api_keys(r, key_manager))
    app.router.add_post("/api/keys", lambda r: handle_api_keys(r, key_manager))
    app.router.add_delete("/api/keys/{key}", lambda r: handle_api_key_action(r, key_manager))
    app.router.add_put("/api/keys/{key}", lambda r: handle_api_key_action(r, key_manager))

    # 检测接口
    app.router.add_get("/api/health-check", lambda r: handle_health_check(r, key_manager, config))
    app.router.add_post("/api/test-key", lambda r: handle_test_key(r, key_manager, config))

    # 模型接口
    app.router.add_get("/v1/models", handle_models)
    app.router.add_get("/models", handle_models)
    app.router.add_get("/api/models-info", handle_models_info)

    # OpenAI 兼容代理
    app.router.add_post("/v1/chat/completions", lambda r: proxy_request(r, key_manager, config))
    app.router.add_post("/chat/completions", lambda r: proxy_request(r, key_manager, config))

    # 通配路径
    async def fallback_proxy(request: web.Request):
        return await proxy_request(request, key_manager, config)
    app.router.add_route("*", "/api/v1/{path:.*}", fallback_proxy)

    return app


async def on_shutdown(app):
    await cleanup_session()


def main():
    config = load_config()
    key_manager = KeyManager(KEYS_PATH, config["cooldown_seconds"])

    # 迁移旧 config.json 中的 api_key
    old_key = None
    if CONFIG_PATH.exists():
        with open(CONFIG_PATH, "r", encoding="utf-8") as f:
            old_config = json.load(f)
        old_key = old_config.get("api_key")
    if old_key and not any(k.key == old_key for k in key_manager.keys):
        key_manager.add_key(old_key, name="迁移的旧Key")
        log.info("已将旧 config.json 中的 api_key 迁移到 keys.json")

    app = create_app(key_manager, config)
    app.on_shutdown.append(on_shutdown)

    host = config.get("host", "0.0.0.0")
    port = config.get("port", 55991)
    log.info(f"ClinePass Proxy 启动于 {host}:{port}")
    log.info(f"管理面板: http://{host}:{port}/")
    log.info(f"代理端点: http://{host}:{port}/v1/chat/completions")
    log.info(f"已加载 {len(key_manager.keys)} 个 Key")

    web.run_app(app, host=host, port=port, print=None)


if __name__ == "__main__":
    main()
