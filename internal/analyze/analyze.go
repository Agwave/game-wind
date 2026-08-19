// Package analyze 对比前后两天快照，判定「重大变化」。
package analyze

import (
	"fmt"
	"sort"

	"game-wind/internal/config"
	"game-wind/internal/fetch"
	"game-wind/internal/mapping"
	"game-wind/internal/store"
)

// Kind 变化类型。
type Kind string

// 变化类型常量：新进榜、畅销榜 TopN 内变动、排名大幅上升。
const (
	KindNew     Kind = "new"
	KindTopBand Kind = "top"
	KindRise    Kind = "rise"
)

// Change 一条重大变化记录。
type Change struct {
	AppID     string // App Store app id
	Game      string // 游戏名（该地区标题）
	Canonical string // 映射表规范名（names[0]，跨地区统一展示用）
	Company   string // 上市公司（"" = 未映射）
	Code      string
	Market    string
	Region    string // cn/us/jp/kr
	Chart     string // fetch.ChartGrossing / fetch.ChartFree
	Kind      Kind
	FromRank  int // 0 = 新进
	ToRank    int
	Estimate  string // 中国区畅销榜流水估算（其他地区为空）
}

// Result 一个地区的分析结果。
type Result struct {
	Changes  []Change
	Unmapped []Change // 未映射到目标公司的重大变化（Company 为空），供报告「补充·待确认」使用
}

// Analyze 对比 prev 与 cur 两个快照，产出地区 cc 的重大变化。
// prev 为 nil 时（首次运行）不产出任何变化。
func Analyze(prev, cur *store.Snapshot, cc string, tab *mapping.Table, cfg config.Region) *Result {
	res := &Result{}
	if prev == nil {
		return res
	}
	pc, ok := prev.Charts[cc]
	if !ok || pc == nil {
		return res
	}
	ccur := cur.Charts[cc]
	if ccur == nil {
		return res
	}
	diffChart(prev, pc.TopGrossing, ccur.TopGrossing, cc, fetch.ChartGrossing, tab, cfg.GrossingNewTop, cfg.GrossingRise, cfg.GrossingTopBand, res)
	diffChart(prev, pc.TopFree, ccur.TopFree, cc, fetch.ChartFree, tab, cfg.FreeNewTop, cfg.FreeRise, 0, res)
	sortChanges(res.Changes)
	return res
}

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
				ch := makeChange(e, cc, chart, KindNew, 0, e.Rank, tab)
				addChange(res, &ch)
				reported[e.AppID] = true
			}
			continue
		}
		if rise > 0 && pr-e.Rank >= rise {
			ch := makeChange(e, cc, chart, KindRise, pr, e.Rank, tab)
			addChange(res, &ch)
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
			// 找出该 app 的当前条目
			for _, e := range curEntries {
				if e.AppID == appID {
					ch := makeChange(e, cc, chart, KindTopBand, pr, cr, tab)
					addChange(res, &ch)
					return
				}
			}
			// 已跌出榜单（cr == 0）：用 prev 条目的信息补一条
			for _, e := range prevEntries {
				if e.AppID == appID {
					ch := makeChange(e, cc, chart, KindTopBand, pr, 0, tab)
					addChange(res, &ch)
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
func addChange(res *Result, ch *Change) {
	if ch.Company == "" {
		res.Unmapped = append(res.Unmapped, *ch)
		return
	}
	res.Changes = append(res.Changes, *ch)
}

func makeChange(e fetch.Entry, cc, chart string, kind Kind, from, to int, tab *mapping.Table) Change {
	ch := Change{
		AppID: e.AppID, Game: e.Name, Region: cc, Chart: chart,
		Kind: kind, FromRank: from, ToRank: to,
	}
	if m := tab.Match(e.AppID, e.Name); m != nil {
		ch.Company, ch.Code, ch.Market = m.Company, m.Code, m.Market
		ch.Canonical = m.Game
	}
	if cc == "cn" && chart == fetch.ChartGrossing && to > 0 {
		ch.Estimate = cnEstimate(to)
	}
	return ch
}

// sortChanges 按公司出现次数降序、再按排名升序排列（热门公司靠前）。
func sortChanges(cs []Change) {
	sort.SliceStable(cs, func(i, j int) bool {
		ci, cj := countByCompany(cs, cs[i].Company), countByCompany(cs, cs[j].Company)
		if ci != cj {
			return ci > cj
		}
		return cs[i].ToRank < cs[j].ToRank
	})
}

func countByCompany(cs []Change, company string) int {
	n := 0
	for _, c := range cs {
		if c.Company == company {
			n++
		}
	}
	return n
}

// cnEstimate 中国区畅销榜排名 → 日流水估算（仅供参考的量级区间）。
func cnEstimate(rank int) string {
	switch {
	case rank == 1:
		return "估算 400万+/日"
	case rank <= 3:
		return "估算 250-400万/日"
	case rank <= 10:
		return "估算 100-250万/日"
	case rank <= 20:
		return "估算 50-100万/日"
	case rank <= 50:
		return "估算 20-50万/日"
	default:
		return "估算 10-20万/日"
	}
}

// CrossRegion 汇总跨区信号：同一款游戏（app_id）在 >=2 个地区同时出现重大变化。
// 返回形如 "Whiteout Survival：美/日/韩 同时上榜变化" 的摘要。
func CrossRegion(results map[string]*Result) []string {
	byApp := make(map[string][]Change)
	for _, r := range results {
		for _, c := range r.Changes {
			if c.Company == "" {
				continue
			}
			byApp[c.AppID] = append(byApp[c.AppID], c)
		}
	}
	var out []string
	apps := make([]string, 0, len(byApp))
	for appID := range byApp {
		apps = append(apps, appID)
	}
	sort.Strings(apps)
	for _, appID := range apps {
		cs := byApp[appID]
		regions := make(map[string]bool)
		var game, canonical, company string
		for _, c := range cs {
			regions[c.Region] = true
			game, company = c.Game, c.Company
			if canonical == "" && c.Canonical != "" {
				canonical = c.Canonical
			}
		}
		if len(regions) >= 2 {
			rs := make([]string, 0, len(regions))
			for r := range regions {
				rs = append(rs, regionName(r))
			}
			sort.Strings(rs)
			name := canonical
			if name == "" {
				name = game
			}
			out = append(out, fmt.Sprintf("%s（%s）：%s 同时上榜变化", name, company, join(rs)))
		}
	}
	return out
}

func join(ss []string) string {
	s := ""
	for i, v := range ss {
		if i > 0 {
			s += "/"
		}
		s += v
	}
	return s
}

func regionName(cc string) string {
	switch cc {
	case "cn":
		return "中国区"
	case "us":
		return "美国区"
	case "jp":
		return "日本区"
	case "kr":
		return "韩国区"
	}
	return cc
}
