"""Ollama 注册探测脚本 - 逐步验证完整流程"""
import httpx
import time
import json
import random
import string
import re
from urllib.parse import urlparse, parse_qs

SOLVER = "http://127.0.0.1:5072"
AHEM = "https://mail.jiuuij.de5.net"
SITEKEY = "0x4AAAAAAAMNIvC45A4Wjjln"
UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"


def solve_turnstile(page_url):
    print("[*] Turnstile 打码...")
    r = httpx.get(f"{SOLVER}/turnstile", params={"url": page_url, "sitekey": SITEKEY}, timeout=10)
    task_id = r.json()["taskId"]
    for i in range(60):
        time.sleep(2)
        r = httpx.get(f"{SOLVER}/result", params={"id": task_id}, timeout=10)
        d = r.json()
        if d.get("status") == "processing":
            if i % 5 == 0:
                print(f"[*] 等待... ({i*2}s)")
            continue
        if d.get("status") == "ready":
            print("[+] 打码成功!")
            return d["solution"]["token"]
        print(f"[-] 失败: {d}")
        return None
    return None


def get_otp_from_email(email_prefix):
    """从 AHEM 获取 Ollama 6位验证码"""
    for attempt in range(20):
        time.sleep(2)
        try:
            r = httpx.get(f"{AHEM}/api/mailbox/{email_prefix}/email", timeout=10)
            if r.status_code != 200:
                continue
            mails = r.json()
            if not mails:
                if attempt % 3 == 0:
                    print(f"[*] 等待邮件... ({attempt*2}s)")
                continue
            for mail in mails:
                eid = mail.get("emailId", "")
                r2 = httpx.get(f"{AHEM}/api/mailbox/{email_prefix}/email/{eid}", timeout=10)
                body = r2.json()
                text = body.get("text", "") or body.get("html", "")
                # 找 6 位数字验证码
                codes = re.findall(r'\b(\d{6})\b', text)
                if codes:
                    print(f"[+] 验证码: {codes[0]}")
                    return codes[0]
                print(f"[*] 邮件无6位码, subject={mail.get('subject','')}")
                print(f"[*] 内容前300字: {text[:300]}")
        except Exception as e:
            if attempt % 3 == 0:
                print(f"[*] 获取邮件出错: {e}")
    print("[-] 邮件超时")
    return None


def main():
    prefix = "".join(random.choices(string.ascii_lowercase + string.digits, k=10))
    email = f"{prefix}@linux.jiuuij.bond"
    print(f"{'='*60}")
    print(f"Ollama 注册测试")
    print(f"Email: {email}")
    print(f"{'='*60}")

    client = httpx.Client(timeout=30, follow_redirects=True, headers={"User-Agent": UA})

    # ═══ Step 1: 获取注册页 ═══
    print("\n[Step 1] 获取注册页...")
    r = client.get("https://signin.ollama.com/sign-up")
    final_url = str(r.url)
    parsed = urlparse(final_url)
    params = parse_qs(parsed.query)
    auth_session = params.get("authorization_session_id", [""])[0]
    print(f"[+] session: {auth_session}")
    print(f"[+] cookies: {list(dict(client.cookies).keys())}")

    # ═══ Step 2: Turnstile 打码 ═══
    print("\n[Step 2] Turnstile 打码...")
    cf_token = solve_turnstile("https://signin.ollama.com/sign-up")
    if not cf_token:
        print("[-] 打码失败")
        return

    # ═══ Step 3: 提交邮箱 (Next.js Server Action FormData) ═══
    print(f"\n[Step 3] 提交邮箱: {email}")

    # 构造 signals（WorkOS radar signals - 浏览器指纹 JSON base64）
    import base64
    signals = {
        "createdAtMs": int(time.time() * 1000),
        "timezone": "Asia/Shanghai",
        "language": "en-US",
        "hardwareConcurrency": 8,
        "webdriver": False,
        "userAgent": UA,
        "appVersion": UA.replace("Mozilla/", ""),
        "platform": "Win32",
        "screen": {
            "width": 1920, "height": 1080,
            "availWidth": 1920, "availHeight": 1040,
            "windowOuterWidth": 1920, "windowOuterHeight": 1040
        }
    }
    signals_b64 = base64.b64encode(json.dumps(signals).encode()).decode()

    # FormData 字段（来自真实表单分析）
    files = {
        "signals": (None, signals_b64),
        "email": (None, email),
        "intent": (None, "sign-up"),
        "redirect_uri": (None, "https://ollama.com/auth/callback"),
        "authorization_session_id": (None, auth_session),
        "cf-turnstile-response": (None, cf_token),
        "0": (None, '["$K1"]'),
        "$ACTION_ID_cbdba1d1e041e600f1d7877f5e502011e412c3cd": (None, ""),
    }

    headers = {
        "Accept": "text/x-component",
        "Next-Action": "cbdba1d1e041e600f1d7877f5e502011e412c3cd",
        "Next-Router-State-Tree": '%5B%22%22%2C%7B%22children%22%3A%5B%22(main)%22%2C%7B%22children%22%3A%5B%22(root)%22%2C%7B%22children%22%3A%5B%22sign-up%22%2C%7B%22children%22%3A%5B%22__PAGE__%22%2C%7B%7D%5D%7D%5D%7D%5D%7D%5D%7D%5D',
        "Origin": "https://signin.ollama.com",
        "Referer": final_url,
    }

    r = client.post(final_url, files=files, headers=headers)
    print(f"[*] HTTP {r.status_code}")
    resp = r.text
    print(f"[*] 响应: {resp[:500]}")

    # 检查是否有 pending_authentication_token
    pat_match = re.search(r'pending_authentication_token["\s:]*["\']([^"\']+)', resp)
    if pat_match:
        print(f"[+] pending_auth_token: {pat_match.group(1)}")

    # 不管响应内容，检查邮箱是否收到验证码
    print(f"\n[Step 4] 等待验证码邮件...")
    code = get_otp_from_email(prefix)
    if not code:
        print("[-] 没收到验证码，注册可能失败")
        return

    # ═══ Step 5: 提交验证码 ═══
    print(f"\n[Step 5] 提交验证码: {code}")

    verify_files = {
        "signals": (None, signals_b64),
        "code": (None, code),
        "email": (None, email),
        "intent": (None, "magic-code"),
        "redirect_uri": (None, "https://ollama.com/auth/callback"),
        "authorization_session_id": (None, auth_session),
        "0": (None, '["$K1"]'),
        "$ACTION_ID_cbdba1d1e041e600f1d7877f5e502011e412c3cd": (None, ""),
    }

    r = client.post(final_url, files=verify_files, headers=headers)
    print(f"[*] HTTP {r.status_code}")
    resp = r.text
    print(f"[*] 响应: {resp[:800]}")
    print(f"[*] Cookies: {dict(client.cookies)}")

    # 检查是否有重定向 URL
    redirect_match = re.search(r'(https://ollama\.com/auth/callback[^\s"<>\']*)', resp)
    if redirect_match:
        callback_url = redirect_match.group(1)
        print(f"\n[Step 6] 回调: {callback_url}")
        r = client.get(callback_url)
        print(f"[*] HTTP {r.status_code}, URL: {r.url}")
        print(f"[*] Cookies: {dict(client.cookies)}")

    # 看看是否已经登录
    print(f"\n[Step 7] 检查登录状态...")
    r = client.get("https://ollama.com/settings/keys")
    print(f"[*] HTTP {r.status_code}, URL: {r.url}")
    if "keys" in str(r.url).lower() and r.status_code == 200:
        print("[+] 已登录! 可以创建 API Key")
    else:
        print(f"[*] 页面内容: {r.text[:300]}")


if __name__ == "__main__":
    main()
