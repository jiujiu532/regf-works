@echo off
title regf-works

cd /d "D:\Python\fireworks-re\grok-fireworks-reg"

echo ========================================
echo   regf-works 启动脚本
echo ========================================
echo.
echo 请先确保 Turnstile Solver 已启动：
echo   路径: D:\Python\openrouter rot\openrouter\启动打码服务.bat
echo   端口: 5072
echo.
echo 按任意键继续启动其他服务...
pause >nul

REM 启动 Fireworks Python 服务（端口 5000）
start "FW-Python" python scripts\fireworks_reg.py --host 0.0.0.0 --port 5000

REM 启动 OpenRouter Python 服务（端口 5001）
start "OR-Python" python scripts\openrouter_reg.py --host 0.0.0.0 --port 5001

timeout /t 3 /nobreak >nul

echo.
echo ========================================
echo   服务已启动
echo ========================================
echo.
echo Turnstile Solver:   http://localhost:5072  (需手动启动)
echo Fireworks Service:  http://localhost:5000
echo OpenRouter Service: http://localhost:5001
echo.
echo Web UI: http://127.0.0.1:8080
echo Login:  admin / admin123
echo.
echo ========================================
echo.

bin\reg-server.exe --config configs\config.yaml

pause
