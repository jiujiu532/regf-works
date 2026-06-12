@echo off
title regf-works

cd /d "D:\Python\fireworks-re\grok-fireworks-reg"

REM 启动 Turnstile Solver（共享打码服务，端口 5072）
start "Turnstile-Solver" python solver\api_solver.py --browser_type camoufox --thread 4 --port 5072

REM 启动 Fireworks Python 服务（端口 5000）
start "FW-Python" python scripts\fireworks_reg.py --host 0.0.0.0 --port 5000

REM 启动 OpenRouter Python 服务（端口 5001）
start "OR-Python" python scripts\openrouter_reg.py --host 0.0.0.0 --port 5001

timeout /t 5 /nobreak >nul

echo.
echo Turnstile Solver: http://localhost:5072
echo Fireworks Service: http://localhost:5000
echo OpenRouter Service: http://localhost:5001
echo.
echo Web UI: http://127.0.0.1:8080
echo Login: admin / admin123
echo.

bin\reg-server.exe --config configs\config.yaml

pause
