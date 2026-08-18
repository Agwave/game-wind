// Package store 负责榜单快照与运行状态的读写（JSON 文件，零依赖）。
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"game-wind/internal/fetch"
)

// Chart 一个地区的两张榜单。
type Chart struct {
	TopGrossing []fetch.Entry `json:"top_grossing"`
	TopFree     []fetch.Entry `json:"top_free"`
}

// Snapshot 某一天的榜单快照。
type Snapshot struct {
	Date      string            `json:"date"` // YYYY-MM-DD
	FetchedAt time.Time         `json:"fetched_at"`
	Charts    map[string]*Chart `json:"charts"`           // key: 地区代码（cn/us/jp/kr）
	Failed    []string          `json:"failed,omitempty"` // 抓取失败的 feed，如 "us/topfreeapplications"
}

// Save 保存快照到 dataDir/<date>.json。
func (s *Snapshot) Save(dir string) error {
	if s.Date == "" {
		return fmt.Errorf("快照缺少日期")
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, s.Date+".json"), b, 0o644)
}

// Load 读取指定日期的快照。
func Load(dir, date string) (*Snapshot, error) {
	b, err := os.ReadFile(filepath.Join(dir, date+".json"))
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("解析快照 %s: %w", date, err)
	}
	return &s, nil
}

// ListDates 返回已保存快照的日期列表（升序）。
func ListDates(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var dates []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") || len(name) != len("2026-08-18.json") {
			continue
		}
		dates = append(dates, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(dates)
	return dates, nil
}

// LatestBefore 返回严格早于 date 的最近一次快照；没有则返回 nil。
func LatestBefore(dir, date string) (*Snapshot, error) {
	dates, err := ListDates(dir)
	if err != nil {
		return nil, err
	}
	for i := len(dates) - 1; i >= 0; i-- {
		if dates[i] < date {
			return Load(dir, dates[i])
		}
	}
	return nil, nil
}

// State 运行状态，用于防止同一天重复推送。
type State struct {
	LastSnapshotDate string `json:"last_snapshot_date"`
	LastNotifiedHash string `json:"last_notified_hash"`
}

// LoadState 读取运行状态（文件不存在时返回空状态）。
func LoadState(dir string) State {
	var st State
	b, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err == nil {
		_ = json.Unmarshal(b, &st)
	}
	return st
}

// SaveState 保存运行状态。
func SaveState(dir string, st State) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "state.json"), b, 0o644)
}
