import httpx, time, json, random, string, re, secrets

solver = "http://127.0.0.1:5072"
ahem = "https://mail.jiuuij.de5.net"
novita_api = "https://api-server.novita.ai"
sitekey = "0x4AAAAAAAaG28VfN_OxkED8"
proxy = ""  # 先不用代理测试激活问题

def solve(label=""):
    print(f"[*] Turnstile {label}...")
    params = {"url":"https://novita.ai/user/register","sitekey":sitekey}
    r = httpx.get(f"{solver}/turnstile", params=params, timeout=10)
    tid = r.json()["taskId"]
    for i in range(60):
        time.sleep(2)
        r = httpx.get(f"{solver}/result", params={"id":tid}, timeout=10)
        d = r.json()
        if d.get("status")=="processing": continue
        if d.get("status")=="ready":
            print(f"[+] OK")
            return d["solution"]["token"]
        print(f"[-] fail: {d}")
        return None
    return None

prefix = "".join(random.choices(string.ascii_lowercase + string.digits, k=10))
email = f"{prefix}@linux.jiuuij.bond"
pwd_list = [secrets.choice(string.ascii_letters) for _ in range(8)]
pwd_list += [secrets.choice(string.digits) for _ in range(3)]
pwd_list += [secrets.choice("!@#$%")]
random.shuffle(pwd_list)
password = "".join(pwd_list)
print(f"Email: {email}")
print(f"Password: {password}")

client = httpx.Client(timeout=30)
headers = {"Content-Type":"application/json","Origin":"https://novita.ai","Referer":"https://novita.ai/user/register"}

# 1: Register
cf1 = solve("register")
if not cf1: exit(1)
print("[*] Registering...")
r = client.post(f"{novita_api}/v1/user/register", json={
    "email":email,"password":password,"confirmPassword":password,
    "redirectUrl":"/user/login","cloudflareToken":cf1,
    "allowNotification":True,"fromInviteCode":""
}, headers=headers)
print(f"  {r.status_code}: {r.text[:200]}")
if r.status_code != 200: exit(1)

# 2: Wait email
print("[*] Waiting for email...")
token = None
for a in range(15):
    time.sleep(2)
    try:
        r2 = httpx.get(f"{ahem}/api/mailbox/{prefix}/email", timeout=10)
        for m in r2.json():
            eid = m.get("emailId","")
            r3 = httpx.get(f"{ahem}/api/mailbox/{prefix}/email/{eid}", timeout=10)
            html = r3.json().get("html","")
            mt = re.search(r"token=([A-Za-z0-9_-]+)", html)
            if mt:
                token = mt.group(1)
                break
        if token: break
    except: pass
    if a%3==0: print(f"  waiting ({a*2}s)")
if not token:
    print("[-] NO TOKEN")
    exit(1)
print(f"[+] token: {token}")

# 3: Activate
cf2 = solve("activate")
if not cf2: exit(1)
print("[*] Activating...")
r = client.post(f"{novita_api}/v1/user/email/verify", json={
    "token":token,"email":email,"cloudflareToken":cf2
}, headers={"Content-Type":"application/json","Origin":"https://novita.ai","Referer":"https://novita.ai/user/email-validate"})
print(f"  HTTP {r.status_code}")
print(f"  Body: {r.text}")
if r.status_code != 200:
    print("[-] Activate FAILED")
    exit(1)
print("[+] Activated!")

# 4: Login
cf3 = solve("login")
if not cf3: exit(1)
print("[*] Login...")
r = client.post(f"{novita_api}/v1/user/login", json={
    "email":email,"password":password,"cloudflareToken":cf3
}, headers=headers)
login_data = r.json()
jwt = login_data.get("token","")
if not jwt:
    print(f"[-] Login fail: {r.text[:200]}")
    exit(1)
print(f"[+] JWT OK ({len(jwt)} chars)")

# 5: Questionnaire
print("[*] Questionnaire...")
r = client.post(f"{novita_api}/v1/user/questionnaire", json={
    "name":"Alex Chen","company":"TechFlow","role":"developer","monthlySpend":"0-100"
}, headers={"Content-Type":"application/json","Authorization":f"Bearer {jwt}"})
print(f"  {r.status_code}: {r.text[:100]}")

# 6: API Key
print("[*] Create API Key...")
r = client.post(f"{novita_api}/v2/user/key", json={"name":"auto"}, headers={
    "Content-Type":"application/json","Authorization":f"Bearer {jwt}"
})
key = r.json().get("apiKey","")
print(f"  API Key: {key}")

print(f"\n{'='*50}")
print(f"Email: {email}")
print(f"Password: {password}")
print(f"API Key: {key}")
print(f"{'='*50}")
