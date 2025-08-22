"""Example remote node that registers via websocket and responds to input.

Set the environment variables HUROZO_TOKEN and HUROZO_API_URL before running:
HUROZO_TOKEN: API token created in settings.
HUROZO_API_URL: Base URL of the Hurozo instance (default https://app.hurozo.com).
"""

import asyncio
import json
import os
import threading
import time

import requests
import websockets

API_TOKEN = os.environ.get("HUROZO_TOKEN", "YOUR_TOKEN")
BASE_URL = os.environ.get("HUROZO_API_URL", "https://app.hurozo.com")
NODE_NAME = os.environ.get("NODE_NAME", "ws_hello")
NODE_INPUTS = ["name"]
NODE_OUTPUTS = ["greeting", "shout"]

ws_info = {}
ws_lock = threading.Lock()


def register_loop():
    url = f"{BASE_URL}/api/remote_nodes/register"
    headers = {"Authorization": f"Bearer {API_TOKEN}"}
    payload = {"name": NODE_NAME, "inputs": NODE_INPUTS, "outputs": NODE_OUTPUTS}
    while True:
        try:
            res = requests.post(url, json=payload, headers=headers, timeout=60)
            data = res.json()
            if res.ok and "websocket_url" in data:
                with ws_lock:
                    ws_info.update(data)
        except Exception as e:
            print("Registration failed:", e)
        time.sleep(60)


async def websocket_loop():
    while True:
        with ws_lock:
            url = ws_info.get("websocket_url")
            token = ws_info.get("token")
        if not url or not token:
            await asyncio.sleep(1)
            continue
        try:
            async with websockets.connect(f"{url}?auth={token}") as ws:
                while True:
                    msg = await ws.recv()
                    try:
                        payload = json.loads(msg)
                    except Exception:
                        continue
                    if payload.get("node") != NODE_NAME:
                        continue
                    inputs = payload.get("inputs", {})
                    name = str(inputs.get("name") or "").strip() or "world"
                    outputs = {
                        "greeting": f"Hello {name}",
                        "shout": f"HELLO {name.upper()}"
                    }
                    await ws.send(json.dumps({
                        "node": NODE_NAME,
                        "outputs": outputs,
                        "uuid": payload.get("uuid")
                    }))
        except Exception as e:
            print("Websocket error:", e)
            await asyncio.sleep(5)


if __name__ == "__main__":
    threading.Thread(target=register_loop, daemon=True).start()
    asyncio.run(websocket_loop())

