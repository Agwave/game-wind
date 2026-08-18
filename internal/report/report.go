// Package report 生成每日 Markdown 报告。
package report

import (
	"fmt"
	"sort"
	"strings"

	"game-wind/internal/analyze"
	"game-wind/internal/fetch"
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

var kindLabel = map[analyze.Kind]string{
	analyze.KindNew:     "新进",
	analyze.KindTopBand: "Top10变动",
	analyze.KindRise:    "上升",
}

// Build 生成完整日报。
// results: 地区 → 分析结果；cross: 跨区信号；failed: 抓取失败的 feed；
// baseline: 首次运行（无历史对比）。
func Build(date string, results map[string]*analyze.Result, cross, failed []string, baseline bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 📊 游戏公司观察日报 %s\n", date)
	fmt.Fprintf(&b, "> 数据源：iOS App Store 畅销榜/免费榜（游戏分类 · 前100）· 与最近一次快照对比\n\n")

	if baseline {
		b.WriteString("> ℹ️ 首次运行，已建立基线快照，明日开始对比变化。\n\n")
	}
	if len(cross) > 0 {
		b.WriteString("## 🏆 跨区信号\n")
		for _, s := range cross {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}

	regions := make([]string, 0, len(results))
	for cc := range results {
		regions = append(regions, cc)
	}
	sort.Strings(regions)
	hasChanges := false
	for _, cc := range regions {
		if sec := RegionSection(cc, results[cc]); sec != "" {
			hasChanges = true
			b.WriteString(sec)
		}
	}
	if !hasChanges && !baseline {
		b.WriteString("## 今日无重大变化\n\n")
	}
	if len(failed) > 0 {
		b.WriteString("## ⚠️ 抓取状态\n")
		for _, f := range failed {
			fmt.Fprintf(&b, "- %s 抓取失败（已重试）\n", feedLabel(f))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// RegionSection 生成一个地区的报告片段；无变化时返回 ""。
func RegionSection(cc string, r *analyze.Result) string {
	if r == nil || (len(r.Changes) == 0 && len(r.Unmapped) == 0) {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## %s %s\n", regionFlags[cc], RegionNames[cc])
	// 按公司分组；未映射的单独归入「未映射变动」
	byCompany := make(map[string][]analyze.Change)
	var order, unmapped []string
	for _, c := range r.Changes {
		if c.Company == "" {
			unmapped = append(unmapped, changeLine(&c))
			continue
		}
		if _, ok := byCompany[c.Company]; !ok {
			order = append(order, c.Company)
		}
		byCompany[c.Company] = append(byCompany[c.Company], c)
	}
	sort.SliceStable(order, func(i, j int) bool {
		return len(byCompany[order[i]]) > len(byCompany[order[j]])
	})
	if len(unmapped) > 0 {
		b.WriteString("**未映射变动**（可能是未上市公司，或映射表遗漏）\n")
		for _, l := range unmapped {
			fmt.Fprintf(&b, "- %s\n", l)
		}
		b.WriteString("\n")
	}
	for _, company := range order {
		cs := byCompany[company]
		multi := ""
		if len(cs) >= 2 {
			multi = "（多款齐升）"
		}
		code := ""
		if cs[0].Code != "" {
			code = fmt.Sprintf(" · %s", cs[0].Code)
		}
		fmt.Fprintf(&b, "### %s%s%s\n", company, code, multi)
		for _, c := range cs {
			b.WriteString("- " + changeLine(&c) + "\n")
		}
		b.WriteString("\n")
	}
	if len(r.Unmapped) > 0 {
		b.WriteString("**未映射新进**（待补充映射表）\n")
		for _, u := range r.Unmapped {
			fmt.Fprintf(&b, "- %s\n", u)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func changeLine(c *analyze.Change) string {
	chart := fetch.ChartNames[c.Chart]
	arrow := fmt.Sprintf("第%d→第%d", c.FromRank, c.ToRank)
	switch c.Kind {
	case analyze.KindNew:
		arrow = fmt.Sprintf("新进第%d", c.ToRank)
	case analyze.KindTopBand:
		if c.ToRank == 0 {
			arrow = fmt.Sprintf("跌出前%d", c.FromRank)
		} else if c.FromRank == 0 {
			arrow = fmt.Sprintf("新进第%d", c.ToRank)
		}
	}
	var parts []string
	parts = append(parts, fmt.Sprintf("**%s**：%s %s", c.Game, chart, arrow))
	if c.Kind == analyze.KindRise {
		parts = append(parts, fmt.Sprintf("↑%d名", c.FromRank-c.ToRank))
	}
	parts = append(parts, kindLabel[c.Kind])
	if c.Estimate != "" {
		parts = append(parts, c.Estimate)
	}
	return strings.Join(parts, " · ")
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
