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

// prevRanksByChart 返回 region/chart → appID → 排名；无 prev 快照时为空。
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
		switch {
		case c.ToRank == 0:
			move = fmt.Sprintf("跌出前%d", c.FromRank)
		case c.FromRank == 0:
			move = fmt.Sprintf("新进第%d", c.ToRank)
		default:
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
	sort.SliceStable(unmapped, func(i, j int) bool { return posLess(&unmapped[i], &unmapped[j]) })
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
func posLess(a, b *analyze.Change) bool {
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
