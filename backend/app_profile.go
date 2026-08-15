package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
)

const maxRemoteProfileConfigBytes = 512 * 1024

// FetchRemoteAuthorProfile 拉取远程作者配置 JSON，供前端与本地默认配置合并。
func (a *App) FetchRemoteAuthorProfile(rawURL string, timeoutMs int) (map[string]interface{}, error) {
	targetURL := strings.TrimSpace(rawURL)
	if targetURL == "" {
		return nil, fmt.Errorf("远程作者配置地址不能为空")
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("远程作者配置地址无效")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("远程作者配置仅支持 HTTP/HTTPS 地址")
	}

	if timeoutMs <= 0 {
		timeoutMs = 3000
	}
	if timeoutMs > 15000 {
		timeoutMs = 15000
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建远程作者配置请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ZwBrowser/1.0 profile-fetch")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log := logger.New("ProfileConfig")
		if isTimeoutError(err) {
			log.Warn("远程作者配置请求超时", logger.F("url", targetURL), logger.F("timeout_ms", timeoutMs))
			return nil, fmt.Errorf("远程作者配置请求超时")
		}
		log.Warn("远程作者配置请求失败", logger.F("url", targetURL), logger.F("error", err))
		return nil, fmt.Errorf("拉取远程作者配置失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("远程作者配置返回异常状态码: %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxRemoteProfileConfigBytes))
	decoder.UseNumber()

	var payload map[string]interface{}
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析远程作者配置失败: %w", err)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("远程作者配置为空")
	}

	return payload, nil
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
