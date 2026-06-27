package proxy

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ProbeResult 协议探测结果。用于导入时自动判定 host:port[:user:pass] 的真实协议，
// 并把代理网关自身的响应（如 403「china IP is not allow」、407 鉴权）暴露给用户，
// 而不是只甩一个看不懂的 EOF / 连接被强制关闭。
type ProbeResult struct {
	// Protocol 探测出的协议："socks5" | "http" | ""（两者都不是 / 不可达）
	Protocol string `json:"protocol"`
	// Reachable 能否对 server:port 建立 TCP 连接
	Reachable bool `json:"reachable"`
	// Usable 代理握手 / CONNECT 是否完整成功（可真正转发流量）
	Usable bool `json:"usable"`
	// NeedAuth 代理要求鉴权（SOCKS5 仅接受 user/pass，或 HTTP 407），
	// 但提供的凭据缺失或被拒
	NeedAuth bool `json:"needAuth"`
	// GatewayStatus HTTP 代理在 CONNECT 阶段返回的状态码（如 403 / 407），SOCKS5 不适用
	GatewayStatus int `json:"gatewayStatus"`
	// GatewayMessage 网关自身的响应文本（HTTP 状态行 / 正文片段，或 SOCKS5 失败原因），
	// 用于向用户解释“代理活着但拒绝了你”这类情况
	GatewayMessage string `json:"gatewayMessage"`
	// LatencyMs TCP 连接建立耗时
	LatencyMs int64 `json:"latencyMs"`
	// Error 探测过程的错误（不可达 / 解析失败等）
	Error string `json:"error"`
}

// probeConnectTarget 是 HTTP CONNECT 探测用的目标。选 gstatic 是因为它全球可达、
// 不易被目标侧拦截，这样 CONNECT 失败基本只可能来自代理网关本身。
const probeConnectTarget = "www.gstatic.com:443"

// ProbeBareProxy 探测无协议头的裸代理 host:port[:user:pass] 实际是 SOCKS5 还是 HTTP。
// 先做 SOCKS5 字节握手（确定性最高，HTTP 代理不会回 0x05），失败再尝试 HTTP CONNECT。
func ProbeBareProxy(host string, port int, username, password string, timeout time.Duration) ProbeResult {
	res := ProbeResult{}
	if strings.TrimSpace(host) == "" || port <= 0 || port > 65535 {
		res.Error = "代理地址或端口无效"
		return res
	}
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	endpoint := net.JoinHostPort(host, strconv.Itoa(port))

	// 先确认 TCP 可达，并记录连接延迟。
	start := time.Now()
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	res.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = fmt.Sprintf("无法连接代理服务器: %v", err)
		return res
	}
	conn.Close()

	// 1) SOCKS5 握手探测
	if socks := probeSocks5(endpoint, username, password, timeout); socks.Protocol == "socks5" {
		return socks
	}

	// 2) HTTP 代理 CONNECT 探测
	if httpRes := probeHTTP(endpoint, username, password, timeout); httpRes.Protocol == "http" {
		return httpRes
	}

	// TCP 通但两种协议握手都不成立：可能是其它协议（vmess/vless/trojan…）或被网关静默拒绝。
	res.Reachable = true
	res.Error = "已连接服务器，但既不是标准 SOCKS5 也不是 HTTP 代理（可能是机场节点协议或被网关拒绝）"
	return res
}

// probeSocks5 发送 SOCKS5 greeting，并据返回的版本字节判定协议。
func probeSocks5(endpoint, username, password string, timeout time.Duration) ProbeResult {
	res := ProbeResult{}
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer conn.Close()
	deadline := time.Now().Add(timeout)
	_ = conn.SetDeadline(deadline)

	// greeting：同时提供「无鉴权(0x00)」与「用户名/密码(0x02)」两种方法，
	// 让服务器自己挑。这样无论代理是否要鉴权都能正确判定。
	greeting := []byte{0x05, 0x02, 0x00, 0x02}
	if _, err := conn.Write(greeting); err != nil {
		res.Error = err.Error()
		return res
	}

	reply := make([]byte, 2)
	if _, err := readFull(conn, reply); err != nil {
		// 没回 2 字节版本应答 —— 不是 SOCKS5。
		return res
	}
	if reply[0] != 0x05 {
		// 首字节不是 0x05（HTTP 代理会回 ASCII 'H'=0x48）—— 不是 SOCKS5。
		return res
	}

	// 确认是 SOCKS5 服务器。
	res.Protocol = "socks5"
	res.Reachable = true
	method := reply[1]

	switch method {
	case 0xFF:
		// 服务器拒绝了我们提供的所有方法（通常是需要它支持的特定鉴权）。
		res.NeedAuth = true
		res.GatewayMessage = "SOCKS5 服务器拒绝了无鉴权与用户名/密码方法"
		return res
	case 0x02:
		// 需要用户名/密码鉴权。
		if username == "" {
			res.NeedAuth = true
			res.GatewayMessage = "SOCKS5 服务器要求用户名/密码鉴权，但未提供账号"
			return res
		}
		if !socks5UserPassAuth(conn, username, password) {
			res.NeedAuth = true
			res.GatewayMessage = "SOCKS5 用户名/密码鉴权失败"
			return res
		}
		res.Usable = true
		return res
	case 0x00:
		// 无需鉴权即接受。
		res.Usable = true
		return res
	default:
		res.GatewayMessage = fmt.Sprintf("SOCKS5 服务器返回未知鉴权方法 0x%02x", method)
		return res
	}
}

// socks5UserPassAuth 执行 RFC1929 用户名/密码子协商，返回是否通过。
func socks5UserPassAuth(conn net.Conn, username, password string) bool {
	if len(username) > 255 || len(password) > 255 {
		return false
	}
	buf := make([]byte, 0, 3+len(username)+len(password))
	buf = append(buf, 0x01)
	buf = append(buf, byte(len(username)))
	buf = append(buf, username...)
	buf = append(buf, byte(len(password)))
	buf = append(buf, password...)
	if _, err := conn.Write(buf); err != nil {
		return false
	}
	resp := make([]byte, 2)
	if _, err := readFull(conn, resp); err != nil {
		return false
	}
	// resp[0]=版本(0x01)，resp[1]=0x00 表示成功。
	return resp[1] == 0x00
}

// probeHTTP 通过 CONNECT 隧道探测 HTTP 代理，并抓取网关返回的状态码与正文片段。
func probeHTTP(endpoint, username, password string, timeout time.Duration) ProbeResult {
	res := ProbeResult{}
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	var sb strings.Builder
	fmt.Fprintf(&sb, "CONNECT %s HTTP/1.1\r\n", probeConnectTarget)
	fmt.Fprintf(&sb, "Host: %s\r\n", probeConnectTarget)
	fmt.Fprintf(&sb, "User-Agent: AntChrome/1.0\r\n")
	fmt.Fprintf(&sb, "Proxy-Connection: Keep-Alive\r\n")
	if username != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		fmt.Fprintf(&sb, "Proxy-Authorization: Basic %s\r\n", cred)
	}
	sb.WriteString("\r\n")
	if _, err := conn.Write([]byte(sb.String())); err != nil {
		res.Error = err.Error()
		return res
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		// 没读到状态行 —— 不是 HTTP 代理（可能是 SOCKS5，已在前一步排除）。
		return res
	}
	statusLine = strings.TrimRight(statusLine, "\r\n")
	if !strings.HasPrefix(statusLine, "HTTP/") {
		// 应答不以 HTTP/ 开头 —— 不是 HTTP 代理。
		return res
	}

	// 确认是 HTTP 代理。
	res.Protocol = "http"
	res.Reachable = true
	res.GatewayMessage = statusLine

	// 解析状态码：HTTP/1.1 <code> <reason>
	if fields := strings.Fields(statusLine); len(fields) >= 2 {
		if code, convErr := strconv.Atoi(fields[1]); convErr == nil {
			res.GatewayStatus = code
		}
	}

	switch {
	case res.GatewayStatus == 200:
		res.Usable = true
	case res.GatewayStatus == 407:
		res.NeedAuth = true
	default:
		// 读取响应头/正文片段，把网关的真实拒绝原因（如 403 china IP is not allow）带出来。
		if snippet := readGatewaySnippet(reader); snippet != "" {
			res.GatewayMessage = statusLine + " — " + snippet
		}
	}
	return res
}

// readGatewaySnippet 读取 CONNECT 失败后网关返回的头部/正文片段（限长，避免阻塞）。
func readGatewaySnippet(reader *bufio.Reader) string {
	var headers []string
	for i := 0; i < 20; i++ {
		line, err := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		headers = append(headers, line)
		if err != nil {
			break
		}
	}
	// 尝试读取少量正文（很多网关把原因写在 body 里）。
	body := make([]byte, 256)
	n, _ := reader.Read(body)
	bodyStr := strings.TrimSpace(string(body[:n]))

	parts := make([]string, 0, 2)
	for _, h := range headers {
		// 只挑可能含原因的头，避免噪音。
		lower := strings.ToLower(h)
		if strings.HasPrefix(lower, "x-") || strings.Contains(lower, "message") || strings.Contains(lower, "error") {
			parts = append(parts, h)
		}
	}
	if bodyStr != "" {
		parts = append(parts, bodyStr)
	}
	return strings.TrimSpace(strings.Join(parts, " | "))
}

// readFull 在已设置 deadline 的连接上读满 buf。
func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// ProbeProxyConfig 探测一段代理配置串的协议。
//   - 已带协议头（socks5:// / http:// / https://）：直接回报该协议，不做网络探测。
//   - 裸格式 host:port[:user:pass] 或 user:pass@host:port：发起握手探测。
//
// 返回的 ProbeResult.Protocol 为空表示无法判定。
func ProbeProxyConfig(proxyConfig string, timeout time.Duration) ProbeResult {
	src := strings.TrimSpace(proxyConfig)
	if src == "" {
		return ProbeResult{Error: "代理配置为空"}
	}
	l := strings.ToLower(src)

	// 已带协议头：信任声明的协议，不重复探测。
	if strings.HasPrefix(l, "socks5://") || strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
		u, err := url.Parse(src)
		if err != nil {
			return ProbeResult{Error: fmt.Sprintf("代理地址解析失败: %v", err)}
		}
		scheme := strings.ToLower(u.Scheme)
		if scheme == "https" {
			scheme = "http"
		}
		return ProbeResult{Protocol: scheme, Reachable: true, Usable: true}
	}

	host, port, username, password, err := parseBareProxy(src)
	if err != nil {
		return ProbeResult{Error: err.Error()}
	}
	return ProbeBareProxy(host, port, username, password, timeout)
}

// parseBareProxy 解析裸格式 host:port[:user:pass] 或 user:pass@host:port。
func parseBareProxy(src string) (host string, port int, username, password string, err error) {
	core := strings.TrimSpace(src)
	// 去掉行尾 #备注
	if idx := strings.Index(core, "#"); idx >= 0 {
		core = strings.TrimSpace(core[:idx])
	}
	if core == "" {
		return "", 0, "", "", fmt.Errorf("代理配置为空")
	}

	if strings.Contains(core, "@") {
		at := strings.LastIndex(core, "@")
		auth := core[:at]
		hostport := core[at+1:]
		authSegs := strings.SplitN(auth, ":", 2)
		username = strings.TrimSpace(authSegs[0])
		if len(authSegs) > 1 {
			password = strings.TrimSpace(authSegs[1])
		}
		host, port = splitHostPort(hostport)
	} else {
		segs := strings.Split(core, ":")
		if len(segs) < 2 {
			return "", 0, "", "", fmt.Errorf("无法解析代理地址（需要 host:port）")
		}
		host = strings.TrimSpace(segs[0])
		fmt.Sscanf(strings.TrimSpace(segs[1]), "%d", &port)
		if len(segs) >= 3 {
			username = strings.TrimSpace(segs[2])
		}
		if len(segs) >= 4 {
			password = strings.TrimSpace(strings.Join(segs[3:], ":"))
		}
	}

	if host == "" || port <= 0 || port > 65535 {
		return "", 0, "", "", fmt.Errorf("无法解析代理地址或端口")
	}
	return host, port, username, password, nil
}
