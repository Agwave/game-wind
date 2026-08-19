package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"game-wind/internal/analyze"
	"game-wind/internal/fetch"
	"game-wind/internal/mapping"
	"game-wind/internal/store"
)

func entry(id, name, artist string, rank int) fetch.Entry {
	return fetch.Entry{Rank: rank, AppID: id, Name: name, Artist: artist}
}

func chart(entries ...fetch.Entry) []fetch.Entry { return entries }

// testTable 3 家公司：腾讯（王者荣耀/三角洲行动）、世纪华通（无尽冬日）、电魂网络（梦三国）。
func testTable(t *testing.T) *mapping.Table {
	t.Helper()
	yamlContent := `
companies:
  - company: 腾讯控股
    code: "0700.HK"
    market: 港股
    games:
      - ids: [id1]
        names: [王者荣耀]
      - ids: [id2]
        names: [三角洲行动]
  - company: 世纪华通
    code: "002602.SZ"
    market: A股
    games:
      - ids: [id3]
        names: [无尽冬日, Whiteout Survival]
  - company: 电魂网络
    code: "603258.SH"
    market: A股
    games:
      - ids: [id4]
        names: [梦三国]
unrelated_artists:
  - Nintendo
  - miHoYo
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

// testManyGamesTable 单公司 6 款游戏，验证 top5 截断。
func testManyGamesTable(t *testing.T) *mapping.Table {
	t.Helper()
	yamlContent := `
companies:
  - company: 腾讯控股
    code: "0700.HK"
    market: 港股
    games:
      - ids: [g1]
        names: [游戏一]
      - ids: [g2]
        names: [游戏二]
      - ids: [g3]
        names: [游戏三]
      - ids: [g4]
        names: [游戏四]
      - ids: [g5]
        names: [游戏五]
      - ids: [g6]
        names: [游戏六]
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

func TestCompanySection(t *testing.T) {
	cur := &store.Snapshot{Date: "2026-08-19", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(
			entry("id1", "王者荣耀", "", 2),
			entry("id2", "三角洲行动", "", 9),
			entry("id3", "无尽冬日", "", 5),
		)},
		"us": {TopGrossing: chart(entry("id3", "Whiteout Survival", "", 6))},
	}}
	prev := &store.Snapshot{Date: "2026-08-18", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(
			entry("id1", "王者荣耀", "", 3),
			entry("id3", "无尽冬日", "", 3),
		)},
		"us": {TopGrossing: chart(entry("id3", "Whiteout Survival", "", 11))},
	}}
	got := CompanySection(cur, prev, testTable(t))
	for _, want := range []string{
		"## 一、游戏公司（3 家）",
		"### 腾讯控股 · 0700.HK",
		"🇨🇳 畅销榜：王者荣耀 第2(↑1) · 三角洲行动 第9(新)",
		"### 世纪华通 · 002602.SZ",
		"🇨🇳 畅销榜：无尽冬日 第5(↓2)",
		"🇺🇸 畅销榜：无尽冬日 第6(↑5)",
		"### 电魂网络 · 603258.SH",
		"该公司目前无上榜",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("缺少 %q\n---\n%s", want, got)
		}
	}
}

func TestCompanySectionTop5(t *testing.T) {
	cur := &store.Snapshot{Date: "2026-08-19", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(
			entry("g1", "游戏一", "", 1), entry("g2", "游戏二", "", 2),
			entry("g3", "游戏三", "", 3), entry("g4", "游戏四", "", 4),
			entry("g5", "游戏五", "", 5), entry("g6", "游戏六", "", 6),
		)},
	}}
	prev := &store.Snapshot{Date: "2026-08-18", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(
			entry("g1", "游戏一", "", 1), entry("g2", "游戏二", "", 2),
			entry("g3", "游戏三", "", 3), entry("g4", "游戏四", "", 4),
			entry("g5", "游戏五", "", 5), entry("g6", "游戏六", "", 6),
		)},
	}}
	got := CompanySection(cur, prev, testManyGamesTable(t))
	if strings.Contains(got, "游戏六") {
		t.Errorf("只应显示前5款，却包含 游戏六:\n%s", got)
	}
	if !strings.Contains(got, "游戏五 第5(—)") {
		t.Errorf("应显示前5款并含 游戏五 第5(—):\n%s", got)
	}
}

func TestCompanySectionBaseline(t *testing.T) {
	cur := &store.Snapshot{Date: "2026-08-19", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(entry("id1", "王者荣耀", "", 2))},
	}}
	got := CompanySection(cur, nil, testTable(t))
	if !strings.Contains(got, "王者荣耀 第2(—)") {
		t.Errorf("首次运行（无 prev）变动应标 —:\n%s", got)
	}
	if strings.Contains(got, "(新)") {
		t.Errorf("首次运行不应标 新:\n%s", got)
	}
}

func TestSummarySection(t *testing.T) {
	results := map[string]*analyze.Result{
		"cn": {Changes: []analyze.Change{
			{Game: "王者荣耀", Canonical: "王者荣耀", Company: "腾讯控股", Region: "cn", Chart: fetch.ChartGrossing, Kind: analyze.KindRise, FromRank: 3, ToRank: 2, Estimate: "估算 400万+/日"},
		}},
		"us": {Changes: []analyze.Change{
			{Game: "Whiteout Survival", Canonical: "无尽冬日", Company: "世纪华通", Region: "us", Chart: fetch.ChartGrossing, Kind: analyze.KindRise, FromRank: 11, ToRank: 6},
		}},
	}
	got := SummarySection(results, []string{"无尽冬日（世纪华通）：中国区/美国区 同时上榜变化"}, false)
	for _, want := range []string{
		"## 二、总结",
		"值得关注：",
		"腾讯控股",
		"世纪华通",
		"王者荣耀",
		"跨区信号：",
		"无尽冬日（世纪华通）：中国区/美国区 同时上榜变化",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("缺少 %q\n---\n%s", want, got)
		}
	}
}

func TestSummarySectionNoChange(t *testing.T) {
	got := SummarySection(map[string]*analyze.Result{"cn": {}}, nil, false)
	if !strings.Contains(got, "今日无重大变化") {
		t.Errorf("应输出无变化: %s", got)
	}
}

func TestSummarySectionBaseline(t *testing.T) {
	got := SummarySection(nil, nil, true)
	if !strings.Contains(got, "基线") {
		t.Errorf("应输出基线提示: %s", got)
	}
}

func TestPendingSection(t *testing.T) {
	results := map[string]*analyze.Result{
		"cn": {Unmapped: []analyze.Change{
			{Game: "灵墟幻想", AppID: "888", Region: "cn", Chart: fetch.ChartGrossing, Kind: analyze.KindNew, ToRank: 8},
			{Game: "山海迷城", AppID: "999", Region: "cn", Chart: fetch.ChartGrossing, Kind: analyze.KindRise, FromRank: 40, ToRank: 22},
		}},
	}
	got := PendingSection(results)
	for _, want := range []string{
		"待确认（未映射，可能属于目标公司，请确认）：",
		"🇨🇳 畅销榜 第8 灵墟幻想(id:888) 新进",
		"🇨🇳 畅销榜 第22 山海迷城(id:999) 上升↑18",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("缺少 %q\n---\n%s", want, got)
		}
	}
}

func TestUnrelatedSection(t *testing.T) {
	cur := &store.Snapshot{Date: "2026-08-19", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(
			entry("id1", "王者荣耀", "", 1),
			entry("zzz", "塞尔达传说", "Nintendo Co., Ltd.", 12),
			entry("yyy", "原神", "miHoYo", 9),
		)},
	}}
	got := UnrelatedSection(cur, testTable(t))
	for _, want := range []string{
		"无关公司（确认非目标上市公司，其变动忽略）：",
		"miHoYo",
		"Nintendo",
		"原神 第9",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("缺少 %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, "王者荣耀") {
		t.Errorf("目标公司游戏不应列为无关:\n%s", got)
	}
}

func TestSupplementEmpty(t *testing.T) {
	got := SupplementSection(&store.Snapshot{Charts: map[string]*store.Chart{}}, testTable(t), map[string]*analyze.Result{"cn": {}})
	if got != "" {
		t.Errorf("补充段应输出空, 实际: %s", got)
	}
}

func TestBuild(t *testing.T) {
	cur := &store.Snapshot{Date: "2026-08-19", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(entry("id1", "王者荣耀", "", 2))},
	}}
	prev := &store.Snapshot{Date: "2026-08-18", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(entry("id1", "王者荣耀", "", 3))},
	}}
	results := map[string]*analyze.Result{
		"cn": {Changes: []analyze.Change{
			{Game: "王者荣耀", Canonical: "王者荣耀", Company: "腾讯控股", Region: "cn", Chart: fetch.ChartGrossing, Kind: analyze.KindRise, FromRank: 3, ToRank: 2},
		}},
	}
	md := Build("2026-08-19", cur, prev, testTable(t), results, nil, nil, false)
	for _, want := range []string{
		"# 📊 游戏公司观察日报 2026-08-19",
		"## 一、游戏公司（3 家）",
		"## 二、总结",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("缺少 %q\n---\n%s", want, md)
		}
	}
}
