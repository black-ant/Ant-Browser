package launchcode

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

const testAPIKey = "test-key"

// fakeChrome 模拟 Chrome 调试端口：/json/* 按请求 Host 拼地址（与 Chrome 一致），
// /devtools/* 接受 WebSocket 升级并回显消息。
type fakeChrome struct {
	server *httptest.Server
	mu     sync.Mutex
	hosts  []string
	paths  []string
	keys   []string
}

func newFakeChrome(t *testing.T) *fakeChrome {
	t.Helper()
	fc := &fakeChrome{}
	upgrader := websocket.Upgrader{}
	fc.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fc.mu.Lock()
		fc.hosts = append(fc.hosts, r.Host)
		fc.paths = append(fc.paths, r.URL.Path)
		fc.keys = append(fc.keys, r.Header.Get(DefaultAPIKeyHeader))
		fc.mu.Unlock()

		path := strings.TrimSuffix(r.URL.Path, "/")
		switch {
		case strings.HasPrefix(path, "/devtools/"):
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			for {
				kind, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				if err := conn.WriteMessage(kind, msg); err != nil {
					return
				}
			}
		case path == "/json/version":
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"Browser":              "Chrome/146.0.0.0",
				"Protocol-Version":     "1.3",
				"webSocketDebuggerUrl": "ws://" + r.Host + "/devtools/browser/b-1",
			})
		case path == "/json" || path == "/json/list":
			writeJSON(w, http.StatusOK, []map[string]interface{}{
				{
					"id":                   "page-1",
					"type":                 "page",
					"title":                "A & B",
					"devtoolsFrontendUrl":  "/devtools/inspector.html?ws=" + r.Host + "/devtools/page/page-1",
					"webSocketDebuggerUrl": "ws://" + r.Host + "/devtools/page/page-1",
				},
				{
					"id":                   "page-2",
					"type":                 "page",
					"devtoolsFrontendUrl":  "https://chrome-devtools-frontend.appspot.com/serve_rev/@rev/inspector.html?ws=" + r.Host + "/devtools/page/page-2",
					"webSocketDebuggerUrl": "ws://" + r.Host + "/devtools/page/page-2",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fc.server.Close)
	return fc
}

func (fc *fakeChrome) port(t *testing.T) int {
	t.Helper()
	_, rawPort, err := net.SplitHostPort(fc.server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split fake chrome addr: %v", err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatalf("parse fake chrome port: %v", err)
	}
	return port
}

func (fc *fakeChrome) lastRequest() (host, path, key string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if len(fc.paths) == 0 {
		return "", "", ""
	}
	last := len(fc.paths) - 1
	return fc.hosts[last], fc.paths[last], fc.keys[last]
}

func newAuthTestServer(enabled bool, apiKey string) *LaunchServer {
	server := NewLaunchServer(nil, nil, nil, 0)
	server.SetAPIAuthConfig(APIAuthConfig{Enabled: enabled, APIKey: apiKey})
	return server
}

func getJSON(t *testing.T, url string, header http.Header, out interface{}) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func keyHeader(key string) http.Header {
	h := http.Header{}
	if key != "" {
		h.Set(DefaultAPIKeyHeader, key)
	}
	return h
}

// dialWS 走代理做 WebSocket 升级并回显一条消息；返回升级响应的状态码。
func dialWS(t *testing.T, wsURL string, header http.Header) int {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if resp == nil {
			t.Fatalf("dial %s: %v", wsURL, err)
		}
		return resp.StatusCode
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"id":1}`)); err != nil {
		t.Fatalf("ws write: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if string(msg) != `{"id":1}` {
		t.Fatalf("ws echo = %q", msg)
	}
	return resp.StatusCode
}
