#!/bin/sh
set -e

# 如果没有用户配置文件，使用 full 默认配置
if [ ! -f /app/configs/config.yaml ]; then
  cp /app/configs/config.full.yaml /app/configs/config.yaml
fi

# 首次启动时下载 Camoufox 浏览器（缓存到 /app/.camoufox/）
if ! python -c "from camoufox.pkgman import installed_verstr; assert installed_verstr()" 2>/dev/null; then
  echo "[*] 首次启动，下载 Camoufox 浏览器..."
  python -m camoufox fetch || echo "[!] Camoufox 下载失败，Turnstile solver 可能无法工作"
fi

# 启动 Turnstile Solver（后台，端口 8888）
python3 /app/scripts/turnstile_solver.py --host 127.0.0.1 --port 8888 &
SOLVER_PID=$!

# 启动 Fireworks Python 服务（后台，端口 5000）
python3 /app/scripts/fireworks_reg.py --host 0.0.0.0 --port 5000 &
FIREWORKS_PID=$!

# 等待服务就绪
sleep 2

# 启动 Go HTTP 服务（前台）
exec /app/reg-server --config /app/configs/config.yaml
