#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
geo-tool 百度 SERP 抓取转发服务（宿主机侧）
=================================================
作用：Docker 容器出口 IP 被百度风控时，由宿主机（出口 IP 信誉正常）代为
抓取百度 SERP，再把 HTML 原样返回给容器内的后端服务。

调用方式（容器内后端）：
    GET http://host.docker.internal:18888/fetch?url=<urlencoded 百度 SERP URL>

返回：抓取成功 → 200 + HTML 原文；失败 → 4xx/5xx + 简短错误信息。

依赖：仅 Python 3 标准库，无需 pip 安装。
启动：
    nohup python3 scripts/baidu_proxy.py > /tmp/baidu_proxy.log 2>&1 &
    （如需开机自启，可注册为 launchd LaunchAgent）
"""
import http.server
import random
import socket
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

LISTEN_HOST = "0.0.0.0"
LISTEN_PORT = 18888
FETCH_TIMEOUT = 20

USER_AGENTS = [
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/122.0.0.0",
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
]

COOKIES = [
    "BAIDUID=0A1B2C3D4E5F60718293A4B5C6D7E8F9:FG=1; BIDUPSID=0A1B2C3D4E5F60718293A4B5C6D7E8F9; PSTM=1700000000; H_PS_PSSID=39512_39109; BDSVRTM=0",
    "BAIDUID=9F8E7D6C5B4A3120987654ABCDEF1234:FG=1; BIDUPSID=9F8E7D6C5B4A3120987654ABCDEF1234; PSTM=1710000000; H_PS_PSSID=39456_39080; BDSVRTM=0",
]


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def do_GET(self):
        parsed = urllib.parse.urlparse(self.path)
        if parsed.path != "/fetch":
            self.send_error(404, "not found")
            return
        params = urllib.parse.parse_qs(parsed.query)
        target = (params.get("url") or [""])[0]
        if not target or not target.startswith("http"):
            self.send_error(400, "missing/invalid url param")
            return
        self._forward(target)

    def _forward(self, target):
        req = urllib.request.Request(
            target,
            headers={
                "User-Agent": random.choice(USER_AGENTS),
                "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
                "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
                "Cookie": random.choice(COOKIES),
                "Referer": "https://www.baidu.com/",
                "Connection": "close",
            },
        )
        try:
            with urllib.request.urlopen(req, timeout=FETCH_TIMEOUT) as resp:
                body = resp.read()
                self.send_response(resp.status)
                self.send_header("Content-Type", resp.headers.get("Content-Type", "text/html; charset=utf-8"))
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
        except urllib.error.HTTPError as e:
            self.send_error(e.code, "upstream http error")
        except Exception as e:  # noqa: BLE001
            msg = str(e)[:200]
            try:
                self.send_error(502, "upstream fetch failed: %s" % msg)
            except Exception:  # noqa: BLE001
                pass

    def log_message(self, fmt, *args):  # 静默日志，避免刷屏
        pass


class ThreadingHTTPServer(http.server.ThreadingHTTPServer):
    daemon_threads = True


if __name__ == "__main__":
    server = ThreadingHTTPServer((LISTEN_HOST, LISTEN_PORT), Handler)
    print("baidu_proxy listening on %s:%d" % (LISTEN_HOST, LISTEN_PORT), flush=True)
    server.serve_forever()
