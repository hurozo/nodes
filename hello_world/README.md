# Hurozo Remote Node example: Hello World

This example demonstrates how to implement a remote node that communicates with Hurozo. The example script is named `hello_world.py` and showcases how a node can register itself, listen for incoming execution requests, and send back outputs.

## Environment Variables
Set the following environment variables before running the script:

- `HUROZO_TOKEN`: API token created in the Hurozo settings page.
- `HUROZO_API_URL`: Base URL of the Hurozo instance (defaults to `https://app.hurozo.com`).
- `NODE_NAME`: Optional node name (defaults to `ws_hello`).

## How It Works

### 1. Registration Loop
`hello_world.py` includes a background thread that repeatedly registers the node with the Hurozo API. It sends the node name along with its input and output fields:

```python
payload = {"name": NODE_NAME, "inputs": NODE_INPUTS, "outputs": NODE_OUTPUTS}
requests.post(url, json=payload, headers=headers, timeout=60)
```

The node will be available in Hurozo under 'components'.

### 2. Websocket Loop
Once registration succeeds, the script connects to the provided WebSocket URL and waits for execution messages. Each message contains the node's inputs, which are used to craft responses:

```python
inputs = payload.get("inputs", {})
name = str(inputs.get("name") or "").strip() or "world"
outputs = {
    "greeting": f"Hello {name}",
    "shout": f"HELLO {name.upper()}"
}
```

After computing the outputs, the node sends them back along with the execution identifier.
If Hurozo didn't receive a registration from your node within 10 minutes it'll be deemed dead and will be removed.

## Running the Example

1. Install dependencies:
   ```bash
   pip install -r requirements.txt
   ```
2. Set the required environment variables.
3. Run the script:
   ```bash
   python hello_world.py
   ```

The node will register itself and respond with a friendly greeting whenever it receives an execution request.
