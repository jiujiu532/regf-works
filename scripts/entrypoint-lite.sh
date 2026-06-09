#!/bin/sh
set -e

# 如果没有用户配置文件，从模板复制
if [ ! -f /app/configs/config.yaml ]; then
  cp /app/configs/config.example.yaml /app/configs/config.yaml
fi

# 启动 Fireworks Python 服务（后台）
python3 /app/scripts/fireworks_reg.py --host 0.0.0.0 --port 5000 &

# 启动 Go HTTP 服务（前台）
exec /app/reg-server --config /app/configs/config.yaml
