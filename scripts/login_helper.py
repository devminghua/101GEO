#!/usr/bin/env python3
"""GEO 登录辅助：自动过滑块验证码，返回 token。仅供本机链路检验使用。"""
import json
import sys
import urllib.request

BASE = "http://127.0.0.1:8080"


def post(path, body):
    req = urllib.request.Request(
        BASE + path,
        data=json.dumps(body).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=15) as r:
        return json.loads(r.read())


def get(path):
    with urllib.request.urlopen(BASE + path, timeout=15) as r:
        return json.loads(r.read())


def login(username, password):
    # 1) 拿一次性滑块凭证
    cap = get("/api/auth/captcha")
    cid = cap["data"]["id"]
    # 2) 模拟真人拖到最右的轨迹
    track = [0, 10, 30, 70, 130, 200, 250, 272]
    slide_x = 272
    # 3) 登录
    return post(
        "/api/auth/login",
        {
            "username": username,
            "password": password,
            "captcha_id": cid,
            "slide_x": slide_x,
            "track": track,
        },
    )


if __name__ == "__main__":
    u, p = sys.argv[1], sys.argv[2]
    r = login(u, p)
    if r.get("code") == 0:
        print(r["data"]["token"])
    else:
        print("LOGIN_FAIL:" + json.dumps(r, ensure_ascii=False), file=sys.stderr)
        sys.exit(1)
