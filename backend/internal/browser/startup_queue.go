package browser

import (
	"fmt"
	"sync"
	"time"
)

// StartupQueue 启动队列，用于限制并发启动数量
type StartupQueue struct {
	maxConcurrent int
	semaphore     chan struct{}
	waiting       map[string]time.Time // profileId -> 加入队列时间
	mu            sync.Mutex
}

// NewStartupQueue 创建启动队列
func NewStartupQueue(maxConcurrent int) *StartupQueue {
	if maxConcurrent <= 0 {
		maxConcurrent = 3 // 默认并发3个
	}
	return &StartupQueue{
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
		waiting:       make(map[string]time.Time),
	}
}

// Acquire 获取启动许可（会阻塞直到有空位）
func (q *StartupQueue) Acquire(profileId string) error {
	q.mu.Lock()
	q.waiting[profileId] = time.Now()
	queueSize := len(q.waiting)
	q.mu.Unlock()

	// 尝试获取信号量
	select {
	case q.semaphore <- struct{}{}:
		// 成功获取
		q.mu.Lock()
		delete(q.waiting, profileId)
		q.mu.Unlock()
		return nil
	case <-time.After(60 * time.Second):
		// 超时
		q.mu.Lock()
		delete(q.waiting, profileId)
		q.mu.Unlock()
		return fmt.Errorf("启动队列等待超时（队列长度：%d，超时：60秒）", queueSize)
	}
}

// Release 释放启动许可
func (q *StartupQueue) Release() {
	<-q.semaphore
}

// GetQueueInfo 获取队列信息
func (q *StartupQueue) GetQueueInfo() map[string]interface{} {
	q.mu.Lock()
	defer q.mu.Unlock()

	return map[string]interface{}{
		"maxConcurrent": q.maxConcurrent,
		"current":       len(q.semaphore),
		"waiting":       len(q.waiting),
		"available":     q.maxConcurrent - len(q.semaphore),
	}
}

// GetWaitingProfiles 获取等待中的实例列表
func (q *StartupQueue) GetWaitingProfiles() []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	profiles := make([]string, 0, len(q.waiting))
	for profileId := range q.waiting {
		profiles = append(profiles, profileId)
	}
	return profiles
}
