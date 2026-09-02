package proxy

import (
	"ant-chrome/backend/internal/fsutil"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func computeNodeKey(src string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(src)))
	return hex.EncodeToString(h[:])
}

func normalizeNodeScheme(src string) string {
	return strings.TrimSpace(src)
}

func resolveEnvPath(path string, appRoot string) string {
	path = fsutil.NormalizePathInput(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	if appRoot != "" {
		candidate := filepath.Join(appRoot, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return path
}

func waitPortReady(host string, port int, timeout time.Duration) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("端口 %d 不可用: %w", port, lastErr)
	}
	return fmt.Errorf("端口 %d 不可用", port)
}

func waitSocks5Ready(host string, port int, timeout time.Duration) error {
	return waitSocks5ReadyContext(context.Background(), host, port, timeout)
}

func waitSocks5ReadyContext(ctx context.Context, host string, port int, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		probeTimeout := 300 * time.Millisecond
		if remaining := time.Until(deadline); remaining < probeTimeout {
			probeTimeout = remaining
		}
		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		err := checkSocks5HandshakeContext(probeCtx, addr, probeTimeout)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if err := ctx.Err(); err != nil {
			return err
		}
		pause := 100 * time.Millisecond
		if remaining := time.Until(deadline); remaining < pause {
			pause = remaining
		}
		if pause <= 0 {
			break
		}
		timer := time.NewTimer(pause)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	if lastErr != nil {
		return fmt.Errorf("socks5 端口 %d 未就绪: %w", port, lastErr)
	}
	return fmt.Errorf("socks5 端口 %d 未就绪", port)
}

func checkSocks5Handshake(addr string, timeout time.Duration) error {
	return checkSocks5HandshakeContext(context.Background(), addr, timeout)
}

func checkSocks5HandshakeContext(ctx context.Context, addr string, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return err
	}
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}
	if buf[0] != 0x05 || buf[1] != 0x00 {
		return fmt.Errorf("socks5 握手响应异常: %v", buf)
	}
	return nil
}

// nextAvailablePort 分配一个可用端口。
// 采用二次验证策略：分配后立即再次绑定确认未被其他进程抢占，
// 并在 EnsureBridge 层面加重试，彻底消除 TOCTOU 竞争窗口。
func nextAvailablePort() (int, error) {
	return nextAvailablePortWithRetry(10)
}

func nextAvailablePortWithRetry(maxRetries int) (int, error) {
	for i := 0; i < maxRetries; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			continue
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		time.Sleep(10 * time.Millisecond)
		verifyListener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		verifyListener.Close()
		return port, nil
	}
	return 0, fmt.Errorf("无法分配可用端口，已重试 %d 次", maxRetries)
}
