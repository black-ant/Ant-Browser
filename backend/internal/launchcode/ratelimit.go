package launchcode

import (
	"net"
	"sync"
	"time"
)

// rateLimiter 基于令牌桶算法的简单速率限制器。
// 用于防止 CDP 代理等端点被本地恶意进程 DoS。
type rateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rate     int           // 每秒允许的请求数
	burst    int           // 突发容量（桶大小）
	cleanupT *time.Ticker  // 清理过期桶的定时器
	stopCh   chan struct{} // 停止清理协程的信号
}

type tokenBucket struct {
	tokens   int       // 当前令牌数
	lastFill time.Time // 上次填充时间
}

// newRateLimiter 创建速率限制器。
// rate: 每秒允许的请求数，burst: 突发容量。
func newRateLimiter(rate, burst int) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    rate,
		burst:   burst,
		stopCh:  make(chan struct{}),
	}
	// 每分钟清理一次超过 5 分钟未使用的桶，避免内存泄漏
	rl.cleanupT = time.NewTicker(1 * time.Minute)
	go rl.cleanupLoop()
	return rl
}

// allow 判断指定 IP 是否允许通过（消耗 1 个令牌）。
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[ip]
	if !exists {
		bucket = &tokenBucket{
			tokens:   rl.burst,
			lastFill: time.Now(),
		}
		rl.buckets[ip] = bucket
	}

	// 根据时间流逝填充令牌
	now := time.Now()
	elapsed := now.Sub(bucket.lastFill)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.rate))
	if tokensToAdd > 0 {
		bucket.tokens += tokensToAdd
		if bucket.tokens > rl.burst {
			bucket.tokens = rl.burst
		}
		bucket.lastFill = now
	}

	// 尝试消耗 1 个令牌
	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}
	return false
}

// cleanupLoop 定期清理长时间未使用的桶。
func (rl *rateLimiter) cleanupLoop() {
	for {
		select {
		case <-rl.cleanupT.C:
			rl.cleanup()
		case <-rl.stopCh:
			rl.cleanupT.Stop()
			return
		}
	}
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	threshold := time.Now().Add(-5 * time.Minute)
	for ip, bucket := range rl.buckets {
		if bucket.lastFill.Before(threshold) {
			delete(rl.buckets, ip)
		}
	}
}

// stop 停止清理协程。
func (rl *rateLimiter) stop() {
	close(rl.stopCh)
}

// extractIP 从 RemoteAddr 提取 IP 地址（去除端口）。
func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr // 解析失败时直接返回原值
	}
	return host
}
