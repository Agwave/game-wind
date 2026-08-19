// gamewind：A股/港股游戏公司观察工具。
//
// 用法：
//
//	gamewind run                 抓取四地区榜单 → 分析 → 生成报告 → 企业微信推送
//	gamewind run --dry-run       同上但不推送
//	gamewind run --date 2026-08-17  用历史日期重跑（对比该日期之前的最近快照）
//	gamewind fetch               只抓取并入库，不分析不推送
//	gamewind report [--date]     用已有快照生成报告（可推送）
//	gamewind list                列出已有快照
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"game-wind/internal/analyze"
	"game-wind/internal/config"
	"game-wind/internal/fetch"
	"game-wind/internal/mapping"
	"game-wind/internal/notify"
	"game-wind/internal/report"
	"game-wind/internal/store"
)

const (
	configPath     = "config.yaml"
	localCfgPath   = "config.local.yaml"
	gamesFile      = "data/games.yaml"
	defaultDataDir = "data"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func usage() {
	fmt.Fprintln(os.Stderr, `gamewind — A股/港股游戏公司观察工具

用法:
  gamewind run                 抓取四地区榜单 → 分析 → 报告 → 推送
  gamewind fetch               只抓取入库
  gamewind report              用已有快照生成报告（可推送）
  gamewind list                列出已有快照

选项:
  --dry-run                    不推送企业微信
  --date YYYY-MM-DD            指定日期（run/report）
  --data-dir DIR               数据目录（默认 data/）
  --games FILE                 映射表路径（默认 data/games.yaml）`)
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	cmd, opts := args[0], parseOpts(args[1:])
	switch cmd {
	case "run":
		return cmdRun(opts)
	case "fetch":
		return cmdFetch(opts)
	case "report":
		return cmdReport(opts)
	case "list":
		return cmdList(opts)
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", cmd)
		usage()
		return 2
	}
}

// parseOpts 解析 --key value / --key=value；布尔开关 --dry-run 记为空值。
func parseOpts(args []string) map[string]string {
	opts := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			continue
		}
		key := strings.TrimPrefix(a, "--")
		if eq := strings.Index(key, "="); eq >= 0 {
			opts[key[:eq]] = key[eq+1:]
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			opts[key] = args[i+1]
			i++
		} else {
			opts[key] = ""
		}
	}
	return opts
}

func cmdRun(opts map[string]string) int {
	cfg, tab, err := loadBase(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	date := opts["date"]
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	snap, err := fetchAll(ctx, cfg, date)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir := dataDir(opts)
	if err := snap.Save(dir); err != nil {
		fmt.Fprintln(os.Stderr, "保存快照失败:", err)
		return 1
	}
	prev, err := store.LatestBefore(dir, date)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return analyzeReportAndNotify(cfg, tab, snap, prev, date, opts)
}

func cmdFetch(opts map[string]string) int {
	cfg, _, err := loadBase(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	date := opts["date"]
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	snap, err := fetchAll(ctx, cfg, date)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := snap.Save(dataDir(opts)); err != nil {
		fmt.Fprintln(os.Stderr, "保存快照失败:", err)
		return 1
	}
	fmt.Printf("已保存快照 %s：%d 个地区，%d 个 feed 失败\n", date, len(snap.Charts), len(snap.Failed))
	for _, f := range snap.Failed {
		fmt.Println(" 失败:", f)
	}
	return 0
}

func cmdReport(opts map[string]string) int {
	cfg, tab, err := loadBase(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir := dataDir(opts)
	date := opts["date"]
	if date == "" {
		dates, err := store.ListDates(dir)
		if err != nil || len(dates) == 0 {
			fmt.Fprintln(os.Stderr, "没有可用快照，请先运行 gamewind run / fetch")
			return 1
		}
		date = dates[len(dates)-1]
	}
	snap, err := store.Load(dir, date)
	if err != nil {
		fmt.Fprintln(os.Stderr, "读取快照失败:", err)
		return 1
	}
	prev, err := store.LatestBefore(dir, date)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return analyzeReportAndNotify(cfg, tab, snap, prev, date, opts)
}

func cmdList(opts map[string]string) int {
	dates, err := store.ListDates(dataDir(opts))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, d := range dates {
		fmt.Println(d)
	}
	return 0
}

// loadBase 加载配置与映射表。
func loadBase(opts map[string]string) (*config.Config, *mapping.Table, error) {
	cfg, err := config.Load(configPath, localCfgPath)
	if err != nil {
		return nil, nil, err
	}
	games := opts["games"]
	if games == "" {
		games = gamesFile
	}
	tab, err := mapping.Load(games)
	if err != nil {
		return nil, nil, err
	}
	return cfg, tab, nil
}

func dataDir(opts map[string]string) string {
	if d := opts["data-dir"]; d != "" {
		return d
	}
	return defaultDataDir
}

// fetchAll 并发抓取所有地区 × 两张榜单，组装快照。
func fetchAll(ctx context.Context, cfg *config.Config, date string) (*store.Snapshot, error) {
	fc := fetch.New(cfg.Limit)
	snap := &store.Snapshot{Date: date, FetchedAt: time.Now(), Charts: map[string]*store.Chart{}}
	for cc := range cfg.Regions {
		snap.Charts[cc] = &store.Chart{}
	}
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed []string
	)
	for cc := range cfg.Regions {
		for _, chart := range []string{fetch.ChartGrossing, fetch.ChartFree} {
			wg.Add(1)
			go func(cc, chart string) {
				defer wg.Done()
				entries, err := fc.FetchChart(ctx, cc, chart)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					failed = append(failed, cc+"/"+chart)
					fmt.Fprintf(os.Stderr, "⚠️ %s/%s: %v\n", cc, chart, err)
					return
				}
				if chart == fetch.ChartGrossing {
					snap.Charts[cc].TopGrossing = entries
				} else {
					snap.Charts[cc].TopFree = entries
				}
			}(cc, chart)
		}
	}
	wg.Wait()
	sort.Strings(failed)
	snap.Failed = failed
	return snap, nil
}

// analyzeReportAndNotify 分析 → 生成报告 → 落盘 → 推送（按选项）。
func analyzeReportAndNotify(cfg *config.Config, tab *mapping.Table, snap, prev *store.Snapshot, date string, opts map[string]string) int {
	results := map[string]*analyze.Result{}
	for cc, rc := range cfg.Regions {
		results[cc] = analyze.Analyze(prev, snap, cc, tab, rc)
	}
	cross := analyze.CrossRegion(results)
	baseline := prev == nil
	md := report.Build(date, snap, prev, tab, results, cross, snap.Failed, baseline)

	dir := dataDir(opts)
	if err := os.MkdirAll(dir+"/reports", 0o755); err == nil {
		_ = os.WriteFile(dir+"/reports/"+date+".md", []byte(md), 0o644)
	}
	fmt.Println(md)

	hasChanges := false
	for _, r := range results {
		if len(r.Changes) > 0 || len(r.Unmapped) > 0 {
			hasChanges = true
			break
		}
	}

	// 推送：基线日/有变化/配置了安静也推送 三种情况才推送
	if _, dry := opts["dry-run"]; !dry && cfg.Notify.WebhookURL != "" {
		if !hasChanges && !cfg.Notify.NotifyWhenQuiet && !baseline {
			fmt.Println("ℹ️ 今日无重大变化，不推送（notify_when_quiet=false）")
		} else {
			if err := notifyAll(cfg, date, hasChanges, baseline, md, dir); err != nil {
				fmt.Fprintln(os.Stderr, "推送失败:", err)
				return 1
			}
		}
	}
	return 0
}

// notifyAll 组装并推送企业微信消息；用 state.json 防同一天重复推送。
func notifyAll(cfg *config.Config, date string, hasChanges, baseline bool, md, dir string) error {
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
		// 推送完整四段报告（游戏公司+总结+补充），超过 MaxBytes 按行截断，完整版见落盘文件
		var err error
		msgs, err = notify.BuildMessages("", md, footer)
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
