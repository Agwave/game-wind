# 报告新格式（按公司分组）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把每日报告从「按地区」改为「标题 / 游戏公司 / 总结 / 补充」四段式，并支持按公司枚举各榜单 top5 排名变动、未映射待确认与无关发行商过滤。

**Architecture:** `report` 包重写为接收 `cur/prev` 快照 + 映射表 + analyze 结果，输出新四段；`mapping.Table` 增加公司列表（`Companies`）与无关发行商名单（`UnrelatedArtists`/`IsUnrelated`）；`analyze.Result.Unmapped` 由 `[]string` 改为结构化 `[]Change`；`notify.BuildMessages` 简化为单正文截断；`main` 接线并把 `hasChanges` 扩展到未映射变动。

**Tech Stack:** Go（gopkg.in/yaml.v3、标准库），CLAUDE.md 强制校验链 `gofmt -w cmd internal && go build ./... && go vet ./... && golangci-lint run ./... && go test ./...`。

前置说明：本计划改 `report.go` / `analyze.go` / `notify.go` / `main.go` 有跨文件耦合（`report.Build` 新签名、`analyze.Result.Unmapped` 类型、`notify.BuildMessages` 签名），Task 3、Task 4 内部各文件必须同任务一起改、一起提交，才能保持每次提交可编译。

参考 spec：`docs/superpowers/specs/2026-08-19-report-redesign-design.md`。

---

### Task 1: mapping 增加 Companies / UnrelatedArtists / IsUnrelated

**Files:**
- Modify: `internal/mapping/mapping.go`
- Test: `internal/mapping/mapping_test.go`

- [ ] **Step 1: 先加失败测试**

在 `internal/mapping/mapping_test.go` 末尾追加：

```go
// TestUnrelatedArtists 无关发行商名单：加载、小写子串匹配、空 artist、不误伤目标公司。
func TestUnrelatedArtists(t *testing.T) {
	yamlContent := `
unrelated_artists:
  - Nintendo
  - miHoYo
  - King
companies:
  - company: 腾讯控股
    code: "0700.HK"
    market: 港股
    games:
      - names: [王者荣耀]
`
	p := filepath.Join(t.TempDir(), "games.yaml")
	if err := os.WriteFile(p, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	tab, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(tab.Companies) != 1 || tab.Companies[0].Company != "腾讯控股" {
		t.Errorf("Companies 应保留 yaml 顺序: %+v", tab.Companies)
	}
	if !tab.IsUnrelated("Nintendo Co., Ltd.") {
		t.Error("小写子串应命中 Nintendo Co., Ltd.")
	}
	if !tab.IsUnrelated("miHoYo Limited") {
		t.Error("应命中 miHoYo Limited")
	}
	if tab.IsUnrelated("") {
		t.Error("空 artist 不应命中")
	}
	if tab.IsUnrelated("Century Games Pte. Ltd.") {
		t.Error("Century（目标公司出海品牌）不应命中")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `source ~/.g/env && go test -run TestUnrelatedArtists ./internal/mapping/`
Expected: FAIL — `tab.Companies` 编译报错（字段不存在）/ 断言失败。

- [ ] **Step 3: 实现 mapping.go 变更**

`internal/mapping/mapping.go` 三处修改：

(1) `tableFile` 结构体加字段：

```go
type tableFile struct {
	Companies        []Company `yaml:"companies"`
	UnrelatedArtists []string  `yaml:"unrelated_artists"`
}
```

(2) `Table` 结构体加字段：

```go
type Table struct {
	byID             map[string]*Matched
	byName           map[string]*Matched
	byContains       []containsEntry
	Companies        []Company
	UnrelatedArtists []string
}
```

(3) `Load` 中构造 `Table` 后填充新字段，并在文件末尾加两个方法（`Load` 里已有 `strings` 导入，直接可用）：

```go
	t := &Table{
		byID:             make(map[string]*Matched),
		byName:           make(map[string]*Matched),
		Companies:        tf.Companies,
		UnrelatedArtists: normalizeArtists(tf.UnrelatedArtists),
	}
```

```go
// normalizeArtists 小写、去首尾空格、去掉空串。
func normalizeArtists(ss []string) []string {
	var out []string
	for _, s := range ss {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// IsUnrelated 判断开发者账号名（Artist）是否命中无关发行商名单（小写子串匹配，大小写不敏感）。
func (t *Table) IsUnrelated(artist string) bool {
	if t == nil || artist == "" {
		return false
	}
	artist = strings.ToLower(artist)
	for _, a := range t.UnrelatedArtists {
		if a != "" && strings.Contains(artist, a) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: 运行确认通过**

Run: `source ~/.g/env && go test ./internal/mapping/`
Expected: PASS（含新增 TestUnrelatedArtists）。

- [ ] **Step 5: Commit**

```bash
git add internal/mapping/mapping.go internal/mapping/mapping_test.go
git commit -m "feat(mapping): Table 增加公司列表与无关发行商名单（IsUnrelated）
- Companies 保留 yaml 顺序，供报告按公司分段
- unrelated_artists 按 Artist 小写子串匹配"
```

---

### Task 2: games.yaml 增加无关发行商种子名单

**Files:**
- Modify: `data/games.yaml`

- [ ] **Step 1: 在文件顶部（`companies:` 之前）插入名单**

在 `data/games.yaml` 的 `# ids/names 均来自 2026-08-18 真实榜单抓取…` 注释块之后、`companies:` 之前插入：

```yaml
# 无关发行商名单：确认非目标上市公司的发行商。榜单上命中 Artist（开发者账号名）的游戏
# 视为无关，报告归到「无关公司」，其变动不再单独提及。可用「补充·待确认」持续补充。
# 注意：不要加入 Century（世纪华通出海品牌，属于目标公司）。
unrelated_artists:
  - Nintendo
  - miHoYo
  - Supercell
  - Playrix
  - King
  - Roblox
  - Netmarble
  - Nexon
  - Krafton
  - Konami
  - Bandai Namco
  - Square Enix
  - Capcom
  - SEGA
  - IronSource
  - Voodoo
```

- [ ] **Step 2: 确认映射表仍可加载**

Run: `source ~/.g/env && go test ./internal/mapping/`
Expected: PASS（`unrelated_artists` 是新增顶层字段，mapping 正常解析）。

- [ ] **Step 3: Commit**

```bash
git add data/games.yaml
git commit -m "feat(mapping): games.yaml 增加无关发行商种子名单"
```

---

### Task 3: analyze.Unmapped 结构化 + report 全量重写（耦合变更，一起提交）

**Files:**
- Modify: `internal/analyze/analyze.go`
- Modify: `internal/analyze/analyze_test.go`
- Rewrite: `internal/report/report.go`
- Create: `internal/report/report_test.go`

- [ ] **Step 1: 更新 analyze 测试（先红）**

在 `internal/analyze/analyze_test.go` 末尾追加两个新用例，验证未映射的上升与 Top10 变动进入 `Unmapped`：

```go
// TestUnmappedRise 未映射游戏的明显上升应收录到 Unmapped（供报告补充段使用），不进入 Changes。
func TestUnmappedRise(t *testing.T) {
	prev := &store.Snapshot{Date: "2026-08-17", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(entry("id100", "某未映射游戏", 80))},
	}}
	cur := &store.Snapshot{Date: "2026-08-18", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(entry("id100", "某未映射游戏", 40))}, // 80→40 上升40 ≥30
	}}
	tab := loadTestTable(t)
	res := Analyze(prev, cur, "cn", tab, config.Region{GrossingRise: 30})
	if len(res.Unmapped) != 1 {
		t.Fatalf("期望 1 条未映射上升, 实际 %+v", res.Unmapped)
	}
	if res.Unmapped[0].Kind != KindRise || res.Unmapped[0].Company != "" {
		t.Errorf("期望 Rise 且未映射, 实际 %+v", res.Unmapped[0])
	}
	if len(res.Changes) != 0 {
		t.Errorf("未映射上升不应进入 Changes: %+v", res.Changes)
	}
}

// TestUnmappedTopBand 未映射游戏进入 Top10 也应录入 Unmapped。
func TestUnmappedTopBand(t *testing.T) {
	prev := &store.Snapshot{Date: "2026-08-17", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(entry("id100", "某未映射游戏", 12))},
	}}
	cur := &store.Snapshot{Date: "2026-08-18", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: chart(entry("id100", "某未映射游戏", 6))}, // 12→6 进入 Top10
	}}
	tab := loadTestTable(t)
	res := Analyze(prev, cur, "cn", tab, config.Region{GrossingTopBand: 10})
	if len(res.Unmapped) != 1 || res.Unmapped[0].Kind != KindTopBand {
		t.Fatalf("期望 1 条未映射 Top10 变动, 实际 %+v", res.Unmapped)
	}
}
```

`TestAnalyzeCN` 中 `if len(res.Unmapped) != 2 { t.Errorf(...) }` 的断言保持原样（改为 `[]Change` 后长度仍为 2），类型变化无需改断言。

Run: `source ~/.g/env && go test ./internal/analyze/`
Expected: FAIL — 编译失败（`Unmapped` 仍是 `[]string`，比较/赋值 `Kind` 字段报错）。

- [ ] **Step 2: 修改 analyze.go**

`internal/analyze/analyze.go` 三处修改：

(1) `Result` 结构体注释与类型：

```go
// Result 一个地区的分析结果。
type Result struct {
	Changes  []Change
	Unmapped []Change // 未映射到目标公司的重大变化（Company 为空），供报告「补充·待确认」使用
}
```

(2) `diffChart` 函数体整体替换为（`addChange` 分流，未映射的上升/Top10 变动/跌出也进 `Unmapped`）：

```go
// diffChart 对比一张榜单：新进（New）优先，其次是 TopN 内变动（TopBand），最后是大幅上升（Rise）。
func diffChart(prev *store.Snapshot, prevEntries, curEntries []fetch.Entry, cc, chart string, tab *mapping.Table, newTop, rise, topBand int, res *Result) {
	prevRank := make(map[string]int, len(prevEntries))
	for _, e := range prevEntries {
		prevRank[e.AppID] = e.Rank
	}
	// 第一遍：新进 + 上升（已报告的 app 在第二遍跳过，避免重复）
	reported := make(map[string]bool, len(curEntries))
	for _, e := range curEntries {
		pr, ok := prevRank[e.AppID]
		if !ok {
			if e.Rank <= newTop {
				addChange(res, makeChange(e, cc, chart, KindNew, 0, e.Rank, tab))
				reported[e.AppID] = true
			}
			continue
		}
		if rise > 0 && pr-e.Rank >= rise {
			addChange(res, makeChange(e, cc, chart, KindRise, pr, e.Rank, tab))
			reported[e.AppID] = true
		}
	}
	// 第二遍：TopN 内变动（含跌出 TopN / 新进 TopN）
	if topBand > 0 {
		curRank := make(map[string]int, len(curEntries))
		for _, e := range curEntries {
			curRank[e.AppID] = e.Rank
		}
		seen := make(map[string]bool, len(curEntries))
		consider := func(appID string, pr, cr int) {
			if seen[appID] || reported[appID] || pr == cr {
				return
			}
			seen[appID] = true
			for _, e := range curEntries {
				if e.AppID == appID {
					addChange(res, makeChange(e, cc, chart, KindTopBand, pr, cr, tab))
					return
				}
			}
			for _, e := range prevEntries {
				if e.AppID == appID {
					addChange(res, makeChange(e, cc, chart, KindTopBand, pr, 0, tab))
					return
				}
			}
		}
		for _, e := range prevEntries {
			if e.Rank <= topBand {
				consider(e.AppID, e.Rank, curRank[e.AppID])
			}
		}
		for _, e := range curEntries {
			if e.Rank <= topBand {
				consider(e.AppID, prevRank[e.AppID], e.Rank)
			}
		}
	}
	_ = prev // 保留签名：prev 快照用于未来扩展（如名字追溯）
}

// addChange 按是否命中目标公司分流：命中的进 Changes，未命中的进 Unmapped。
func addChange(res *Result, ch Change) {
	if ch.Company == "" {
		res.Unmapped = append(res.Unmapped, ch)
		return
	}
	res.Changes = append(res.Changes, ch)
}
```

(3) 删除 `diffChart` 中原先两处 `res.Unmapped = append(res.Unmapped, fmt.Sprintf(...))` 与 `res.Changes = append(res.Changes, ...)` 的直接追加（已被 `addChange` 替代）；若 `fmt` 不再被 `diffChart` 使用，确认文件其他地方仍引用 `fmt`（`makeChange` 的 `cnEstimate`、`CrossRegion`、`changeLine` 均用 `fmt`），保留导入。

Run: `source ~/.g/env && go test ./internal/analyze/`
Expected: PASS。

- [ ] **Step 3: 先写 report 失败测试（新文件）**

创建 `internal/report/report_test.go`，完整内容：

```go
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
      - ids: [g1] names: [游戏一]
      - ids: [g2] names: [游戏二]
      - ids: [g3] names: [游戏三]
      - ids: [g4] names: [游戏四]
      - ids: [g5] names: [游戏五]
      - ids: [g6] names: [游戏六]
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
```

Run: `source ~/.g/env && go test ./internal/report/`
Expected: FAIL — 编译错误（`CompanySection`/`Build` 等函数不存在）。

- [ ] **Step 4: 重写 report.go**

用以下内容整体替换 `internal/report/report.go`（保留 `RegionNames`/`regionFlags`/`feedLabel` 的能力，新增全部小节函数）：

```go
// Package report 生成每日 Markdown 报告。
package report

import (
	"fmt"
	"sort"
	"strings"

	"game-wind/internal/analyze"
	"game-wind/internal/fetch"
	"game-wind/internal/mapping"
	"game-wind/internal/store"
)

// RegionNames 地区代码 → 展示名。
var RegionNames = map[string]string{
	"cn": "中国区",
	"us": "美国区",
	"jp": "日本区",
	"kr": "韩国区",
}

var regionFlags = map[string]string{
	"cn": "🇨🇳",
	"us": "🇺🇸",
	"jp": "🇯🇵",
	"kr": "🇰🇷",
}

var regionOrder = []string{"cn", "us", "jp", "kr"}
var chartOrder = []string{fetch.ChartGrossing, fetch.ChartFree}

// Build 生成完整日报：标题 + 游戏公司 + 总结 + 补充。
// cur/prev：当前/上一快照；tab：映射表（含公司列表与无关发行商名单）；
// results：各地区的分析结果；cross：跨区信号；failed：抓取失败 feed；baseline：首次运行。
func Build(date string, cur, prev *store.Snapshot, tab *mapping.Table,
	results map[string]*analyze.Result, cross []string, failed []string, baseline bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 📊 游戏公司观察日报 %s\n", date)
	fmt.Fprintf(&b, "> 数据源：iOS App Store 畅销榜/免费榜（游戏分类 · 前100）· 与最近一次快照对比\n\n")
	if baseline {
		b.WriteString("> ℹ️ 首次运行，已建立基线快照，明日开始对比变化。\n\n")
	}
	if len(failed) > 0 {
		b.WriteString("> ⚠️ 抓取状态：")
		for i, f := range failed {
			if i > 0 {
				b.WriteString("；")
			}
			b.WriteString(feedLabel(f))
			b.WriteString(" 抓取失败（已重试）")
		}
		b.WriteString("\n\n")
	}
	b.WriteString(CompanySection(cur, prev, tab))
	b.WriteString("\n")
	b.WriteString(SummarySection(results, cross, baseline))
	if sup := SupplementSection(cur, tab, results); sup != "" {
		b.WriteString("\n")
		b.WriteString(sup)
	}
	return b.String()
}

// chartPos 一款游戏在某个榜单上的位置与上期排名（所属地区/榜单由 posKey 承载）。
type chartPos struct {
	game string
	rank int
	prev int // -1 = 无 prev 快照（首次运行）；0 = 有快照但该 app 未上榜；>0 = 上一排名
}

type posKey struct {
	region string
	chart  string
}

// CompanySection 一、游戏公司：按映射表全部公司分段，列出各榜单前 5 款游戏及变动。
func CompanySection(cur, prev *store.Snapshot, tab *mapping.Table) string {
	if tab == nil {
		return ""
	}
	positions := collectPositions(cur, prev, tab)
	var b strings.Builder
	fmt.Fprintf(&b, "## 一、游戏公司（%d 家）\n\n", len(tab.Companies))
	for _, c := range tab.Companies {
		fmt.Fprintf(&b, "### %s · %s\n", c.Company, c.Code)
		pos := positions[c.Company]
		anyOnChart := false
		for _, region := range regionOrder {
			for _, chartType := range chartOrder {
				ps := pos[posKey{region: region, chart: chartType}]
				if len(ps) == 0 {
					continue
				}
				anyOnChart = true
				sort.SliceStable(ps, func(i, j int) bool { return ps[i].rank < ps[j].rank })
				if len(ps) > 5 {
					ps = ps[:5]
				}
				b.WriteString("- ")
				b.WriteString(regionFlags[region])
				b.WriteString(" ")
				b.WriteString(fetch.ChartNames[chartType])
				b.WriteString("：")
				for i, p := range ps {
					if i > 0 {
						b.WriteString(" · ")
					}
					fmt.Fprintf(&b, "%s 第%d(%s)", p.game, p.rank, changeArrow(p.rank, p.prev))
				}
				b.WriteString("\n")
			}
		}
		if !anyOnChart {
			b.WriteString("- 该公司目前无上榜\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// collectPositions 枚举每家公司 × (地区, 榜单) 当前上榜游戏的位置。
func collectPositions(cur, prev *store.Snapshot, tab *mapping.Table) map[string]map[posKey][]chartPos {
	out := map[string]map[posKey][]chartPos{}
	if cur == nil {
		return out
	}
	prevRanks := prevRanksByChart(prev)
	for cc, ch := range cur.Charts {
		for _, chartType := range chartOrder {
			var entries []fetch.Entry
			if chartType == fetch.ChartGrossing {
				entries = ch.TopGrossing
			} else {
				entries = ch.TopFree
			}
			key := posKey{region: cc, chart: chartType}
			prevKey := cc + "/" + chartType
			for _, e := range entries {
				m := tab.Match(e.AppID, e.Name)
				if m == nil {
					continue
				}
				pr := prevRanks[prevKey][e.AppID]
				if prev == nil {
					pr = -1 // 首次运行：无上期排名，变动标 —
				}
				p := chartPos{game: m.Game, rank: e.Rank, prev: pr}
				if out[m.Company] == nil {
					out[m.Company] = map[posKey][]chartPos{}
				}
				out[m.Company][key] = append(out[m.Company][key], p)
			}
		}
	}
	return out
}

// prevRanksByChart 返回 region/chart → appID → 排名；无 prev 快照时各 app 均为 -1。
func prevRanksByChart(prev *store.Snapshot) map[string]map[string]int {
	out := map[string]map[string]int{}
	if prev == nil {
		return out
	}
	for cc, ch := range prev.Charts {
		for _, chartType := range chartOrder {
			key := cc + "/" + chartType
			m := map[string]int{}
			entries := ch.TopGrossing
			if chartType == fetch.ChartFree {
				entries = ch.TopFree
			}
			for _, e := range entries {
				m[e.AppID] = e.Rank
			}
			out[key] = m
		}
	}
	return out
}

// changeArrow 计算变动标注：首次运行 → "—"；新上榜 → "新"；否则 ↑n / ↓n / —。
func changeArrow(curRank, prevRank int) string {
	switch {
	case prevRank == -1:
		return "—"
	case prevRank == 0:
		return "新"
	case curRank == prevRank:
		return "—"
	case curRank < prevRank:
		return fmt.Sprintf("↑%d", prevRank-curRank)
	default:
		return fmt.Sprintf("↓%d", curRank-prevRank)
	}
}

// SummarySection 二、总结：值得关注的公司/游戏变动 + 跨区信号。
func SummarySection(results map[string]*analyze.Result, cross []string, baseline bool) string {
	var b strings.Builder
	b.WriteString("## 二、总结\n\n")
	if baseline {
		b.WriteString("ℹ️ 首次运行，已建立基线快照，明日开始对比变化。\n")
		return b.String()
	}
	var changes []analyze.Change
	for _, r := range results {
		if r == nil {
			continue
		}
		changes = append(changes, r.Changes...)
	}
	if len(changes) == 0 && len(cross) == 0 {
		b.WriteString("今日无重大变化\n")
		return b.String()
	}
	b.WriteString("值得关注：\n")
	byCompany := map[string][]analyze.Change{}
	var order []string
	for _, c := range changes {
		if _, ok := byCompany[c.Company]; !ok {
			order = append(order, c.Company)
		}
		byCompany[c.Company] = append(byCompany[c.Company], c)
	}
	sort.SliceStable(order, func(i, j int) bool {
		return len(byCompany[order[i]]) > len(byCompany[order[j]])
	})
	for _, company := range order {
		cs := byCompany[company]
		sort.SliceStable(cs, func(i, j int) bool { return cs[i].ToRank < cs[j].ToRank })
		multi := ""
		if len(cs) >= 2 {
			multi = fmt.Sprintf("（%d款）", len(cs))
		}
		b.WriteString("- ")
		b.WriteString(company)
		b.WriteString(multi)
		b.WriteString("：")
		for _, c := range cs {
			b.WriteString(" ")
			b.WriteString(summaryChangeLine(&c))
			b.WriteString("；")
		}
		b.WriteString("\n")
	}
	if len(cross) > 0 {
		b.WriteString("跨区信号：\n")
		for _, s := range cross {
			fmt.Fprintf(&b, "- %s\n", s)
		}
	}
	return b.String()
}

// summaryChangeLine 一条已映射变化在总结段的展示。
func summaryChangeLine(c *analyze.Change) string {
	game := c.Canonical
	if game == "" {
		game = c.Game
	}
	region := RegionNames[c.Region]
	chart := fetch.ChartNames[c.Chart]
	move := ""
	switch c.Kind {
	case analyze.KindNew:
		move = fmt.Sprintf("新进第%d", c.ToRank)
	case analyze.KindTopBand:
		if c.ToRank == 0 {
			move = fmt.Sprintf("跌出前%d", c.FromRank)
		} else if c.FromRank == 0 {
			move = fmt.Sprintf("新进第%d", c.ToRank)
		} else {
			move = fmt.Sprintf("第%d→第%d", c.FromRank, c.ToRank)
		}
	case analyze.KindRise:
		move = fmt.Sprintf("第%d→第%d（↑%d）", c.FromRank, c.ToRank, c.FromRank-c.ToRank)
	}
	line := fmt.Sprintf("%s %s%s·%s", game, region, chart, move)
	if c.Estimate != "" {
		line += " · " + c.Estimate
	}
	return line
}

// PendingSection 三、补充·待确认：未映射游戏的重大变化，供人工确认。
func PendingSection(results map[string]*analyze.Result) string {
	var unmapped []analyze.Change
	for _, r := range results {
		if r == nil {
			continue
		}
		unmapped = append(unmapped, r.Unmapped...)
	}
	if len(unmapped) == 0 {
		return ""
	}
	sort.SliceStable(unmapped, func(i, j int) bool { return posLess(unmapped[i], unmapped[j]) })
	var b strings.Builder
	b.WriteString("待确认（未映射，可能属于目标公司，请确认）：\n")
	for _, c := range unmapped {
		if c.ToRank == 0 {
			fmt.Fprintf(&b, "- %s %s %s(id:%s) 跌出前%d\n", regionFlags[c.Region], fetch.ChartNames[c.Chart], c.Game, c.AppID, c.FromRank)
			continue
		}
		fmt.Fprintf(&b, "- %s %s 第%d %s(id:%s) %s\n", regionFlags[c.Region], fetch.ChartNames[c.Chart], c.ToRank, c.Game, c.AppID, pendingMove(&c))
	}
	return b.String()
}

// pendingMove 待确认条目的变动描述。
func pendingMove(c *analyze.Change) string {
	switch c.Kind {
	case analyze.KindNew:
		return "新进"
	case analyze.KindRise:
		return fmt.Sprintf("上升↑%d", c.FromRank-c.ToRank)
	case analyze.KindTopBand:
		return fmt.Sprintf("Top10变动 第%d→第%d", c.FromRank, c.ToRank)
	}
	return ""
}

// posLess 未映射条目的展示顺序：地区 > 榜单 > 排名。
func posLess(a, b analyze.Change) bool {
	if a.Region != b.Region {
		return regionIndex(a.Region) < regionIndex(b.Region)
	}
	if a.Chart != b.Chart {
		return chartIndex(a.Chart) < chartIndex(b.Chart)
	}
	return a.ToRank < b.ToRank
}

func regionIndex(cc string) int {
	for i, r := range regionOrder {
		if r == cc {
			return i
		}
	}
	return len(regionOrder)
}

func chartIndex(ch string) int {
	for i, c := range chartOrder {
		if c == ch {
			return i
		}
	}
	return len(chartOrder)
}

// UnrelatedSection 三、补充·无关公司：命中无关发行商名单、非目标公司的游戏，按发行商分组。
func UnrelatedSection(cur *store.Snapshot, tab *mapping.Table) string {
	if cur == nil || tab == nil {
		return ""
	}
	type unrel struct {
		game string
		rank int
	}
	byArtist := map[string][]unrel{}
	for _, ch := range cur.Charts {
		for _, chartType := range chartOrder {
			var entries []fetch.Entry
			if chartType == fetch.ChartGrossing {
				entries = ch.TopGrossing
			} else {
				entries = ch.TopFree
			}
			for _, e := range entries {
				if tab.Match(e.AppID, e.Name) != nil {
					continue
				}
				if !tab.IsUnrelated(e.Artist) {
					continue
				}
				byArtist[e.Artist] = append(byArtist[e.Artist], unrel{game: e.Name, rank: e.Rank})
			}
		}
	}
	if len(byArtist) == 0 {
		return ""
	}
	var artists []string
	for a := range byArtist {
		artists = append(artists, a)
	}
	sort.Strings(artists)
	var b strings.Builder
	b.WriteString("无关公司（确认非目标上市公司，其变动忽略）：\n")
	for _, a := range artists {
		ps := byArtist[a]
		sort.SliceStable(ps, func(i, j int) bool { return ps[i].rank < ps[j].rank })
		if len(ps) > 5 {
			ps = ps[:5]
		}
		b.WriteString("- ")
		b.WriteString(a)
		b.WriteString("：")
		for i, p := range ps {
			if i > 0 {
				b.WriteString(" · ")
			}
			fmt.Fprintf(&b, "%s 第%d", p.game, p.rank)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// SupplementSection 三、补充 = 待确认 + 无关公司；均无内容时返回空串。
func SupplementSection(cur *store.Snapshot, tab *mapping.Table, results map[string]*analyze.Result) string {
	pend := PendingSection(results)
	unrel := UnrelatedSection(cur, tab)
	if pend == "" && unrel == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("## 三、补充\n\n")
	b.WriteString(pend)
	if pend != "" {
		b.WriteString("\n")
	}
	b.WriteString(unrel)
	return b.String()
}

// feedLabel 把 "us/topfreeapplications" 转成 "美国区/免费榜"。
func feedLabel(feed string) string {
	parts := strings.SplitN(feed, "/", 2)
	cc, chart := parts[0], ""
	if len(parts) == 2 {
		chart = parts[1]
	}
	name := RegionNames[cc]
	if name == "" {
		name = cc
	}
	if chart == "" {
		return name
	}
	return name + "/" + fetch.ChartNames[chart]
}
```

Run: `source ~/.g/env && go test ./internal/report/`
Expected: PASS（含新增 9 个 report 用例）。

- [ ] **Step 5: 全量测试**

Run: `source ~/.g/env && go test ./...`
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/analyze/analyze.go internal/analyze/analyze_test.go internal/report/report.go internal/report/report_test.go
git commit -m "feat(report): 报告改为「标题/游戏公司/总结/补充」四段式

- 游戏公司段按 yaml 全部公司枚举各榜单 top5 与排名变动（↑/↓/新/—），无上榜标明
- 总结段沿用现有阈值 + 跨区信号；补充段含未映射待确认与无关发行商过滤
- analyze.Result.Unmapped 改为结构化 []Change，未映射上升/Top10 变动归入
- 新增 report_test.go 9 个用例"
```

---

### Task 4: notify 简化 + main 接线

**Files:**
- Modify: `internal/notify/notify.go`
- Modify: `cmd/gamewind/main.go`

- [ ] **Step 1: 改 notify.go 的 BuildMessages**

`internal/notify/notify.go`：

(1) 将 `BuildMessages` 与 `truncateSection` 替换为：

```go
// BuildMessages 组装一条推送：header + body + footer；body 超过 MaxBytes 按行截断并注明省略行数。
// 完整报告在 data/reports/ 落盘，不丢失信息。
func BuildMessages(header string, body string, footer string) ([]string, error) {
	msg := header + "\n" + body + "\n" + footer
	if len(msg) <= MaxBytes {
		return []string{msg}, nil
	}
	return []string{header + "\n" + truncateBody(body) + "\n" + footer}, nil
}

// truncateBody 按行截断正文，保证整体不超过 MaxBytes。
func truncateBody(body string) string {
	base := MaxBytes - 64 // 给 header/footer 和省略说明预留
	lines := strings.Split(body, "\n")
	var kept []string
	used := 0
	for _, l := range lines {
		cost := len(l) + 1 // 含换行
		if used+cost > base {
			break
		}
		kept = append(kept, l)
		used += cost
	}
	skipped := len(lines) - len(kept)
	if skipped > 0 {
		kept = append(kept, fmt.Sprintf("…（内容过长，已省略 %d 行，完整报告见 data/reports/）", skipped))
	}
	return strings.Join(kept, "\n")
}
```

(2) 删除不再使用的 `sortedKeys` 函数（`BuildMessages` 不再按地区拆分）。确认 `strings` 仍被 `truncateBody` 使用，保留导入。

- [ ] **Step 2: 改 main.go**

`cmd/gamewind/main.go` 三处修改：

(1) `analyzeReportAndNotify` 中 `report.Build` 调用改为传入快照与映射表：

```go
	md := report.Build(date, snap, prev, tab, results, cross, snap.Failed, baseline)
```

(2) `hasChanges` 扩展为含未映射变动：

```go
	hasChanges := false
	for _, r := range results {
		if len(r.Changes) > 0 || len(r.Unmapped) > 0 {
			hasChanges = true
			break
		}
	}
```

(3) `notifyAll` 签名加 `cross` 参数，并把默认分支的推送正文改为「总结 + 待确认」：

```go
// notifyAll 组装并推送企业微信消息；用 state.json 防同一天重复推送。
func notifyAll(cfg *config.Config, results map[string]*analyze.Result, cross []string, date string, hasChanges, baseline bool, md, dir string) error {
	w := notify.New(cfg.Notify.WebhookURL)
	if w == nil {
		return nil
	}
	header := fmt.Sprintf("# 📊 游戏公司观察 · %s", date)
	footer := fmt.Sprintf("\n---\n完整报告：data/reports/%s.md", date)

	var msgs []string
	kind := "report"
	switch {
	case baseline:
		// 首次运行：告知基线已建立，避免用户以为没跑
		kind = "baseline"
		msgs = []string{header + "\n\n✅ 基线已建立：今日 4 地区榜单快照已入库，明天起每天推送重大变化。" + footer}
	case !hasChanges:
		kind = "quiet"
		msgs = []string{header + "\n\n今日无重大变化。" + footer}
	default:
		// 推送精简版：总结 + 待确认（无关公司只在落盘报告）
		body := report.SummarySection(results, cross, false)
		if pend := report.PendingSection(results); pend != "" {
			body += "\n\n" + pend
		}
		var err error
		msgs, err = notify.BuildMessages(header, body, footer)
		if err != nil {
			return err
		}
	}
	// 指纹包含消息类型：同一天"安静/基线/报告"消息内容不同，不能互相去重
	sum := sha256.Sum256([]byte(md + "\n" + kind))
	hash := hex.EncodeToString(sum[:8])
	st := store.LoadState(dir)
	if st.LastSnapshotDate == date && st.LastNotifiedHash == hash {
		fmt.Println("ℹ️ 今天已推送过相同内容，跳过")
		return nil
	}
	if err := w.SendAll(msgs); err != nil {
		return err
	}
	return store.SaveState(dir, store.State{LastSnapshotDate: date, LastNotifiedHash: hash})
}
```

同时把调用处改为 `notifyAll(cfg, results, cross, date, hasChanges, baseline, md, dir)`。

- [ ] **Step 3: 编译 + 全量测试**

Run: `source ~/.g/env && gofmt -w cmd internal && go build ./... && go vet ./... && golangci-lint run ./... && go test ./...`
Expected: `0 issues.` 且全部 `ok`。

- [ ] **Step 4: 手动跑一份样例报告（验证四段输出）**

Run（临时 demo，跑完自动删除）：

```bash
source ~/.g/env && cat > zz_demo_report.go <<'EOF'
package main

import (
	"fmt"

	"game-wind/internal/analyze"
	"game-wind/internal/fetch"
	"game-wind/internal/mapping"
	"game-wind/internal/report"
	"game-wind/internal/store"
)

func main() {
	tab, err := mapping.Load("data/games.yaml")
	if err != nil {
		panic(err)
	}
	e := func(rank int, id, name, artist string) fetch.Entry {
		return fetch.Entry{Rank: rank, AppID: id, Name: name, Artist: artist}
	}
	cur := &store.Snapshot{Date: "2026-08-19", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: []fetch.Entry{
			e(1, "989673964", "王者荣耀", "Tencent"), e(3, "1321803705", "和平精英", "Tencent"),
			e(5, "6478492012", "无尽冬日", "Century"), e(9, "1642894547", "三角洲行动", "Tencent"),
			e(12, "zzz1", "塞尔达传说", "Nintendo Co., Ltd."), e(9, "yyy1", "原神", "miHoYo"),
		}},
		"us": {TopGrossing: []fetch.Entry{
			e(6, "6478492012", "Whiteout Survival", "Century"), e(21, "zzz2", "Super Mario", "Nintendo Co., Ltd."),
		}},
		"jp": {TopGrossing: []fetch.Entry{
			e(2, "6478492012", "ホワイトアウト・サバイバル", "Century"),
		}},
		"kr": {TopFree: []fetch.Entry{e(18, "6757360957", "삼국지: 천하결전", "bilibili")}},
	}
	prev := &store.Snapshot{Date: "2026-08-18", Charts: map[string]*store.Chart{
		"cn": {TopGrossing: []fetch.Entry{e(3, "989673964", "王者荣耀", ""), e(12, "6478492012", "无尽冬日", "")}},
		"us": {TopGrossing: []fetch.Entry{e(11, "6478492012", "Whiteout Survival", "")}},
		"jp": {TopGrossing: []fetch.Entry{e(4, "6478492012", "ホワイトアウト・サバイバル", "")}},
	}}
	results := map[string]*analyze.Result{
		"cn": {
			Changes: []analyze.Change{
				{AppID: "989673964", Game: "王者荣耀", Canonical: "王者荣耀", Company: "腾讯控股", Region: "cn", Chart: fetch.ChartGrossing, Kind: analyze.KindRise, FromRank: 3, ToRank: 1, Estimate: "估算 400万+/日"},
			},
			Unmapped: []analyze.Change{
				{Game: "灵墟幻想", AppID: "888", Region: "cn", Chart: fetch.ChartGrossing, Kind: analyze.KindNew, ToRank: 8},
			},
		},
		"us": {Changes: []analyze.Change{
			{AppID: "6478492012", Game: "Whiteout Survival", Canonical: "无尽冬日", Company: "世纪华通", Region: "us", Chart: fetch.ChartGrossing, Kind: analyze.KindRise, FromRank: 11, ToRank: 6},
		}},
	}
	cross := analyze.CrossRegion(results)
	md := report.Build("2026-08-19", cur, prev, tab, results, cross, []string{"kr/topfreeapplications"}, false)
	fmt.Println(md)
}
EOF
go run zz_demo_report.go; rm -f zz_demo_report.go
```

Expected: 输出包含 `## 一、游戏公司（N 家）`、各公司榜单行、`## 二、总结`、`## 三、补充`（待确认 + 无关公司）、抓取状态行。

- [ ] **Step 5: Commit**

```bash
git add internal/notify/notify.go cmd/gamewind/main.go
git commit -m "feat(notify): 推送改为「总结+待确认」精简版，report.Build 接入新签名

- notify.BuildMessages 由按地区多段改为单正文截断
- hasChanges 含未映射变动，避免待确认日不推送"
```

---

### Task 5: 整体校验

- [ ] **Step 1: 完整校验链**

Run: `source ~/.g/env && gofmt -w cmd internal && go build ./... && go vet ./... && golangci-lint run ./... && go test ./...`
Expected: `0 issues.` 且全部测试 `ok`。

- [ ] **Step 2: 确认工作区干净（仅期望提交过的文件）**

Run: `git status --short`
Expected: 无未提交改动。

---

## 自检记录

- **Spec 覆盖**：标题保留（Task 4 Step 2 / report.Build）✓；游戏公司段全部公司+top5+↑↓新—（Task 3 CompanySection）✓；无上榜（CompanySection `anyOnChart`）✓；展示顺序中国>美国>日本>韩国、畅销>免费（regionOrder/chartOrder）✓；总结沿用阈值+跨区（SummarySection）✓；补充待确认+无关公司（Pending/UnrelatedSection）✓；抓取状态保留在标题下（Build）✓；推送改新格式（Task 4）✓；baseline 行为（CompanySection changeArrow prev=-1 / SummarySection baseline）✓。
- **占位符扫描**：无 TBD/TODO；每个代码步骤均给出完整代码。
- **类型一致性**：`chartPos.prev` 用 -1/0/正数三态贯穿 `collectPositions`/`changeArrow`；`analyze.Result.Unmapped` 类型 `[]Change` 在 analyze_test/report_test/main.go 中一致；`notify.BuildMessages(header, body, footer)` 签名在 notify 与 main 中一致。
