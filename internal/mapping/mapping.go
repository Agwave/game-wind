// Package mapping 加载 data/games.yaml 游戏→上市公司映射表并提供匹配。
package mapping

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Game 一款游戏（可对应多个地区的多个 App Store app）。
type Game struct {
	IDs      []string `yaml:"ids"`      // App Store app_id，跨地区稳定，优先级最高
	Names    []string `yaml:"names"`    // 与榜单标题完全相等匹配
	Contains []string `yaml:"contains"` // 标题包含该子串匹配
	Note     string   `yaml:"note,omitempty"`
}

// Company 一家上市公司及其游戏列表。
type Company struct {
	Company string `yaml:"company"`
	Code    string `yaml:"code"`
	Market  string `yaml:"market"`
	Games   []Game `yaml:"games"`
}

// Matched 匹配结果。
type Matched struct {
	Company string
	Code    string
	Market  string
	Game    string // 命中的游戏名（取第一个 names，无 names 取第一个 contains）
	Note    string
}

type tableFile struct {
	Companies []Company `yaml:"companies"`
}

// Table 映射表，按优先级索引：app_id → 公司，名称（精确/包含）→ 公司。
type Table struct {
	byID       map[string]*Matched
	byName     map[string]*Matched
	byContains []containsEntry
}

type containsEntry struct {
	sub string
	m   *Matched
}

// Load 从 YAML 文件加载映射表。
func Load(path string) (*Table, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取映射表 %s: %w", path, err)
	}
	var tf tableFile
	if err := yaml.Unmarshal(b, &tf); err != nil {
		return nil, fmt.Errorf("解析映射表 %s: %w", path, err)
	}
	t := &Table{
		byID:   make(map[string]*Matched),
		byName: make(map[string]*Matched),
	}
	for _, c := range tf.Companies {
		if c.Company == "" {
			return nil, fmt.Errorf("映射表 %s: 存在缺少 company 的条目", path)
		}
		for _, g := range c.Games {
			m := &Matched{Company: c.Company, Code: c.Code, Market: c.Market, Note: g.Note}
			switch {
			case len(g.Names) > 0:
				m.Game = g.Names[0]
			case len(g.Contains) > 0:
				m.Game = g.Contains[0]
			default:
				m.Game = "(未命名条目)"
			}
			for _, id := range g.IDs {
				t.byID[id] = m
			}
			for _, n := range g.Names {
				t.byName[n] = m
			}
			for _, sub := range g.Contains {
				t.byContains = append(t.byContains, containsEntry{sub: sub, m: m})
			}
		}
	}
	return t, nil
}

// Match 按 app_id → 精确名 → 包含名 的顺序匹配一条榜单记录。
func (t *Table) Match(appID, name string) *Matched {
	if t == nil {
		return nil
	}
	if m, ok := t.byID[appID]; ok && appID != "" {
		return m
	}
	if m, ok := t.byName[name]; ok {
		return m
	}
	for _, e := range t.byContains {
		if strings.Contains(name, e.sub) {
			return e.m
		}
	}
	return nil
}
