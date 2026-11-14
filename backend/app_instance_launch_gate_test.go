package backend

import (
	"ant-chrome/backend/internal/browser"
	"errors"
	"fmt"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

// 安全控制：实例并发启动闸门——超限排队、释放放行、重复请求不占名额、不限制模式。
func TestInstanceLaunchGateLimitsAndQueues(t *testing.T) {
	app := &App{config: &config.Config{}}
	app.config.Browser.MaxConcurrentInstances = 2

	type acquireResult struct {
		release func()
		queued  bool
	}

	release1, queued1 := app.acquireInstanceLaunchSlot("p1")
	if queued1 {
		t.Fatal("名额充足时 p1 不应排队")
	}
	defer release1()
	release2, queued2 := app.acquireInstanceLaunchSlot("p2")
	if queued2 {
		t.Fatal("名额充足时 p2 不应排队")
	}

	granted := make(chan acquireResult, 1)
	go func() {
		release, queued := app.acquireInstanceLaunchSlot("p3")
		granted <- acquireResult{release: release, queued: queued}
	}()
	select {
	case <-granted:
		t.Fatal("超出上限时 p3 不应立即获准")
	case <-time.After(600 * time.Millisecond):
		// 预期：p3 排队等待
	}

	// 同一实例的重复启动请求：不重复占名额、不排队（启动逻辑内部会按"已在运行"处理）
	dupRelease, dupQueued := app.acquireInstanceLaunchSlot("p1")
	if dupQueued {
		t.Fatal("重复请求不应排队")
	}
	dupRelease()

	// 释放一个名额 → 排队中的 p3 自动获准
	release2()
	select {
	case result := <-granted:
		if !result.queued {
			t.Fatal("p3 应经历排队")
		}
		result.release()
	case <-time.After(2 * time.Second):
		t.Fatal("释放名额后 p3 未能获准")
	}

	// 全部释放后可再次获准
	app.releaseInstanceLaunchSlot("p1")
	release4, queued4 := app.acquireInstanceLaunchSlot("p4")
	if queued4 {
		t.Fatal("名额空闲时 p4 不应排队")
	}
	release4()

	// limit<=0 = 不限制
	app.config.Browser.MaxConcurrentInstances = 0
	for i := 0; i < 5; i++ {
		release, queued := app.acquireInstanceLaunchSlot(fmt.Sprintf("x%d", i))
		if queued {
			t.Fatal("不限制模式不应排队")
		}
		defer release()
	}
}

// 负数上限与重复释放的安全兜底。
func TestInstanceLaunchGateGuards(t *testing.T) {
	app := &App{config: &config.Config{}}
	app.config.Browser.MaxConcurrentInstances = -3

	release, queued := app.acquireInstanceLaunchSlot("p1")
	if queued {
		t.Fatal("不限制模式不应排队")
	}
	release()
	release() // 重复释放不应panic/产生负占用

	app.config.Browser.MaxConcurrentInstances = 1
	releaseA, _ := app.acquireInstanceLaunchSlot("pa")
	releaseA()
	releaseA() // 重复释放后应仍可立即获准
	releaseB, queuedB := app.acquireInstanceLaunchSlot("pb")
	if queuedB {
		t.Fatal("重复释放后名额应正常回收")
	}
	releaseB()
}

func TestInstanceLaunchGateAppliesUpdatedLimitToQueuedRequest(t *testing.T) {
	app := &App{config: &config.Config{}}
	app.config.Browser.MaxConcurrentInstances = 1

	releaseFirst, _ := app.acquireInstanceLaunchSlot("first")
	defer releaseFirst()

	type acquireResult struct {
		release func()
		queued  bool
	}
	granted := make(chan acquireResult, 1)
	go func() {
		release, queued := app.acquireInstanceLaunchSlot("second")
		granted <- acquireResult{release: release, queued: queued}
	}()

	select {
	case <-granted:
		t.Fatal("second instance should queue at the original limit")
	case <-time.After(100 * time.Millisecond):
	}

	app.config.Browser.MaxConcurrentInstances = 2
	select {
	case result := <-granted:
		if !result.queued {
			t.Fatal("second instance should report queued before the limit changed")
		}
		result.release()
	case <-time.After(time.Second):
		t.Fatal("queued instance was not released after raising the limit")
	}
}

func TestQueuedInstanceStartRejectedAfterRuntimeStops(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.config.Browser.MaxConcurrentInstances = 1
	app.browserMgr = browser.NewManager(app.config, t.TempDir())

	releaseRunning, queued := app.acquireInstanceLaunchSlot("running")
	if queued {
		t.Fatal("首个实例启动不应排队")
	}

	type startResult struct {
		profile *BrowserProfile
		err     error
	}
	started := make(chan startResult, 1)
	go func() {
		profile, err := app.browserInstanceStartInternal("queued", nil, nil, false, false, false, "", "")
		started <- startResult{profile: profile, err: err}
	}()

	gate := app.launchGate()
	deadline := time.Now().Add(time.Second)
	for {
		gate.mu.Lock()
		waiting := gate.waiting
		gate.mu.Unlock()
		if waiting == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("排队启动请求未进入闸门等待")
		}
		time.Sleep(10 * time.Millisecond)
	}

	app.stopRuntimeEvents()
	releaseRunning()

	select {
	case result := <-started:
		if result.profile != nil {
			t.Fatalf("关闭期间排队启动不应返回实例: %#v", result.profile)
		}
		if !errors.Is(result.err, errBrowserStartRuntimeStopped) {
			t.Fatalf("排队启动错误 = %v, want %v", result.err, errBrowserStartRuntimeStopped)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("关闭后排队启动未及时返回")
	}

	gate.mu.Lock()
	occupied := gate.occupied()
	gate.mu.Unlock()
	if occupied != 0 {
		t.Fatalf("关闭后的闸门占用 = %d, want 0", occupied)
	}
}
