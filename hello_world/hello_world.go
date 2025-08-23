// go:build ignore
// Save as main.go
//
// Example remote node that registers via HTTP and responds over WebSocket.
//
// Env vars:
//   HUROZO_TOKEN    - API token created in settings
//   HUROZO_API_URL  - Base URL of the Hurozo instance (default https://app.hurozo.com)
//   NODE_NAME       - Optional node name (default ws_hello_go)
//
// Build & run:
//   go mod init hurozo-node
//   go get github.com/gorilla/websocket
//   HUROZO_TOKEN=YOUR_TOKEN go run .
//      (optionally set HUROZO_API_URL and NODE_NAME)

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	apiToken = getenv("HUROZO_TOKEN", "YOUR_TOKEN")
	baseURL  = strings.TrimRight(getenv("HUROZO_API_URL", "https://app.hurozo.com"), "/")
	nodeName = getenv("NODE_NAME", "ws_hello_go")

	nodeInputs  = []string{"name"}
	nodeOutputs = []string{"greeting", "shout"}

	httpClient = &http.Client{
		Timeout: 60 * time.Second,
	}
)

// shared registration info
type wsInfo struct {
	URL   string `json:"websocket_url"`
	Token string `json:"token"`
}

type wsInfoSafe struct {
	mu sync.RWMutex
	v  wsInfo
}

func (s *wsInfoSafe) Get() wsInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.v
}

func (s *wsInfoSafe) Set(v wsInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.v = v
}

var shared wsInfoSafe

func main() {
	// Kick off registration loop in background
	go registerLoop(context.Background())

	// Run websocket loop (foreground)
	websocketLoop(context.Background())
}

func registerLoop(ctx context.Context) {
	type registerReq struct {
		Name    string   `json:"name"`
		Inputs  []string `json:"inputs"`
		Outputs []string `json:"outputs"`
	}

	endpoint := fmt.Sprintf("%s/api/remote_nodes/register", baseURL)
	body := registerReq{
		Name:    nodeName,
		Inputs:  nodeInputs,
		Outputs: nodeOutputs,
	}

	for {
		// Marshal payload
		buf, _ := json.Marshal(body)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+apiToken)
			req.Header.Set("Content-Type", "application/json")

			resp, err := httpClient.Do(req)
			if err == nil {
				func() {
					defer resp.Body.Close()
					b, _ := io.ReadAll(resp.Body)

					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						var data wsInfo
						if err := json.Unmarshal(b, &data); err == nil && data.URL != "" && data.Token != "" {
							shared.Set(data)
						}
					} else {
						// Optional: print server error body for troubleshooting
						fmt.Println("Registration failed status:", resp.Status, string(b))
					}
				}()
			} else {
				fmt.Println("Registration request error:", err)
			}
		} else {
			fmt.Println("Registration build error:", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(60 * time.Second):
		}
	}
}

func websocketLoop(ctx context.Context) {
	for {
		info := shared.Get()
		if info.URL == "" || info.Token == "" {
			time.Sleep(1 * time.Second)
			continue
		}

		u, err := url.Parse(info.URL)
		if err != nil {
			fmt.Println("Invalid websocket_url:", err)
			time.Sleep(5 * time.Second)
			continue
		}
		// append ?auth=token
		q := u.Query()
		q.Set("auth", info.Token)
		u.RawQuery = q.Encode()

		dialer := websocket.Dialer{
			HandshakeTimeout:  30 * time.Second,
			EnableCompression: true,
		}

		conn, _, err := dialer.DialContext(ctx, u.String(), nil)
		if err != nil {
			fmt.Println("WebSocket dial error:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Set a generous read deadline that will be refreshed on each message
		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		})

		// Simple ping loop to keep the connection alive
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			defer close(done)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_ = conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second))
				}
			}
		}()

		// Read loop
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("WebSocket read error:", err)
				_ = conn.Close()
				<-done
				time.Sleep(5 * time.Second)
				break
			}
			_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))

			var payload map[string]any
			if err := json.Unmarshal(msg, &payload); err != nil {
				// ignore unparseable frames
				continue
			}

			if node, _ := payload["node"].(string); node != nodeName {
				continue
			}

			uuid, _ := payload["uuid"].(string)

			// inputs can be arbitrary; expect "name"
			var inputs map[string]any
			if m, ok := payload["inputs"].(map[string]any); ok {
				inputs = m
			}

			name := strings.TrimSpace(asString(inputs["name"]))
			if name == "" {
				name = "world"
			}

			outputs := map[string]string{
				"greeting": fmt.Sprintf("Hello %s", name),
				"shout":    fmt.Sprintf("HELLO %s", strings.ToUpper(name)),
			}

			resp := map[string]any{
				"node":    nodeName,
				"outputs": outputs,
				"uuid":    uuid,
			}

			out, _ := json.Marshal(resp)
			if err := conn.WriteMessage(websocket.TextMessage, out); err != nil {
				fmt.Println("WebSocket write error:", err)
				_ = conn.Close()
				<-done
				time.Sleep(5 * time.Second)
				break
			}
		}
	}
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case float64: // JSON numbers decode to float64
		// remove .0 if present
		s := fmt.Sprintf("%v", t)
		s = strings.TrimSuffix(s, ".0")
		return s
	case int, int64, int32, uint64, uint32, uint:
		return fmt.Sprintf("%v", t)
	case nil:
		return ""
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

