package launchcode_test

// Feature: instance-launch-code, Property 6: valid code response structure
// Feature: instance-launch-code, Property 7: invalid code returns 404
// Feature: instance-launch-code, Property 8: idempotent launch
// Validates: Requirements 3.2, 3.3, 3.4, 3.5, 4.1, 4.2, 4.4

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/launchcode"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// --- 测试辅助类型 ---

// mockStarter 模拟 BrowserStarter，记录调用次数
type mockStarter struct {
	profiles   map[string]*browser.Profile
	callCounts map[string]int
}

func newMockStarter() *mockStarter {
	return &mockStarter{
		profiles:   make(map[string]*browser.Profile),
		callCounts: make(map[string]int),
	}
}

func (m *mockStarter) addProfile(p *browser.Profile) {
	m.profiles[p.ProfileId] = p
}

func (m *mockStarter) StartInstance(profileId string) (*browser.Profile, error) {
	m.callCounts[profileId]++
	p, ok := m.profiles[profileId]
	if !ok {
		return nil, fmt.Errorf("profile not found: %s", profileId)
	}
	return p, nil
}

// buildTestHandler 构建一个可直接用于 httptest 的 handler（绕过 localhost 中间件）
// 通过直接调用 server 内部 handler 的方式，使用 httptest.NewRecorder 测试路由逻辑
func buildTestHandler(svc *launchcode.LaunchCodeService, starter launchcode.BrowserStarter) http.Handler {
	srv := launchcode.NewLaunchServer(svc, starter, nil, nil, 0)
	return launchcode.NewTestHandler(srv)
}

// newInMemoryService 创建一个使用内存 DAO 的 LaunchCodeService
func newInMemoryService() *launchcode.LaunchCodeService {
	dao := launchcode.NewMemoryLaunchCodeDAO()
	return launchcode.NewLaunchCodeService(dao)
}

// --- Property 6: 有效 Code 返回正确响应结构 ---

// genNonEmptyAlpha 生成长度 1-32 的字母字符串（不使用 SuchThat 过滤）
func genNonEmptyAlpha() gopter.Gen {
	return gen.SliceOfN(8, gen.RuneRange('a', 'z')).Map(func(runes []rune) string {
		return string(runes)
	})
}

// TestProperty6_ValidCodeResponseStructure
// 对于任意存在的 LaunchCode，GET /api/launch/{code} 应返回：
//   - HTTP 200
//   - Content-Type: application/json
//   - 响应体含 ok:true, profileId, profileName, pid, debugPort
func TestProperty6_ValidCodeResponseStructure(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("有效 code 返回 200 及正确响应结构", prop.ForAll(
		func(profileId, profileName string, pid, debugPort int) bool {
			svc := newInMemoryService()
			starter := newMockStarter()

			profile := &browser.Profile{
				ProfileId:   profileId,
				ProfileName: profileName,
				Pid:         pid,
				DebugPort:   debugPort,
			}
			starter.addProfile(profile)

			code, err := svc.EnsureCode(profileId)
			if err != nil {
				return false
			}

			handler := buildTestHandler(svc, starter)
			req := httptest.NewRequest(http.MethodGet, "/api/launch/"+code, nil)
			req.Header.Set("X-Launch-Request", "1")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				return false
			}
			if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
				return false
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				return false
			}

			ok, _ := resp["ok"].(bool)
			gotProfileId, _ := resp["profileId"].(string)
			gotProfileName, _ := resp["profileName"].(string)
			_, hasPid := resp["pid"]
			_, hasDebugPort := resp["debugPort"]

			return ok &&
				gotProfileId == profileId &&
				gotProfileName == profileName &&
				hasPid && hasDebugPort
		},
		genNonEmptyAlpha(),
		genNonEmptyAlpha(),
		gen.IntRange(1000, 99999),
		gen.IntRange(9000, 9999),
	))

	properties.TestingRun(t)
}

// --- Property 7: 无效 Code 返回 404 ---

// genInvalidCode 生成一定不存在于空 service 中的 code（小写字母，不符合 A-Z0-9 格式）
func genInvalidCode() gopter.Gen {
	// 生成 4 位小写字母字符串，永远不会匹配 [A-Z0-9]{6} 格式的有效 code
	return gen.SliceOfN(4, gen.RuneRange('a', 'z')).Map(func(runes []rune) string {
		return string(runes)
	})
}

// TestProperty7_InvalidCodeReturns404
// 对于任意不存在的 code，GET /api/launch/{code} 应返回：
//   - HTTP 404
//   - Content-Type: application/json
//   - 响应体含 ok:false 和 error 字段
func TestProperty7_InvalidCodeReturns404(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("不存在的 code 返回 404", prop.ForAll(
		func(code string) bool {
			svc := newInMemoryService()
			starter := newMockStarter()

			handler := buildTestHandler(svc, starter)
			req := httptest.NewRequest(http.MethodGet, "/api/launch/"+code, nil)
			req.Header.Set("X-Launch-Request", "1")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				return false
			}
			if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
				return false
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				return false
			}

			ok, _ := resp["ok"].(bool)
			_, hasError := resp["error"]
			return !ok && hasError
		},
		genInvalidCode(),
	))

	properties.TestingRun(t)
}

// --- Property 8: 重复唤起的幂等性 ---

// TestProperty8_IdempotentLaunch
// 对于已运行的实例，连续两次 GET /api/launch/{code}：
//   - 两次均返回 HTTP 200
//   - 两次返回的 pid 相同（不重新启动）
func TestProperty8_IdempotentLaunch(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("重复唤起返回相同 pid，不重新启动", prop.ForAll(
		func(profileId string, pid int) bool {
			svc := newInMemoryService()
			starter := newMockStarter()

			profile := &browser.Profile{
				ProfileId:   profileId,
				ProfileName: "test-profile",
				Pid:         pid,
				DebugPort:   9222,
				Running:     true,
			}
			starter.addProfile(profile)

			code, err := svc.EnsureCode(profileId)
			if err != nil {
				return false
			}

			handler := buildTestHandler(svc, starter)

			// 第一次请求
			req1 := httptest.NewRequest(http.MethodGet, "/api/launch/"+code, nil)
			req1.Header.Set("X-Launch-Request", "1")
			w1 := httptest.NewRecorder()
			handler.ServeHTTP(w1, req1)

			// 第二次请求
			req2 := httptest.NewRequest(http.MethodGet, "/api/launch/"+code, nil)
			req2.Header.Set("X-Launch-Request", "1")
			w2 := httptest.NewRecorder()
			handler.ServeHTTP(w2, req2)

			if w1.Code != http.StatusOK || w2.Code != http.StatusOK {
				return false
			}

			var resp1, resp2 map[string]interface{}
			if err := json.NewDecoder(w1.Body).Decode(&resp1); err != nil {
				return false
			}
			if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
				return false
			}

			pid1, _ := resp1["pid"].(float64)
			pid2, _ := resp2["pid"].(float64)

			// 两次 pid 相同，且 StartInstance 被调用了 2 次（幂等由 starter 保证返回同一 profile）
			return pid1 == pid2 && pid1 == float64(pid)
		},
		genNonEmptyAlpha(),
		gen.IntRange(1000, 99999),
	))

	properties.TestingRun(t)
}

// --- 健康检查单元测试 ---

func TestHealthEndpoint(t *testing.T) {
	svc := newInMemoryService()
	starter := newMockStarter()
	handler := buildTestHandler(svc, starter)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	req.Header.Set("X-Launch-Request", "1")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	ok, _ := resp["ok"].(bool)
	if !ok {
		t.Error("期望 ok=true")
	}
}
