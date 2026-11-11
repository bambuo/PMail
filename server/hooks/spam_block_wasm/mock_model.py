#!/usr/bin/env python3
"""本地测试用的 mock 模型服务，模拟 TensorFlow Serving 的 predictions 响应。

用法：
    python3 mock_model.py
然后确保插件 manifest 的 config.api_url 指向 http://127.0.0.1:8501/v1/models/emotion_model:predict
"""
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

SPAM_WORDS = ("广告", "优惠", "促销", "buy now", "promo", "discount")
BLACKMAIL_WORDS = ("诈骗", "敲诈", "银行卡", "密码", "转账", "blackmail")


def classify(text: str):
    lowered = text.lower()
    if any(word in lowered for word in BLACKMAIL_WORDS):
        return [0.01, 0.05, 0.94]
    if any(word in lowered for word in SPAM_WORDS):
        return [0.05, 0.90, 0.05]
    return [0.95, 0.03, 0.02]


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        try:
            payload = json.loads(raw or b"{}")
            text = " ".join(
                token
                for instance in payload.get("instances", [])
                for token in instance.get("token", [])
            )
        except Exception:
            text = ""
        body = json.dumps({"predictions": [classify(text)]}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        print("[mock-model] " + fmt % args)


if __name__ == "__main__":
    print("mock model listening on http://127.0.0.1:8501")
    HTTPServer(("127.0.0.1", 8501), Handler).serve_forever()
