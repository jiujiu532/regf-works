"""
Novita AI 注册脚本 - 验证流程
用法: python novita_test.py --email test@example.com --password TestPass123!
"""
import asyncio
import argparse
import httpx
import time
import json

SOLVER_URL = "http://127.0.0.1:5072"
NOVITA_BASE = "https://api-server.novita.ai"
NOVITA_ORIGIN = "https://novita.ai"
TURNSTILE_SITEKEY = "0x4AAAAAAAaG28VfN_OxkED8"
REGISTER_URL = f"https://novita.ai/user/register"


async def solve_turnstile(solver_url: str, proxy: str = "") -> str:
    """调用本地 Solver 获取 Turnstile token"""
    print("[*] 提交 Turnstile 验证任务...")
    
    params = {
        "url": REGISTER_URL,
        "sitekey": TURNSTILE_SITEKEY,
    }
    if proxy:
        params["proxy"] = proxy
    
    async with httpx.AsyncClient(timeout=10) as client:
        # Step 1: 提交任务
        r = await client.get(f"{solver_url}/turnstile", params=params)
        data = r.json()
        
        if data.get("errorId") != 0:
            raise Exception(f"提交任务失败: {data}")
        
        task_id = data["taskId"]
        print(f"[+] 任务已提交: {task_id}")
        
        # Step 2: 轮询结果
        for i in range(60):  # 最多等 60 秒
            await asyncio.sleep(2)
            r = await client.get(f"{solver_url}/result", params={"id": task_id})
            result = r.json()
            
            if result.get("status") == "processing":
                if i % 5 == 0:
                    print(f"[*] 等待验证码... ({i*2}s)")
                continue
            
            if result.get("errorId") == 0 and result.get("status") == "ready":
                token = result["solution"]["token"]
                print(f"[+] Turnstile 验证通过! token={token[:20]}...")
                return token
            
            if result.get("errorCode") == "ERROR_CAPTCHA_UNSOLVABLE":
                raise Exception("Solver 无法解决验证码")
            
            raise Exception(f"未知响应: {result}")
        
        raise Exception("Turnstile 超时 (120s)")


async def register(email: str, password: str, turnstile_token: str, invite_code: str = "", proxy: str = "") -> dict:
    """调用 Novita 注册 API"""
    print(f"[*] 注册中... email={email}")
    
    payload = {
        "email": email,
        "password": password,
        "confirmPassword": password,
        "redirectUrl": "/user/login",
        "cloudflareToken": turnstile_token,
        "allowNotification": True,
        "fromInviteCode": invite_code,
    }
    
    headers = {
        "Content-Type": "application/json",
        "Origin": NOVITA_ORIGIN,
        "Referer": f"{NOVITA_ORIGIN}/user/register",
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
    }
    
    transport = httpx.AsyncHTTPTransport(proxy=proxy) if proxy else None
    async with httpx.AsyncClient(timeout=30, transport=transport) as client:
        r = await client.post(
            f"{NOVITA_BASE}/v1/user/register",
            json=payload,
            headers=headers,
        )
        
        print(f"[*] HTTP {r.status_code}")
        
        try:
            data = r.json()
        except:
            data = {"raw": r.text[:500]}
        
        if r.status_code == 200:
            print(f"[+] 注册成功! 请检查邮箱激活账户")
            print(f"[+] 响应: {json.dumps(data, indent=2, ensure_ascii=False)}")
        else:
            print(f"[-] 注册失败: {json.dumps(data, indent=2, ensure_ascii=False)}")
        
        return data


async def main():
    parser = argparse.ArgumentParser(description="Novita AI 注册测试")
    parser.add_argument("--email", required=True, help="注册邮箱")
    parser.add_argument("--password", default="NovitaTest123!", help="密码 (至少8位,含字母+数字+特殊字符)")
    parser.add_argument("--invite", default="", help="邀请码 (可选)")
    parser.add_argument("--solver", default=SOLVER_URL, help="Solver 地址")
    parser.add_argument("--proxy", default="", help="代理地址")
    args = parser.parse_args()
    
    print("=" * 50)
    print("Novita AI 注册测试")
    print("=" * 50)
    print(f"邮箱: {args.email}")
    print(f"Solver: {args.solver}")
    print(f"代理: {args.proxy or '无'}")
    print()
    
    # Step 1: 解 Turnstile
    try:
        token = await solve_turnstile(args.solver, args.proxy)
    except Exception as e:
        print(f"[-] Turnstile 失败: {e}")
        return
    
    # Step 2: 注册
    try:
        result = await register(args.email, args.password, token, args.invite, args.proxy)
    except Exception as e:
        print(f"[-] 注册失败: {e}")
        return
    
    print()
    print("=" * 50)
    print("下一步: 去邮箱点击激活链接")
    print("=" * 50)


if __name__ == "__main__":
    asyncio.run(main())
