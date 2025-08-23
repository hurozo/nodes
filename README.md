# Hurozo Remote Nodes

### What is a “remote node”?

A remote node lets a graph running in your backend call into out‑of‑process code (e.g., a Python script running on another machine). At runtime, the backend node:

- Opens a temporary WebSocket channel and waits for a response.
- Publishes a remote_node event (with your node name, inputs, and a uuid) to the user’s WebSocket stream.
- Your remote worker, already connected to the WebSocket, receives that event, performs work, and sends back outputs
- The backend node returns those outputs to the graph. 

The "Hello World" worker in this repo demonstrates the remote half of the handshake: it registers + keeps a WebSocket connection, listens for events matching its `NODE_NAME`, computes outputs, and sends them back.

### High-level end‑to‑end architecture

###### Registration (HTTP)
- Remote workers periodically call `POST /api/remote_nodes/register with {name, inputs[], outputs[]}`. The backend stores (or refreshes) the registration and responds with {websocket_url, token} that the worker uses to connect. Registrations expire if not refreshed.

###### Execution (WebSocket)
- When a graph executes a RemoteNode, the backend:
    - Generates a short‑lived WS token and opens a waiting connection for a unique uuid. It also sends a remote_node event to the user stream with `{node, inputs, uuid}`. It then waits up to 600 seconds for a reply.
    - Your remote worker (connected using the token it received during registration) receives the event and must reply on the socket with `{node, outputs: {…}, uuid}`.

###### Security
- All API calls require Authorization: `Bearer <token>` (either a Firebase ID token or an API token you issued).
- WebSocket tokens are HMAC‑signed and expire after 60 seconds.

### Backend API & Message contracts

###### Register a remote node
Request:
```
POST /api/remote_nodes/register
Authorization: Bearer <HUROZO_TOKEN>
Content-Type: application/json

{
  "name": "ws_hello",
  "inputs": ["name"],
  "outputs": ["greeting", "shout"]
}
```

###### Semantics & constraints
- name is required and may not contain “/”. 
- outputs must contain at least one output. 
- The registration expires 10 minutes after creation; refresh before it expires

###### Response
```json
{
  "status": "registered",
  "websocket_url": "wss://…/Prod",
  "token": "<base64-ws-token>",
  "inputs": ["name"],
  "outputs": ["greeting", "shout"]
}
```

- The backend returns a WS URL and a short‑lived WS token bound to the user (the one behind HUROZO_TOKEN).

###### List active remote nodes

```
GET /api/remote_nodes
Authorization: Bearer <HUROZO_TOKEN>
```

Returns your non‑expired registrations:
```json
{
  "nodes": [
    {"name": "ws_hello", "inputs": ["name"], "outputs": ["greeting","shout"]}
  ]
}
```

### The remote worker
A remote worker must do two things:

- Register in a loop
- The example uses a background thread to call `POST /api/remote_nodes/register` every 60s:

```python
url = f"https://app.hurozo.com/api/remote_nodes/register"
headers = {"Authorization": f"Bearer {API_TOKEN}"}
payload = {"name": NODE_NAME, "inputs": NODE_INPUTS, "outputs": NODE_OUTPUTS}
res = requests.post(url, json=payload, headers=headers, timeout=60)
```

- Hold a WebSocket and serve requests and use the websocket_url and token returned from registration:

```python
async with websockets.connect(f"{url}?auth={token}") as ws:
    while True:
        msg = await ws.recv()
        payload = json.loads(msg)
        if payload.get("node") != NODE_NAME:
            continue  # ignore messages for other nodes
        inputs = payload.get("inputs", {})
        # ... compute outputs dict ...
        await ws.send(json.dumps({
            "node": NODE_NAME,
            "outputs": outputs,      # keys must match your outputs[] list
            "uuid": payload.get("uuid")  # echo uuid so backend correlates
        }))
```

#### Environment variables

`HUROZO_TOKEN` — API token for your user (Bearer token).
`HUROZO_API_URL` — API base (defaults to https://app.hurozo.com). Change to https://staging.hurozo.com for development setups
`NODE_NAME` — optional; default ws_hello.

### Common errors
- `Missing or invalid name` — you sent an empty name or it contained “/”. Fix your name.
- `At least one output required` — your outputs array was empty. Remote nodes must have an output defined. Define an output.
- `Invalid API token` / `Missing Authorization header` — your Bearer token is wrong or missing. Check your API Token. Ensure it's a write token.

### Operational tips & best practices

- Refresh aggressively. Re‑register at least every 60s (the example’s approach) to keep your node visible and ensure you always have a fresh WS token on hand. 
- Echo the uuid. The backend depends on it to correlate request→response. 
- Name uniqueness. Registrations are stored per user. Ppick unique names if you run multiple workers under the same account. 
- Map outputs carefully. The backend returns a tuple in the same order as your outputs list; if your worker mis‑names keys, the backend will return None or an empty string. 
- Deal with timeouts. Long jobs must respond within 600 seconds or the backend node returns empty values. Consider chunking or background job IDs if you need longer. 
- Permissions: Using a read‑only API token will block write endpoints; registration requires POST, so ensure your token has the right permission.

