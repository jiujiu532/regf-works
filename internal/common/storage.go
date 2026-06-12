package common

import (
	"encoding/json"
	"os"
	"sync"
)

// ResultStorage 结果持久化存储
type ResultStorage struct {
	filePath string
	mu       sync.RWMutex
	results  []RegisterResult
}

// NewResultStorage 创建结果存储
func NewResultStorage(filePath string) *ResultStorage {
	s := &ResultStorage{
		filePath: filePath,
		results:  []RegisterResult{},
	}
	s.Load()
	return s
}

// Load 从文件加载历史结果
func (s *ResultStorage) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在，忽略
		}
		return err
	}

	return json.Unmarshal(data, &s.results)
}

// Save 保存当前结果到文件
func (s *ResultStorage) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.results, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := "data"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// Append 追加新结果并持久化
func (s *ResultStorage) Append(result RegisterResult) error {
	s.mu.Lock()
	s.results = append([]RegisterResult{result}, s.results...) // 新结果插入开头
	s.mu.Unlock()

	return s.Save()
}

// GetAll 获取所有结果
func (s *ResultStorage) GetAll() []RegisterResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本，避免外部修改
	copy := make([]RegisterResult, len(s.results))
	for i, r := range s.results {
		copy[i] = r
	}
	return copy
}

// Clear 清空所有结果
func (s *ResultStorage) Clear() error {
	s.mu.Lock()
	s.results = []RegisterResult{}
	s.mu.Unlock()

	return s.Save()
}
