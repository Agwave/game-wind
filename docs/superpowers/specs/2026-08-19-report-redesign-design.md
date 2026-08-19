# 报告新格式（按公司分组）设计

日期：2026-08-19 · 状态：已确认方案 A

## 1. 背景与目标

当前报告按「地区 → 公司」组织：每个地区下列出有重大变化的公司及其游戏。用户希望改为「按 yaml 中全部公司分组」的形态，让一份报告既能看清"每家公司目前有哪些游戏上榜、排名与变动"，又能快速定位"哪些变动值得关注"。

目标输出四段结构：

1. **标题**（保留现有标题行 + 数据源说明 + 抓取状态）
2. **游戏公司**（核心段）：按 `data/games.yaml` 的全部公司分段，列出该公司各榜单的前 5 款游戏及排名变动；无上榜则说明"该公司目前无上榜"
3. **总结**：哪些公司/游戏的变动值得关注（沿用现有阈值：畅销新进前50 / 上升≥30 / Top10 内变动 / 免费新进前20 / 上升≥30）
4. **补充**：未映射游戏有明显变动的列出来供确认；命中"无关发行商名单"的游戏归为无关公司，其变动不再单独提及

已确认的两项设计决定：

- 「无关公司」判定：在 games.yaml 维护一份**无关发行商名单**，按 App Store 开发者账号名（Artist）子串匹配（不区分大小写），命中即无关
- 旧的「跨区信号」并入「总结」段；「抓取状态」保留在标题下

## 2. 现状

- `report.Build(date, results, cross, failed, baseline)`：按地区渲染（`RegionSection`），输入是 analyze 的分析结果，**没有当前/历史快照与映射表**，无法枚举"每公司 × 每榜单 top5 排名"
- `analyze.Result.Unmapped []string`：只记录未映射**新进**游戏的字符串，未映射的上升/Top10 变动混在 `Changes`（Company=""）里
- `mapping.Table`：只有 `byID/byName/byContains` 索引，**没有公司列表**，也没有无关发行商名单
- `notify.BuildMessages(header, sections map[cc]string, footer)`：按地区拆多条推送
- 现有测试：`analyze_test`（6 个用例，断言 `len(res.Unmapped)` 等）、`mapping_test`、`fetch_test`；report 包暂无测试

## 3. 新报告最终形态（示例）

```markdown
# 📊 游戏公司观察日报 2026-08-19
> 数据源：iOS App Store 畅销榜/免费榜（游戏分类 · 前100）· 与最近一次快照对比
> ⚠️ 抓取状态：韩国区/免费榜 抓取失败（已重试）        ← 仅失败时出现

## 一、游戏公司（14 家）

### 腾讯控股 · 0700.HK
- 🇨🇳 畅销榜：王者荣耀 第2(↑1) · 金铲铲之战 第6(↓2) · 三角洲行动 第9(新) · 和平精英 第11(—)
- 🇨🇳 免费榜：三角洲行动 第7(↑3) · …
- 🇺🇸 畅销榜：Delta Force 第23(↓5)

### 电魂网络 · 603258.SH
- 该公司目前无上榜

### …其余公司同规则（空榜单行不显示）…
```

```markdown
## 二、总结

值得关注：
- 腾讯控股（2款）：王者荣耀 畅销榜第2（↑1）· 估算400万+/日；三角洲行动 畅销榜 新进第9
- 网易（1款）：蛋仔派对 畅销榜第8（↑7）
- 完美世界（1款）：异环 Top10变动 第5→第3
- 跨区信号：无尽冬日（世纪华通）中国区/日本区/美国区 同时上榜变化
```

```markdown
## 三、补充

待确认（未映射，可能属于目标公司，请确认）：
- 🇨🇳 畅销榜 第8 灵墟幻想(id:888888888) 新进
- 🇨🇳 畅销榜 第22 山海迷城(id:999999999) 上升↑18

无关公司（确认非目标上市公司，其变动忽略）：
- Nintendo：塞尔达传说 第12 · 宝可梦睡眠 第45
- miHoYo：原神 第9 · 崩坏：星穹铁道 第21
```

## 4. 数据结构变更

### 4.1 mapping 包

- `tableFile` 增加字段：`UnrelatedArtists []string yaml:"unrelated_artists"`
- `Table` 增加字段：
  - `Companies []Company`（保留 yaml 顺序，供报告枚举全部公司）
  - `UnrelatedArtists []string`（规范化：小写、去空格）
- 新增方法：`func (t *Table) IsUnrelated(artist string) bool`
  - artist 为空 → false
  - 小写后对名单做**子串**匹配（如 `Nintendo Co., Ltd.` 命中 `nintendo`）
- `Load` 里填充上述字段

### 4.2 games.yaml

文件顶部（`companies:` 之前）新增顶层段：

```yaml
# 无关发行商名单：确认非目标上市公司的发行商。榜单上命中 Artist（开发者账号名）的游戏
# 视为无关，报告归到「无关公司」，其变动不再单独提及。可用「补充·待确认」持续补充。
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

注意：不要加入 Century（世纪华通出海品牌，属于目标公司）。

### 4.3 analyze 包

- `Result.Unmapped` 由 `[]string` 改为 `[]Change`（结构化，Company 为空、含 Region/Chart/Kind/FromRank/ToRank/Game/AppID）
- `diffChart` 分流规则统一为：
  - 命中目标公司（Company != ""）→ `Changes`
  - 未命中（Company == ""）→ `Unmapped`（新进、上升、Top10 变动、跌出 Top10 都归入）
- `CrossRegion` 逻辑不变（只扫 `Changes`）
- `TestAnalyzeCN`/`TestAnalyzeFirstRun` 中 `len(res.Unmapped)` 断言仍成立（类型变化不影响长度断言）

## 5. report 包重构

### 5.1 新签名与导出函数

```go
func Build(date string, cur, prev *store.Snapshot, tab *mapping.Table,
	results map[string]*analyze.Result, cross []string, failed []string, baseline bool) string

func CompanySection(cur, prev *store.Snapshot, tab *mapping.Table) string      // 一、游戏公司
func SummarySection(results map[string]*analyze.Result, cross []string, baseline bool) string // 二、总结
func PendingSection(results map[string]*analyze.Result) string                 // 三、补充·待确认
func UnrelatedSection(cur *store.Snapshot, tab *mapping.Table) string          // 三、补充·无关公司
func SupplementSection(cur *store.Snapshot, tab *mapping.Table, results map[string]*analyze.Result) string // 三、补充 = 待确认 + 无关公司
```

`Build` = 标题行（+baseline 提示 + 抓取状态） + `CompanySection` + `SummarySection` + `SupplementSection`。

把各段拆成导出函数，是为了让 `main` 的推送可以复用 `SummarySection` + `PendingSection` 作为推送正文（推送不带「无关公司」噪音，完整三段在落盘报告里）。

### 5.2 一、游戏公司（核心段）

数据组装：

- 固定顺序：`regionOrder = [cn, us, jp, kr]`；`chartOrder = [topgrossingapplications, topfreeapplications]`
- 对每个地区 × 榜单，扫描 `cur.Charts[cc]` 的条目：`m := tab.Match(appID, name)`；`m != nil` 时归到 `m.Company` 名下，记录 `(game=规范名 m.Game, curRank, region, chart)`
- prev 侧：对每个地区 × 榜单建 `map[appID]int`（prev 排名），用于算变动
- 展示规则：
  - 按 `tab.Companies`（yaml 顺序）逐公司输出 `### 公司 · 代码`
  - 按地区 > 榜单顺序，列出该公司在该榜单上排名前 5 的游戏（按 curRank 升序截取前 5）
  - 变动标注：无 prev 排名 → `新`；相同 → `—`；上升 → `↑n`；下降 → `↓n`
  - 空榜单行不显示（只显示该公司有上榜的榜单）
  - 8 个榜单全部无该公司的游戏 → 该行改为 `- 该公司目前无上榜`
  - 游戏名用映射表规范名（names[0]），跨地区统一展示

### 5.3 二、总结

- 从 `results` 收集 `Changes`（Company != ""）按公司分组，公司按变动数降序
- 每条变动渲染为：`<规范名> <榜单名>第<ToRank>（<变动>）[ · 估算]`，其中变动 = 新进 / ↑n / Top10变动（第X→第Y）/ 跌出前X
- 跨区信号 `cross` 追加在末尾（`- 跨区信号：…`）
- `baseline == true`：只输出 `ℹ️ 首次运行，已建立基线快照，明日开始对比变化。`
- 无任何变动：输出 `今日无重大变化`

### 5.4 三、补充

- **待确认（`PendingSection`）**：遍历 `results` 的 `Unmapped`（结构化），按地区渲染为 `- 🇨🇳 畅销榜 第8 游戏名(id:…) 新进/上升↑n/Top10变动`
- **无关公司（`UnrelatedSection`）**：扫描 `cur` 各榜单，`tab.Match == nil` 且 `tab.IsUnrelated(artist)` 的条目，按 artist 分组；每个 artist 列出排名前 5 的游戏（`<artist>：<游戏> 第X · …`）；无命中则不输出该小节

## 6. main / notify 联动

- `analyzeReportAndNotify`：`report.Build(date, cur, prev, tab, results, cross, snap.Failed, baseline)`
- `hasChanges` 判定扩展为：任一地区 `len(Changes) > 0 || len(Unmapped) > 0`（未映射明显变动也应触发推送）
- `notifyAll`：
  - 有变化时推送正文 = `SummarySection` + `PendingSection`（待确认），由 `notify.BuildMessages` 截断；「无关公司」只出现在落盘报告，不进推送
  - 底部 footer 保留 `完整报告：data/reports/<date>.md`
  - baseline / 安静 文案不变
- `notify.BuildMessages` 签名简化为 `(header, body string, footer string) ([]string, error)`，单条 body 超过 MaxBytes 按行截断（复用现有 `truncateSection` 逻辑，改名为 `truncateBody`）

## 7. 边界情况

| 场景 | 行为 |
|---|---|
| 首次运行（prev == nil，baseline） | 游戏公司段显示当前排名（变动标 `—`）；总结段输出基线提示；补充段只列「无关公司」，「待确认」为空 |
| 公司无任何游戏上榜 | 该公司段落显示「该公司目前无上榜」 |
| 游戏在 prev 有、cur 无（跌出前100） | 不进游戏公司段（不在当前榜单）；跌出 Top10 由总结段 Top10 变动覆盖 |
| 多 id / 多地区同名游戏 | 按 app_id 匹配各自归属，规范名统一展示 |
| artist 为空 | `IsUnrelated` 返回 false |
| 无关名单命中（如 `Nintendo Co., Ltd.`） | 小写子串匹配命中 `nintendo` |
| 抓取失败 feed | 标题下逐行列 `⚠️ <地区>/<榜单> 抓取失败（已重试）` |

## 8. 测试计划

- **mapping**：新增 `TestUnrelatedArtists`——加载 `unrelated_artists`、`IsUnrelated` 子串/大小写/空 artist 判定；`Table.Companies` 保留 yaml 顺序
- **analyze**：更新 `Unmapped` 类型相关断言；新增断言——未映射上升也进 `Unmapped`、已映射变化不进 `Unmapped`
- **report（新增 `report_test.go`）**：
  - `TestCompanySection`：多地区多榜单 top5 截断、变动箭头（↑/↓/新/—）、「该公司目前无上榜」、榜单顺序（中国>美国>日本>韩国、畅销>免费）
  - `TestSummarySection`：有变化/无变化/基线三种输出；跨区信号追加
  - `TestSupplementSection`：待确认结构化渲染、无关公司按 artist 分组
- **整体**：按 CLAUDE.md 依次跑 `gofmt -w cmd internal` / `go build ./...` / `go vet ./...` / `golangci-lint run ./...`（0 issues）/ `go test ./...`

## 9. 范围说明

- 本次只改报告输出形态与相关数据结构；不改变抓取、存储、映射匹配逻辑
- 无关发行商名单先种子化常见大厂，之后可通过「补充·待确认」反馈持续维护
- 推送正文采用「总结 + 补充」精简版（不推全 14 家公司列表），完整报告落盘 `data/reports/<date>.md`
