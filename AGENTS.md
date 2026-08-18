# AGENTS.md — 本仓库对 AI 助手的工作要求

## 1. 每次改完代码，必须依次完成以下检查（禁止跳过）

```bash
source ~/.g/env                  # go / golangci-lint 由 ~/.g 管理，先加载环境
gofmt -w cmd internal            # 1. 格式化（改完先跑）
go vet ./...                     # 2. 静态检查
golangci-lint run ./...          # 3. lint（配置见 .golangci.yml，必须 0 issues）
go test ./...                    # 4. 测试（命令见第 3 节）
```

- 若 `golangci-lint run ./...` 有输出，必须修复到 **0 issues** 才能收尾
- 若改动引入新测试文件，同样必须通过第 3 节中的全部测试

## 2. Git commit message 格式

首行：`[改动类型](改动核心模块): 细节描述`

**改动类型**（常用）：`feat` `fix` `perf` `chore` `test` `refactor` `docs` `style` `build` `revert`

**核心模块**：`fetch`（抓取）`mapping`（映射表）`store`（存储）`analyze`（分析）`report`（报告）`notify`（通知）`config`（配置）`cli`（命令行）`tests`（测试）

**规则**：
- 首行简洁，说清楚"改了什么、为什么"
- 单次改动内容较多时，首行之后空一行，用「- 」短横线分点列出

示例：

```
[feat](notify): 企业微信推送支持超长截断与基线确认消息

- 分地区消息按 4096 字节上限按行截断，注明省略条数
- 首次运行推送「基线已建立」确认消息
- 去重指纹加入消息类型，避免安静/基线/报告三种消息互相误判
```

```
[fix](analyze): 修复同一游戏同时上升且进入 Top10 时重复上报
```

## 3. Go 测试命令（以本仓库现有测试为准）

**一键完整校验**（构建 + 格式化检查 + vet + lint + 全部测试）：

```bash
source ~/.g/env && gofmt -w cmd internal && go build ./... && go vet ./... && golangci-lint run ./... && go test ./...
```

**常用测试命令**：

| 目的 | 命令 |
|---|---|
| 运行全部测试 | `go test ./...` |
| 运行指定包 | `go test ./internal/analyze/` |
| 运行单个用例（带详细输出） | `go test -run TestAnalyzeCN -v ./internal/analyze/` |
| 绕过缓存强制重跑 | `go test -count=1 ./...` |
| 列出包内全部用例 | `go test -list . ./internal/mapping/` |

**现有测试清单**（3 个包，12 个用例）：

| 包 | 用例 | 覆盖内容 |
|---|---|---|
| `internal/analyze` | `TestAnalyzeCN` | 新进前50 / 上升≥30 / Top10 变动 / 跌出 Top10 / 免费榜 / 未映射新进 / 中国区流水估算 |
| `internal/analyze` | `TestNoDuplicateChange` | 上升 + 进 Top10 只报一条（回归） |
| `internal/analyze` | `TestAnalyzeFirstRun` | 首次运行（无历史）不产出变化 |
| `internal/analyze` | `TestCrossRegion` | 同一 app_id 多地区同时变化的跨区信号 |
| `internal/mapping` | `TestMatchByID` | app_id 匹配（含多 id 条目） |
| `internal/mapping` | `TestMatchByNameExact` | 精确名匹配 |
| `internal/mapping` | `TestMatchByNameContains` | 包含名匹配 |
| `internal/mapping` | `TestMatchPriorityIDOverName` | id 优先级高于名字 |
| `internal/mapping` | `TestLoadErrors` | 缺 company 的条目报错 |
| `internal/fetch` | `TestParseList` | 榜单多条目 JSON 解析 |
| `internal/fetch` | `TestParseSingleEntry` | 单条目（entry 为对象）解析 |
| `internal/fetch` | `TestParseEmpty` | 空响应 / 非法 JSON 报错 |
