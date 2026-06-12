@echo off
title regf-works

cd /d "D:\Python\fireworks-re\grok-fireworks-reg"

start "FW-Python" python scripts\fireworks_reg.py --host 0.0.0.0 --port 5000

timeout /t 3 /nobreak >nul

echo.
echo Open http://127.0.0.1:8080
echo Login: admin / admin123
echo.

bin\reg-server.exe --config configs\config.yaml

pause
