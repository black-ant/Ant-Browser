package proxy

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

// LocalProxyCandidate 扫描到的一个本地代理。
type LocalProxyCandidate struct {
	Addr     string `json:"addr"`     // 规范化后的代理地址，如 socks5://127.0.0.1:7891
	Protocol string `json:"protocol"` // "socks5" | "http"
	Port     int    `json:"port"`
}

// LocalProxyScanResult 本地代理扫描结果。
type LocalProxyScanResult struct {
	Found      bool                  `json:"found"`
	Best       string                `json:"best"`       // 推荐地址（第一个命中的，按端口优先级）
	Candidates []LocalProxyCandidate `json:"candidates"` // 所有命中的本地代理
	Error      string                `json:"error"`
}

// commonLocalProxyPorts 常见本地代理软件的监听端口，按命中优先级排序。
// 排在前面的更可能是用户真正在用的混合/代理端口。
var commonLocalProxyPorts = []int{
	7890, // Clash 混合端口
	7897, // Clash Verge 混合端口
	7891, // Clash SOCKS5
	1080, // 通用 SOCKS5
	10808, // v2rayN SOCKS5
	10809, // v2rayN HTTP
	2080,  // NekoBox / sing-box
	8889,  // 部分客户端 HTTP
	1087,  // Surge/部分客户端 HTTP
	8118,  // Privoxy HTTP
}

// ScanLocalProxy 并发探测 127.0.0.1 上常见端口，返回活着的本地代理。
// 复用 probeSocks5 / probeHTTP 的字节级握手判定协议，因此能区分 SOCKS5 与 HTTP。
func ScanLocalProxy() LocalProxyScanResult {
	const (
		dialTimeout  = 400 * time.Millisecond
		probeTimeout = 1200 * time.Millisecond
	)

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		hits []LocalProxyCandidate
	)

	for _, port := range commonLocalProxyPorts {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			endpoint := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

			// 先快速 TCP 探测端口是否开放，避免对大量关闭端口做完整握手。
			conn, err := net.DialTimeout("tcp", endpoint, dialTimeout)
			if err != nil {
				return
			}
			conn.Close()

			// SOCKS5 字节握手（确定性最高）。
			if socks := probeSocks5(endpoint, "", "", probeTimeout); socks.Protocol == "socks5" {
				mu.Lock()
				hits = append(hits, LocalProxyCandidate{
					Addr:     fmt.Sprintf("socks5://127.0.0.1:%d", port),
					Protocol: "socks5",
					Port:     port,
				})
				mu.Unlock()
				return
			}
			// 再试 HTTP CONNECT。
			if httpRes := probeHTTP(endpoint, "", "", probeTimeout); httpRes.Protocol == "http" {
				mu.Lock()
				hits = append(hits, LocalProxyCandidate{
					Addr:     fmt.Sprintf("http://127.0.0.1:%d", port),
					Protocol: "http",
					Port:     port,
				})
				mu.Unlock()
			}
		}(port)
	}
	wg.Wait()

	if len(hits) == 0 {
		return LocalProxyScanResult{Found: false, Error: "未在本机常见端口检测到可用的 HTTP / SOCKS5 代理"}
	}

	// 按 commonLocalProxyPorts 的优先级排序，取第一个作为推荐。
	priority := make(map[int]int, len(commonLocalProxyPorts))
	for i, p := range commonLocalProxyPorts {
		priority[p] = i
	}
	sort.SliceStable(hits, func(i, j int) bool {
		return priority[hits[i].Port] < priority[hits[j].Port]
	})

	return LocalProxyScanResult{
		Found:      true,
		Best:       hits[0].Addr,
		Candidates: hits,
	}
}
