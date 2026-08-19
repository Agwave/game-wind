# 更新 data/games.yaml —— 给大模型的指令与规范

你是 game-wind 游戏公司观察工具的数据维护员。把本文件连同 `data/games.yaml` 一并交给你，你要根据最新 App Store 榜单更新映射表，使它与真实榜单保持同步。

## 一、先读这几份东西

1. **data/games.yaml** —— 当前映射表。先通读，理解结构与已有条目，再动手。
2. **最新榜单快照** —— `data/` 下日期最新的 `*.json`。
   - 若不存在，或日期明显过期（非最近 1~2 天），且环境可联网：执行
     ```bash
     source ~/.g/env && go run ./cmd/gamewind fetch
     ```
     会抓取四地区（cn/us/jp/kr）畅销/免费 Top100，并存为 `data/<今天日期>.json`。
3. 需要时**联网核实**游戏归属。

## 二、快照 JSON 结构

`data/<日期>.json` 形如：

```json
{ "date": "2026-08-19",
  "charts": {
    "cn": { "top_grossing": [ { "rank": 1, "app_id": "989673964", "name": "王者荣耀", "artist": "Tencent" }, ... ],
            "top_free": [...] },
    "us": {...}, "jp": {...}, "kr": {...} } }
```

- `app_id`：App Store 应用 id，跨地区稳定，是匹配主键
- `name`：该地区本地化标题（如 `Whiteout Survival` / `ホワイトアウト・サバイバル`）
- `artist`：开发者账号名，辅助判定归属（注意子公司/关联账号，见「五」）

## 三、更新步骤

1. 读取最新快照（见「一」）。
2. 遍历四地区 × 两榜单的条目，与 `games.yaml` 对照，找出：
   - **a. 目标公司新上榜的游戏**（榜单出现但 yaml 未收录）→ 联网核实归属后入库；
   - **b. 已收录游戏的 id / 标题 / 归属有变化** → 修正；
   - **c. 规范名不是中文也不是官方名的** → 按「四·3」优化。
3. 联网核实归属（见「五」）。
4. 按「四」的更新规范修改 `data/games.yaml`。
5. 按「六」校验，必须全部通过。
6. 汇报改动清单（公司 / 游戏 / 新增或修改的 id / 依据）。

## 四、更新规范

1. **只收录 A 股上市公司 + 港股通标的港股。**
   - 港股通：港股公司必须在港股通名单内。当前已收录：腾讯、网易、金山软件、心动公司、哔哩哔哩。中手游、创梦天地、网龙、友谊时光、禅游科技、青瓷游戏**已不在港股通名单**，勿加回。
   - 新增港股公司前，先联网核实其是否在港股通名单（可查沪深交易所港股通标的名单）。
2. **每款游戏一条目**，字段：
   - `ids`：App Store app_id 列表。同一游戏跨地区同一 id 写一个即可；**不同 id 也都要列进同一条目**（例：无尽冬日 cn=`6478492012`、us/kr=`6443575749`）。
   - `names`：各地区标题（用于精确匹配），**第一个即规范名**。
   - `contains`：标题包含该子串（标题带版本后缀等不确定时用）；有 `names` 时可省略。
   - `core: true`：标记公司核心业绩支撑游戏（收入/流水主力，需财报/榜单佐证）。
   - `note`：必要的归属/说明备注。
3. **规范名**（`names[0]`；无 names 时取 `contains[0]`）：
   - 优先用**确定的中文名**；
   - 无确定中文名用**官方名**（如 `Honor of Kings`、`Puzzles & Survival`、`Kingshot`、`Tasty Travels: Merge Game`）；
   - 不要用日/韩文标题当规范名（除非它同时就是官方名且无中文）。
4. **匹配优先级：ids > names 精确 > contains 包含**。新增尽量带 id（id 跨地区稳定，不受本地化标题影响）。
5. 同一游戏海外版是独立 app 时可单独成条（如 三角洲行动 与 三角洲行动国际版），规范名区分开。
6. 更新表头注释「ids/names 均来自 YYYY-MM-DD 真实榜单抓取」中的日期为本次快照日期。
7. 不轻易删除已有条目；确需删/改公司时给出明确理由（如公司被调出港股通、游戏不再属于该公司）。

## 五、归属判定（联网核实）

- 目标公司以 `games.yaml` 现有 companies 为准。
- 判定某游戏属于某公司时：
  - 查官方/新闻确认**发行方**：海外发行品牌如 Level Infinite=腾讯、Century Games=世纪华通、点点互动=世纪华通；
  - 参考 `artist` 字段，但注意**子公司/关联账号**——例：《鹅鸭杀》的 artist 为 `Chengdu Kingsoft…`，实为金山软件（西山居）发行；不能因为 artist 里含 "King" 就当作 King（Candy Crush 发行商）的游戏；
  - 不要因为游戏同名/相似就归给某公司，必须能确认发行归属。
- **无关发行商不入库**：Nintendo、miHoYo、Supercell、Playrix、King、Roblox、Netmarble、Nexon、KRAFTON、KONAMI、Bandai Namco、Square Enix、Capcom、SEGA、Voodoo 等非目标上市公司，即使霸榜也不加入。

## 六、校验（必须全过）

```bash
source ~/.g/env && gofmt -w cmd internal && go build ./... && go vet ./... && golangci-lint run ./... && go test ./...
```

- 期望：lint **0 issues**、全部测试 `ok`
- 额外确认映射表可加载：`go test ./internal/mapping/` 或 `go run ./cmd/gamewind list` 正常输出

## 七、收尾

- 汇报：改了哪些公司/游戏、新增或修改的 id、依据（榜单排名 / 联网来源）。
- 若用户要求提交，按仓库 `CLAUDE.md` 的 commit 格式，如 `[feat](mapping): 新增 XXX（公司）`。
