package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
)

type resolvedNodeRuntime struct {
	Source             string
	Path               string
	Version            string
	SystemNodeDetected bool
	SystemNodePath     string
	Resolution         string
	SystemNodeError    string
}

type nodeProbeResult struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type SystemNodeProbeResult struct {
	OK      bool   `json:"ok"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

func (m *Manager) resolveNodeRuntime(runtimeDir string, auto config.AutomationConfig) resolvedNodeRuntime {
	mode := config.DefaultAutomationNodeSource
	if auto.NodeSource != "" {
		mode = strings.TrimSpace(auto.NodeSource)
	}

	if mode != config.AutomationNodeSourceBundled {
		if systemNode, err := m.resolveSystemNode(context.Background(), auto.SystemNodePath); err == nil {
			return systemNode
		} else if mode == config.AutomationNodeSourceSystem {
			return resolvedNodeRuntime{
				Source:          config.AutomationNodeSourceSystem,
				Version:         strings.TrimSpace(auto.NodeVersion),
				SystemNodePath:  strings.TrimSpace(auto.SystemNodePath),
				Resolution:      "已配置为 system，必须使用系统 Node",
				SystemNodeError: err.Error(),
			}
		} else {
			return resolvedNodeRuntime{
				Source:          config.AutomationNodeSourceBundled,
				Path:            m.nodeExecutablePath(runtimeDir),
				Version:         strings.TrimSpace(auto.NodeVersion),
				SystemNodePath:  strings.TrimSpace(auto.SystemNodePath),
				Resolution:      "系统 Node 不可用，已回退到内建 Node",
				SystemNodeError: err.Error(),
			}
		}
	}

	return resolvedNodeRuntime{
		Source:     config.AutomationNodeSourceBundled,
		Path:       m.nodeExecutablePath(runtimeDir),
		Version:    strings.TrimSpace(auto.NodeVersion),
		Resolution: "已配置为 bundled，始终使用内建 Node",
	}
}

func (m *Manager) resolveSystemNode(ctx context.Context, explicitPath string) (resolvedNodeRuntime, error) {
	type nodeCandidate struct {
		path       string
		resolution string
	}

	candidatePaths := make([]nodeCandidate, 0, 2)
	if trimmed := strings.TrimSpace(explicitPath); trimmed != "" {
		candidatePaths = append(candidatePaths, nodeCandidate{
			path:       trimmed,
			resolution: "已使用配置的系统 Node 路径",
		})
	}
	if lookupPath, err := exec.LookPath("node"); err == nil && strings.TrimSpace(lookupPath) != "" {
		lookupPath = strings.TrimSpace(lookupPath)
		duplicate := false
		for _, existing := range candidatePaths {
			if strings.EqualFold(existing.path, lookupPath) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			candidatePaths = append(candidatePaths, nodeCandidate{
				path:       lookupPath,
				resolution: "已使用 PATH 中的系统 Node",
			})
		}
	}

	var lastErr error
	for _, candidate := range candidatePaths {
		probe, err := m.probeNodeExecutable(ctx, candidate.path)
		if err != nil {
			lastErr = err
			continue
		}
		return resolvedNodeRuntime{
			Source:             config.AutomationNodeSourceSystem,
			Path:               probe.Path,
			Version:            probe.Version,
			SystemNodeDetected: true,
			SystemNodePath:     probe.Path,
			Resolution:         candidate.resolution,
		}, nil
	}

	if lastErr != nil {
		return resolvedNodeRuntime{}, lastErr
	}
	return resolvedNodeRuntime{}, fmt.Errorf("未找到系统 Node")
}

func (m *Manager) probeNodeExecutable(ctx context.Context, nodePath string) (nodeProbeResult, error) {
	nodePath = strings.TrimSpace(nodePath)
	if nodePath == "" {
		return nodeProbeResult{}, fmt.Errorf("Node 路径为空")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	script := `process.stdout.write(JSON.stringify({path: process.execPath, version: process.versions.node}));`
	cmd := exec.CommandContext(probeCtx, nodePath, "-e", script)
	hideWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return nodeProbeResult{}, fmt.Errorf("检测 Node 可执行文件失败（%s）: %s", nodePath, message)
	}

	var probe nodeProbeResult
	if err := json.Unmarshal(output, &probe); err != nil {
		return nodeProbeResult{}, fmt.Errorf("解析 Node 探测结果失败: %w", err)
	}
	probe.Path = strings.TrimSpace(probe.Path)
	probe.Version = strings.TrimSpace(probe.Version)
	if probe.Path == "" {
		probe.Path = nodePath
	}
	if probe.Version == "" {
		return nodeProbeResult{}, fmt.Errorf("Node 版本为空")
	}
	if absPath, err := filepath.Abs(probe.Path); err == nil {
		probe.Path = absPath
	}
	return probe, nil
}

func (m *Manager) ProbeSystemNode(ctx context.Context, explicitPath string) (SystemNodeProbeResult, error) {
	resolved, err := m.resolveSystemNode(ctx, explicitPath)
	if err != nil {
		return SystemNodeProbeResult{}, err
	}
	return SystemNodeProbeResult{
		OK:      true,
		Path:    strings.TrimSpace(resolved.Path),
		Version: strings.TrimSpace(resolved.Version),
	}, nil
}

func (m *Manager) verifyNodeWithPlaywright(ctx context.Context, nodePath, runtimeDir string) (RuntimeCheckResult, error) {
	nodePath = strings.TrimSpace(nodePath)
	runtimeDir = strings.TrimSpace(runtimeDir)
	if nodePath == "" {
		return RuntimeCheckResult{}, fmt.Errorf("node path is empty")
	}
	if runtimeDir == "" {
		return RuntimeCheckResult{}, fmt.Errorf("runtime dir is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	script := `
const path = require('path');
const pkg = require(path.join(process.argv[1], 'node_modules', 'playwright-core', 'package.json'));
const playwright = require(path.join(process.argv[1], 'node_modules', 'playwright-core'));
process.stdout.write(JSON.stringify({
  nodeVersion: process.versions.node,
  playwrightVersion: pkg.version,
  hasChromium: !!playwright.chromium
}));
`

	cmd := exec.CommandContext(checkCtx, nodePath, "-e", script, runtimeDir)
	cmd.Dir = runtimeDir
	hideWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return RuntimeCheckResult{}, fmt.Errorf("playwright probe failed: %s", message)
	}

	var payload struct {
		NodeVersion       string `json:"nodeVersion"`
		PlaywrightVersion string `json:"playwrightVersion"`
		HasChromium       bool   `json:"hasChromium"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return RuntimeCheckResult{}, fmt.Errorf("parse playwright probe result failed: %w", err)
	}

	result := RuntimeCheckResult{
		OK:                strings.TrimSpace(payload.NodeVersion) != "" && strings.TrimSpace(payload.PlaywrightVersion) != "" && payload.HasChromium,
		NodeVersion:       strings.TrimSpace(payload.NodeVersion),
		PlaywrightVersion: strings.TrimSpace(payload.PlaywrightVersion),
	}
	if !result.OK {
		return RuntimeCheckResult{}, fmt.Errorf("playwright probe returned incomplete result")
	}
	return result, nil
}
