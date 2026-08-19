package analyze

import (
	"os"
	"path/filepath"
	"testing"

	"game-wind/internal/config"
	"game-wind/internal/fetch"
	"game-wind/internal/mapping"
	"game-wind/internal/store"
)

func entry(id, name string, rank int) fetch.Entry {
	return fetch.Entry{Rank: rank, AppID: id, Name: name}
}

func loadTestTable(t *testing.T) *mapping.Table {
	t.Helper()
	yamlContent := `
companies:
  - company: 腾讯控股
    code: "0700.HK"
    market: 港股
    games:
      - ids: [id16]
        names: [穿越火线-枪战王者]
      - ids: [id17]
        names: [金铲铲之战]
      - ids: [id14]
        names: [暗区突围2]
      - names: [元梦之星]
  - company: 网易
    code: "9999.HK"
    market: 港股
    games:
      - ids: [id15]
        names: [星辰大海]
      - ids: [id8]
        names: [梦幻西游]
      - ids: [id9]
        names: [蛋仔派对]
      - ids: [id10]
        names: [第五人格]
  - company: 世纪华通
    code: "002602.SZ"
    market: A股
    games:
      - ids: [6443575749]
        names: [Whiteout Survival]
  - company: 创梦天地
    code: "1119.HK"
    market: 港股
    games:
      - ids: [995122577]
        names: [地铁跑酷]
`
	p := filepath.Join(t.TempDir(), "games.yaml")
	if err := os.WriteFile(p, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	tab, err := mapping.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return tab
}

func chart(entries ...fetch.Entry) []fetch.Entry { return entries }

// TestAnalyzeCN 覆盖：新进畅销榜前50、新进免费榜前20、上升≥30、Top10 内变动、跌出 Top10、未映射新进。
func TestAnalyzeCN(t *testing.T) {
	prev := &store.Snapshot{
		Date: "2026-08-17",
		Charts: map[string]*store.Chart{
			"cn": {
				TopGrossing: chart(
					entry("id1", "王者荣耀", 1),
					entry("id2", "和平精英", 2),
					entry("id3", "三角洲行动", 3),
					entry("id16", "穿越火线-枪战王者", 50),
					entry("id17", "金铲铲之战", 20),
					entry("id8", "梦幻西游", 8),
					entry("id9", "蛋仔派对", 9),
					entry("id10", "第五人格", 10),
					entry("id100", "某未映射游戏", 40),
				),
				TopFree: chart(
					entry("id4", "无畏契约", 1),
					entry("id3", "三角洲行动", 2),
					entry("id17", "金铲铲之战", 40),
				),
			},
		},
	}
	cur := &store.Snapshot{
		Date: "2026-08-18",
		Charts: map[string]*store.Chart{
			"cn": {
				TopGrossing: chart(
					entry("id1", "王者荣耀", 1),
					entry("id2", "和平精英", 2),
					entry("id3", "三角洲行动", 3),
					entry("id16", "穿越火线-枪战王者", 15), // 50→15 上升35 ≥30 → Rise
					entry("id17", "金铲铲之战", 8),      // 20→8 进入 Top10 → TopBand
					entry("id8", "梦幻西游", 8),
					entry("id9", "蛋仔派对", 10),   // 9→10 Top10 内变动 → TopBand
					entry("id10", "第五人格", 12),  // 10→12 跌出 Top10 → TopBand
					entry("id14", "暗区突围2", 13), // 新进前50 → New（腾讯）
					entry("id15", "星辰大海", 14),  // 新进前50 → New（网易）
					entry("id13", "原神", 30),    // 新进前50 → New（未映射）
					entry("id100", "某未映射游戏", 41),
				),
				TopFree: chart(
					entry("id4", "无畏契约", 1),
					entry("id3", "三角洲行动", 2),
					entry("id17", "金铲铲之战", 5),       // 40→5 上升35 → Rise
					entry("id18", "迷你世界", 5),        // 新进前20 → New（未映射）
					entry("id995122577", "地铁跑酷", 8), // 新进前20 → New（创梦天地）
				),
			},
		},
	}

	tab := loadTestTable(t)
	cfg := config.Region{GrossingNewTop: 50, GrossingRise: 30, GrossingTopBand: 10, FreeNewTop: 20, FreeRise: 30}
	res := Analyze(prev, cur, "cn", tab, cfg)

	find := func(appID string) *Change {
		for i := range res.Changes {
			if res.Changes[i].AppID == appID {
				return &res.Changes[i]
			}
		}
		return nil
	}

	// 新进（KindNew）
	if c := find("id14"); c == nil || c.Kind != KindNew || c.Company != "腾讯控股" || c.Estimate == "" {
		t.Errorf("id14 期望 New/腾讯控股/含估算, 实际 %+v", c)
	}
	if c := find("id15"); c == nil || c.Kind != KindNew || c.Company != "网易" {
		t.Errorf("id15 期望 New/网易, 实际 %+v", c)
	}
	if c := find("id995122577"); c == nil || c.Kind != KindNew || c.Company != "创梦天地" || c.Chart != fetch.ChartFree {
		t.Errorf("地铁跑酷 期望 New/创梦天地/免费榜, 实际 %+v", c)
	}
	// 上升（KindRise）
	if c := find("id16"); c == nil || c.Kind != KindRise || c.FromRank != 50 || c.ToRank != 15 {
		t.Errorf("id16 期望 Rise 50→15, 实际 %+v", c)
	}
	if c := find("id17"); c == nil {
		t.Errorf("id17（免费榜 40→5）期望 Rise, 实际没有")
	}
	// Top10 变动（KindTopBand）
	if c := find("id9"); c == nil || c.Kind != KindTopBand {
		t.Errorf("id9 期望 TopBand, 实际 %+v", c)
	}
	if c := find("id10"); c == nil || c.Kind != KindTopBand || c.ToRank != 12 {
		t.Errorf("id10 期望 TopBand 跌出, 实际 %+v", c)
	}
	// 未映射新进不进入 Changes
	if c := find("id13"); c != nil {
		t.Errorf("原神未映射不应进入 Changes: %+v", c)
	}
	// 中国区畅销榜估算
	if c := find("id14"); c != nil && c.Estimate == "" {
		t.Errorf("中国区畅销榜变化应带估算")
	}
}

// TestNoDuplicateChange 同时满足「上升≥30」和「进入 Top10」的游戏只报一条。
func TestNoDuplicateChange(t *testing.T) {
	prev := &store.Snapshot{
		Date: "2026-08-17",
		Charts: map[string]*store.Chart{
			"cn": {TopGrossing: chart(entry("id17", "金铲铲之战", 45))},
		},
	}
	cur := &store.Snapshot{
		Date: "2026-08-18",
		Charts: map[string]*store.Chart{
			"cn": {TopGrossing: chart(entry("id17", "金铲铲之战", 5))}, // 45→5：上升40 且进 Top10
		},
	}
	tab := loadTestTable(t)
	res := Analyze(prev, cur, "cn", tab, config.Region{GrossingNewTop: 50, GrossingRise: 30, GrossingTopBand: 10})
	n := 0
	for _, c := range res.Changes {
		if c.AppID == "id17" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("id17 应只报 1 条变化, 实际 %d 条: %+v", n, res.Changes)
	}
}

// TestAnalyzeFirstRun 首次运行（prev 为 nil）不应产出变化。
func TestAnalyzeFirstRun(t *testing.T) {
	cur := &store.Snapshot{
		Date: "2026-08-18",
		Charts: map[string]*store.Chart{
			"cn": {TopGrossing: chart(entry("id1", "王者荣耀", 1))},
		},
	}
	tab := loadTestTable(t)
	res := Analyze(nil, cur, "cn", tab, config.Region{GrossingNewTop: 50})
	if len(res.Changes) != 0 {
		t.Errorf("首次运行应无变化, 实际 %+v", res)
	}
}

// TestCrossRegion 同一 app_id 在多个地区同时变化应产生跨区信号。
func TestCrossRegion(t *testing.T) {
	prev := &store.Snapshot{
		Date: "2026-08-17",
		Charts: map[string]*store.Chart{
			"us": {TopGrossing: chart(entry("6443575749", "Whiteout Survival", 80))},
			"kr": {TopGrossing: chart(entry("6443575749", "WOS: 화이트아웃 서바이벌", 90))},
		},
	}
	cur := &store.Snapshot{
		Date: "2026-08-18",
		Charts: map[string]*store.Chart{
			"us": {TopGrossing: chart(entry("6443575749", "Whiteout Survival", 40))}, // 上升40 ≥30
			"kr": {TopGrossing: chart(entry("6443575749", "WOS: 화이트아웃 서바이벌", 30))},
		},
	}
	tab := loadTestTable(t)
	results := map[string]*Result{
		"us": Analyze(prev, cur, "us", tab, config.Region{GrossingRise: 30}),
		"kr": Analyze(prev, cur, "kr", tab, config.Region{GrossingRise: 30}),
	}
	if len(results["us"].Changes) != 1 || len(results["kr"].Changes) != 1 {
		t.Fatalf("两个地区都应产出变化, us=%+v kr=%+v", results["us"].Changes, results["kr"].Changes)
	}
	cross := CrossRegion(results)
	if len(cross) != 1 {
		t.Fatalf("期望 1 条跨区信号, 实际 %v", cross)
	}
	if cross[0] != "Whiteout Survival（世纪华通）：美国区/韩国区 同时上榜变化" {
		t.Errorf("跨区信号内容不符: %s", cross[0])
	}
}
