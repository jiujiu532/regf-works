#!/usr/bin/env python3
"""
Turnstile Solver HTTP 服务

使用 Camoufox (反指纹 Firefox) + Playwright 真实渲染 Cloudflare Turnstile 页面，
提取 cf-turnstile-response token。

接口:
  GET /turnstile?url=<siteURL>&sitekey=<siteKey>&proxy=<proxy>
      → {"taskId": "abc123"}

  GET /result?id=<taskId>
      → {"status": "ready", "token": "0.xxx..."} 或 {"status": "pending"} 或 {"errorId": 1, ...}

  GET /health
      → {"status": "ok", "active": 0, "max_concurrent": 1}
"""

import argparse
import asyncio
import logging
import os
import secrets
import time
from typing import Optional

from quart import Quart, jsonify, request

app = Quart(__name__)
logger = logging.getLogger("turnstile-solver")

# 并发控制
MAX_CONCURRENT = int(os.getenv("SOLVER_MAX_CONCURRENT", "1"))
_semaphore: asyncio.Semaphore = asyncio.Semaphore(MAX_CONCURRENT)
_active_count = 0

# 任务存储（内存，重启清空）
_tasks: dict[str, dict] = {}

# 任务超时（秒）
TASK_TIMEOUT = int(os.getenv("SOLVER_TIMEOUT", "60"))


def _parse_proxy_for_playwright(proxy_str: str) -> Optional[dict]:
    """将代理字符串转为 Playwright proxy 配置"""
    if not proxy_str:
        return None
    proxy_str = proxy_str.strip()
    if not proxy_str.startswith(("http://", "https://", "socks5://")):
        proxy_str = "http://" + proxy_str

    from urllib.parse import urlparse
    parsed = urlparse(proxy_str)
    result = {"server": f"{parsed.scheme}://{parsed.hostname}:{parsed.port or 80}"}
    if parsed.username:
        result["username"] = parsed.username
    if parsed.password:
        result["password"] = parsed.password
    return result


async def _solve_turnstile(site_url: str, site_key: str, proxy: str = "") -> str:
    """
    核心求解逻辑：
    1. 启动 Camoufox 浏览器
    2. 构造一个包含 Turnstile widget 的页面
    3. 等待 Turnstile 自动完成挑战
    4. 提取 cf-turnstile-response token
    """
    from camoufox.async_api import AsyncCamoufox

    # headless 模式
    headless = os.getenv("SOLVER_HEADLESS", "true").lower() in ("true", "1", "yes")

    cf = AsyncCamoufox(headless=headless)
    browser = None
    context = None
    page = None

    try:
        browser = await cf.start()

        # 浏览器上下文选项
        ctx_opts = {}
        if proxy:
            pw_proxy = _parse_proxy_for_playwright(proxy)
            if pw_proxy:
                ctx_opts["proxy"] = pw_proxy

        context = await browser.new_context(**ctx_opts)
        page = await context.new_page()

        # 构造包含 Turnstile 的最小 HTML 页面
        turnstile_html = f"""<!DOCTYPE html>
<html>
<head>
    <script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>
</head>
<body>
    <div class="cf-turnstile" data-sitekey="{site_key}" data-callback="onToken"></div>
    <input type="hidden" id="cf-turnstile-response" />
    <script>
        function onToken(token) {{
            document.getElementById('cf-turnstile-response').value = token;
        }}
    </script>
</body>
</html>"""

        # 设置页面内容（使用目标站点的 origin 来通过 sitekey 绑定检查）
        await page.goto(site_url, wait_until="domcontentloaded", timeout=30000)
        await page.set_content(turnstile_html, wait_until="domcontentloaded")

        # 等待 Turnstile 完成（最多 TASK_TIMEOUT 秒）
        token = ""
        for _ in range(TASK_TIMEOUT * 2):  # 每 0.5s 检查一次
            try:
                # 尝试从 hidden input 获取 token
                val = await page.evaluate(
                    "document.getElementById('cf-turnstile-response')?.value || ''"
                )
                if val and len(val) > 20:
                    token = val
                    break

                # 备用：直接从 turnstile iframe 的 response 获取
                frames = page.frames
                for frame in frames:
                    try:
                        val = await frame.evaluate(
                            "document.querySelector('[name=\"cf-turnstile-response\"]')?.value || "
                            "document.querySelector('input[name=\"cf-turnstile-response\"]')?.value || ''"
                        )
                        if val and len(val) > 20:
                            token = val
                            break
                    except Exception:
                        continue
                if token:
                    break

            except Exception:
                pass

            await asyncio.sleep(0.5)

        if not token:
            raise RuntimeError(f"Turnstile 超时（{TASK_TIMEOUT}s 内未解出）")

        return token

    finally:
        if page:
            try:
                await page.close()
            except Exception:
                pass
        if context:
            try:
                await context.close()
            except Exception:
                pass
        if browser:
            try:
                await browser.close()
            except Exception:
                pass
        # 关闭 Camoufox 进程
        try:
            await cf.stop()
        except Exception:
            pass


async def _run_task(task_id: str, site_url: str, site_key: str, proxy: str):
    """后台执行求解任务"""
    global _active_count
    async with _semaphore:
        _active_count += 1
        _tasks[task_id]["status"] = "processing"
        try:
            token = await _solve_turnstile(site_url, site_key, proxy)
            _tasks[task_id] = {
                "status": "ready",
                "token": token,
                "completed_at": time.time(),
            }
            logger.info("Task %s 完成: token=%s...", task_id, token[:20])
        except Exception as exc:
            _tasks[task_id] = {
                "status": "failed",
                "errorId": 1,
                "errorCode": "SOLVE_FAILED",
                "errorDescription": str(exc)[:200],
                "completed_at": time.time(),
            }
            logger.error("Task %s 失败: %s", task_id, exc)
        finally:
            _active_count -= 1


# ─── HTTP 接口 ───


@app.route("/turnstile", methods=["GET"])
async def create_task():
    """提交求解任务"""
    site_url = request.args.get("url", "")
    site_key = request.args.get("sitekey", "")
    proxy = request.args.get("proxy", "")

    if not site_url or not site_key:
        return jsonify({"errorId": 1, "errorCode": "INVALID_PARAMS",
                        "errorDescription": "缺少 url 或 sitekey 参数"}), 400

    task_id = secrets.token_hex(8)
    _tasks[task_id] = {"status": "pending", "created_at": time.time()}

    # 后台启动任务
    asyncio.create_task(_run_task(task_id, site_url, site_key, proxy))

    logger.info("任务已创建: %s (url=%s)", task_id, site_url[:60])
    return jsonify({"taskId": task_id})


@app.route("/result", methods=["GET"])
async def get_result():
    """查询任务结果"""
    task_id = request.args.get("id", "")
    if not task_id or task_id not in _tasks:
        return jsonify({"errorId": 1, "errorCode": "TASK_NOT_FOUND",
                        "errorDescription": "Task not found"}), 404

    task = _tasks[task_id]

    if task["status"] == "ready":
        token = task.get("token", "")
        # 返回后清理（避免内存泄漏）
        del _tasks[task_id]
        return jsonify({"status": "ready", "solution": {"token": token}, "token": token})

    if task["status"] == "failed":
        result = {
            "errorId": task.get("errorId", 1),
            "errorCode": task.get("errorCode", "UNKNOWN"),
            "errorDescription": task.get("errorDescription", ""),
        }
        del _tasks[task_id]
        return jsonify(result)

    return jsonify({"status": "pending"})


@app.route("/health", methods=["GET"])
async def health():
    """健康检查"""
    return jsonify({
        "status": "ok",
        "active": _active_count,
        "max_concurrent": MAX_CONCURRENT,
        "pending_tasks": sum(1 for t in _tasks.values() if t.get("status") == "pending"),
    })


# ─── 定期清理过期任务 ───

async def _cleanup_loop():
    """每 60s 清理超过 5 分钟的已完成任务"""
    while True:
        await asyncio.sleep(60)
        now = time.time()
        expired = [
            tid for tid, t in _tasks.items()
            if t.get("completed_at") and now - t["completed_at"] > 300
        ]
        for tid in expired:
            del _tasks[tid]


@app.before_serving
async def startup():
    asyncio.create_task(_cleanup_loop())
    logger.info("Turnstile Solver 启动: max_concurrent=%d, timeout=%ds", MAX_CONCURRENT, TASK_TIMEOUT)


# ─── 入口 ───

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Turnstile Solver HTTP 服务")
    parser.add_argument("--host", default="0.0.0.0", help="监听地址")
    parser.add_argument("--port", type=int, default=8888, help="监听端口")
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(name)s] %(levelname)s: %(message)s",
    )

    app.run(host=args.host, port=args.port)
