package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	xproxy "golang.org/x/net/proxy"
)

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func proxyCoreHTTPClient(timeout time.Duration, proxyConfig string) (*http.Client, string, error) {
	proxyConfig = strings.TrimSpace(proxyConfig)
	if proxyConfig == "" || strings.EqualFold(proxyConfig, "direct://") {
		return &http.Client{Timeout: timeout, Transport: proxyCoreDirectTransport()}, "直连", nil
	}
	u, err := url.Parse(proxyConfig)
	if err != nil {
		return nil, "", fmt.Errorf("代理地址解析失败: %w", err)
	}
	if isBadLocalHTTPSProxy(u) {
		return nil, "", fmt.Errorf("下载代理不能填 %s，127.0.0.1:443 通常不是本机代理端口；请改成真实代理端口，如 socks5://127.0.0.1:7890，或留空直连", u.Host)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "https":
		return &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: http.ProxyURL(u)}}, "指定代理", nil
	case "socks5":
		var auth *xproxy.Auth
		if u.User != nil {
			password, _ := u.User.Password()
			auth = &xproxy.Auth{User: u.User.Username(), Password: password}
		}
		dialer, err := xproxy.SOCKS5("tcp", u.Host, auth, xproxy.Direct)
		if err != nil {
			return nil, "", fmt.Errorf("SOCKS5 dialer 创建失败: %w", err)
		}
		contextDialer, ok := dialer.(xproxy.ContextDialer)
		if !ok {
			return nil, "", fmt.Errorf("SOCKS5 dialer 不支持 ContextDialer")
		}
		return &http.Client{Timeout: timeout, Transport: &http.Transport{DialContext: contextDialer.DialContext}}, "指定代理", nil
	default:
		return nil, "", fmt.Errorf("仅支持 http://、https://、socks5:// 或 direct://")
	}
}

func proxyCoreDirectTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err == nil && port == "443" && isLocalhostHost(host) {
				return nil, fmt.Errorf("直连下载被解析到 %s：这通常是本机 hosts/DNS 污染或仍在运行旧版本。请重启应用；如果仍出现，请检查 hosts/DNS，或在下载代理中填写真实代理端口", address)
			}
			return dialer.DialContext(ctx, network, address)
		},
	}
}

func isBadLocalHTTPSProxy(u *url.URL) bool {
	if u == nil {
		return false
	}
	return isLocalhostHost(u.Hostname()) && u.Port() == "443"
}

func isLocalhostHost(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func fetchGitHubRelease(ctx context.Context, client *http.Client, repo string, version string) (githubRelease, error) {
	apiURL := "https://api.github.com/repos/" + repo + "/releases/latest"
	if !strings.EqualFold(strings.TrimSpace(version), "latest") {
		apiURL = "https://api.github.com/repos/" + repo + "/releases/tags/" + strings.TrimSpace(version)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ant-chrome-proxy-core-downloader")
	resp, err := client.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return githubRelease{}, fmt.Errorf("GitHub API HTTP %d", resp.StatusCode)
	}
	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return githubRelease{}, err
	}
	if len(release.Assets) == 0 {
		return githubRelease{}, fmt.Errorf("Release 没有可下载资产")
	}
	return release, nil
}

func selectProxyCoreAsset(spec proxyCoreSpec, assets []githubReleaseAsset, goos string, goarch string) (githubReleaseAsset, error) {
	osTokens := map[string][]string{
		"windows": {"windows", "win"},
		"linux":   {"linux"},
		"darwin":  {"darwin", "macos"},
	}
	archTokens := map[string][]string{
		"amd64": {"amd64", "x86_64", "64"},
		"arm64": {"arm64", "aarch64"},
		"386":   {"386", "i386", "x86"},
	}
	extTokens := []string{".zip", ".tar.gz", ".tgz"}
	if spec.Core == "mihomo" {
		extTokens = append(extTokens, ".gz")
	}
	if goos == "windows" {
		extTokens = []string{".zip"}
	}
	badTokens := []string{"sha", "checksum", "dgst", ".sig", ".asc", "source", "geoip", "geosite"}
	candidates := make([]githubReleaseAsset, 0)
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if !hasAnySuffix(name, extTokens) || containsAny(name, badTokens) {
			continue
		}
		if !containsAny(name, osTokens[goos]) || !matchesProxyAssetArch(name, goarch, archTokens[goarch]) {
			continue
		}
		if spec.Core == "mihomo" && !strings.Contains(name, "compatible") {
			continue
		}
		candidates = append(candidates, asset)
	}
	if len(candidates) == 0 && spec.Core == "mihomo" {
		fallbackSpec := spec
		fallbackSpec.Core = "mihomo-fallback"
		for _, asset := range assets {
			name := strings.ToLower(asset.Name)
			if hasAnySuffix(name, extTokens) && !containsAny(name, badTokens) && containsAny(name, osTokens[goos]) && matchesProxyAssetArch(name, goarch, archTokens[goarch]) {
				candidates = append(candidates, asset)
			}
		}
	}
	if len(candidates) == 0 {
		return githubReleaseAsset{}, fmt.Errorf("官方 Release 未找到适配 %s/%s 的 %s 资产", goos, goarch, spec.DisplayName)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		ai := assetScore(spec, candidates[i].Name)
		aj := assetScore(spec, candidates[j].Name)
		if ai != aj {
			return ai > aj
		}
		return candidates[i].Name < candidates[j].Name
	})
	return candidates[0], nil
}

func proxyCoreBinaryName(binaryBase string, targetOS string) string {
	if targetOS == "windows" {
		return binaryBase + ".exe"
	}
	return binaryBase
}

func matchesProxyAssetArch(name string, goarch string, tokens []string) bool {
	if goarch == "amd64" && strings.Contains(name, "arm64") {
		return false
	}
	if goarch == "386" && (strings.Contains(name, "amd64") || strings.Contains(name, "arm64")) {
		return false
	}
	return containsAny(name, tokens)
}

func assetScore(spec proxyCoreSpec, name string) int {
	lower := strings.ToLower(name)
	score := 0
	if strings.HasSuffix(lower, ".zip") {
		score += 3
	}
	if strings.Contains(lower, "compatible") {
		score += 5
	}
	if strings.Contains(lower, spec.BinaryBase) || strings.Contains(lower, spec.Core) {
		score += 2
	}
	if !strings.Contains(lower, "glibc") && !strings.Contains(lower, "musl") && !strings.Contains(lower, "softfloat") && !strings.Contains(lower, "legacy") {
		score += 2
	}
	return score
}

func downloadProxyCoreAsset(ctx context.Context, client *http.Client, url string, file *os.File, totalSize int64, send func(string, int, string)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ant-chrome-proxy-core-downloader")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if totalSize <= 0 {
		totalSize = resp.ContentLength
	}
	buf := make([]byte, 1024*1024)
	var downloaded int64
	lastTick := time.Now()
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := file.Write(buf[:n]); err != nil {
				return err
			}
			downloaded += int64(n)
			if totalSize > 0 && time.Since(lastTick) > 500*time.Millisecond {
				progress := 5 + int(float64(downloaded)/float64(totalSize)*70)
				if progress > 75 {
					progress = 75
				}
				send("downloading", progress, fmt.Sprintf("下载中 %.1f MB / %.1f MB", float64(downloaded)/1024/1024, float64(totalSize)/1024/1024))
				lastTick = time.Now()
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}
