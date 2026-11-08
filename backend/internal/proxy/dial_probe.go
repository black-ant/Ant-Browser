package proxy

import (
	"context"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 服务器 IP 优选：订阅域名常解析成多条 A 记录（BGP 轮询），
// 其中部分 IP 可能已失联；内核拿到死 IP 后只能干等超时，
// 表现为整批节点"超时"而订阅在别的客户端可用。
// 因此在生成内核配置前先对解析结果做并发 TCP 拨号探活，
// 把可用 IP 按拨号延迟升序交给内核，内核自身会轮换这些入口。

const (
	dialProbeResolveTimeout = 3 * time.Second
	dialProbePerIPTimeout   = 2 * time.Second
	dialProbeMaxResults     = 4
)

type dialProbeResult struct {
	ip      string
	latency time.Duration
}

// pickReachableServerIPs 解析 host 并对每个 IP 并发 TCP 拨号，
// 返回按拨号延迟升序的可用 IP（最多 dialProbeMaxResults 个）。
// host 本身是 IP、解析失败或全部不可达时返回 nil，由调用方回退到原域名。
func pickReachableServerIPs(ctx context.Context, host string, port int) []string {
	host = strings.TrimSpace(host)
	if host == "" || port <= 0 || net.ParseIP(host) != nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	resolveCtx, cancelResolve := context.WithTimeout(ctx, dialProbeResolveTimeout)
	defer cancelResolve()
	ips, err := net.DefaultResolver.LookupHost(resolveCtx, host)
	if err != nil || len(ips) == 0 {
		return nil
	}

	results := make(chan dialProbeResult, len(ips))
	var wg sync.WaitGroup
	for _, item := range ips {
		ip := strings.TrimSpace(item)
		if ip == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			dialCtx, cancelDial := context.WithTimeout(ctx, dialProbePerIPTimeout)
			defer cancelDial()
			conn, dialErr := (&net.Dialer{}).DialContext(dialCtx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
			if dialErr != nil {
				return
			}
			_ = conn.Close()
			results <- dialProbeResult{ip: ip, latency: time.Since(start)}
		}()
	}
	wg.Wait()
	close(results)

	probes := make([]dialProbeResult, 0, len(results))
	for result := range results {
		probes = append(probes, result)
	}
	if len(probes) == 0 {
		return nil
	}
	sort.Slice(probes, func(i, j int) bool { return probes[i].latency < probes[j].latency })
	if len(probes) > dialProbeMaxResults {
		probes = probes[:dialProbeMaxResults]
	}
	out := make([]string, 0, len(probes))
	for _, probe := range probes {
		out = append(out, probe.ip)
	}
	return out
}

// parsePortRangeStart 从 "2000-2022" 形态中取起始端口，仅用于探活选路。
func parsePortRangeStart(portRange string) int {
	portRange = strings.TrimSpace(portRange)
	if portRange == "" {
		return 0
	}
	head := portRange
	if idx := strings.IndexAny(portRange, "-~"); idx > 0 {
		head = portRange[:idx]
	}
	port, err := strconv.Atoi(strings.TrimSpace(head))
	if err != nil || port <= 0 {
		return 0
	}
	return port
}
