package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// PoolStore 是运行时可编辑的身份池:以磁盘 overlay 文件为准,缺失时用内嵌默认物化。
// 每次增删改都原子写回 overlay 并重建加权采样池(Pool),供新建环境即时采样。
// 编辑只影响之后新建的环境;已有环境身份已落库(browser_identities),不受影响。
type PoolStore struct {
	mu      sync.Mutex
	path    string
	records []PoolRecord
	pool    *Pool
}

// NewPoolStore 打开(或首次物化)overlay 身份池。
// overlay 存在且可解析→用它;不存在→用内嵌默认物化并落盘;
// 存在但解析失败→把坏文件备份为 <path>.bad,再用内嵌默认恢复(保证始终可用)。
func NewPoolStore(path string) (*PoolStore, error) {
	s := &PoolStore{path: path}
	// 空路径:纯内存池(用内嵌默认物化,不落盘;用于测试或无状态目录场景)。
	if strings.TrimSpace(path) == "" {
		if err := s.materializeFromEmbedded(); err != nil {
			return nil, err
		}
		return s, nil
	}
	data, err := os.ReadFile(path)
	if err == nil {
		var recs []PoolRecord
		if jsonErr := json.Unmarshal(data, &recs); jsonErr == nil && len(recs) > 0 {
			assignMissingIDs(recs)
			s.records = recs
			s.pool = NewPool(recs)
			return s, nil
		}
		// 坏文件:备份后回退内嵌默认。
		_ = os.Rename(path, path+".bad")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := s.materializeFromEmbedded(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PoolStore) materializeFromEmbedded() error {
	recs, err := EmbeddedPoolRecords()
	if err != nil {
		return err
	}
	assignMissingIDs(recs)
	s.records = recs
	s.pool = NewPool(recs)
	return s.persistLocked()
}

// Pool 返回当前采样池(供 GenerateUnique 采样)。
func (s *PoolStore) Pool() *Pool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pool
}

// Count 返回记录数。
func (s *PoolStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}

// Records 返回记录副本(只读)。
func (s *PoolStore) Records() []PoolRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]PoolRecord(nil), s.records...)
}

// Add 新增一条记录(自动赋 ID),返回落库后的记录。
func (s *PoolStore) Add(rec PoolRecord) (PoolRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec.ID = s.newIDLocked()
	s.records = append(s.records, rec)
	if err := s.saveAndRebuildLocked(); err != nil {
		return PoolRecord{}, err
	}
	return rec, nil
}

// Update 按 ID 覆盖一条记录(ID 保持不变)。
func (s *PoolStore) Update(id string, rec PoolRecord) (PoolRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.indexOfLocked(id)
	if idx < 0 {
		return PoolRecord{}, fmt.Errorf("identity: 未找到身份池记录 %q", id)
	}
	rec.ID = id
	s.records[idx] = rec
	if err := s.saveAndRebuildLocked(); err != nil {
		return PoolRecord{}, err
	}
	return rec, nil
}

// Delete 按 ID 删除一条记录。
func (s *PoolStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.indexOfLocked(id)
	if idx < 0 {
		return fmt.Errorf("identity: 未找到身份池记录 %q", id)
	}
	s.records = append(s.records[:idx], s.records[idx+1:]...)
	return s.saveAndRebuildLocked()
}

// RestoreDefaults 用内嵌默认池覆盖 overlay(重新赋 ID)。
func (s *PoolStore) RestoreDefaults() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.materializeFromEmbedded()
}

func (s *PoolStore) indexOfLocked(id string) int {
	for i := range s.records {
		if s.records[i].ID == id {
			return i
		}
	}
	return -1
}

func (s *PoolStore) saveAndRebuildLocked() error {
	if err := s.persistLocked(); err != nil {
		return err
	}
	s.pool = NewPool(s.records)
	return nil
}

func (s *PoolStore) persistLocked() error {
	if strings.TrimSpace(s.path) == "" {
		return nil // 纯内存池,不落盘
	}
	data, err := json.MarshalIndent(s.records, "", "  ")
	if err != nil {
		return err
	}
	return writePoolFileAtomic(s.path, data)
}

func (s *PoolStore) newIDLocked() string {
	return fmt.Sprintf("p%06d", maxIDNum(s.records)+1)
}

func assignMissingIDs(recs []PoolRecord) {
	next := maxIDNum(recs) + 1
	for i := range recs {
		if strings.TrimSpace(recs[i].ID) == "" {
			recs[i].ID = fmt.Sprintf("p%06d", next)
			next++
		}
	}
}

func maxIDNum(recs []PoolRecord) int {
	max := 0
	for _, r := range recs {
		if strings.HasPrefix(r.ID, "p") {
			if n, err := strconv.Atoi(r.ID[1:]); err == nil && n > max {
				max = n
			}
		}
	}
	return max
}

// writePoolFileAtomic 原子写(temp+rename),避免写一半损坏 overlay。
func writePoolFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pool-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
