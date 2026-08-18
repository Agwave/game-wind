# game-wind · A股/港股游戏公司观察工具

[中文](README.md) | [English](README.en.md)

每天自动抓取 **中、美、日、韩** 四个地区 iOS App Store 的游戏畅销榜与免费榜，对比历史快照，
筛选出值得关注的重大变化（某公司关键游戏流水跳涨、新游戏爆发、多地区齐升等），生成 Markdown 报告并推送到企业微信。

> **定位**：只看 **A 股 / 港股上市游戏公司**（含其出海游戏）。未上市公司（米哈游、莉莉丝等）与外国公司游戏不跟踪，
> 但会出现在报告的「未映射」清单里供参考。

## 目录

- [功能特性](#功能特性)
- [快速开始](#快速开始)
- [命令参考](#命令参考)
- [配置](#配置)
- [定时任务（WSL2）](#定时任务wsl2)
- [数据源](#数据源)
- [流水估算说明](#流水估算说明)
- [常见问题](#常见问题)
- [许可证](#许可证)

## 功能特性

- 📡 **每日抓取**：4 地区 × 2 榜单（畅销榜/免费榜）各前 100 名（苹果官方 RSS，免费无需鉴权）
- 📊 **重大变化判定**（阈值可在 `config.yaml` 按地区调整）：
  - 畅销榜：新进前 50 / 排名上升 ≥30 名 / Top10 内任何变动
  - 免费榜：新进前 20 / 排名上升 ≥30 名
- 🏆 **跨区信号**：同一游戏在多个地区同时上榜变化
- 💰 **流水估算**：中国区畅销榜排名 → 日流水量级区间（仅供参考）
- 📝 **报告落盘**：`data/reports/YYYY-MM-DD.md`，快照存 `data/YYYY-MM-DD.json`
- 📲 **企业微信推送**：按地区分条、超长自动截断；无重大变化默认不打扰；同一天不重复推送

## 快速开始

```bash
# 1. 环境（Go 1.22，通过 g 管理）
g install 1.22.12 && g use 1.22.12

# 2. 构建
go build -o bin/gamewind ./cmd/gamewind

# 3. 首次运行（建立基线，不推送）
./bin/gamewind run --dry-run

# 4. 手动触发（抓取→分析→报告→推送）
./bin/gamewind run
```

## 命令参考

| 命令 | 说明 |
|---|---|
| `gamewind run [--dry-run] [--date YYYY-MM-DD]` | 抓取 → 分析 → 报告 → 推送 |
| `gamewind fetch` | 只抓取入库 |
| `gamewind report [--date] [--dry-run]` | 用已有快照生成报告/推送 |
| `gamewind list` | 列出已有快照 |
| `--data-dir DIR` | 数据目录（默认 `data/`） |
| `--games FILE` | 映射表路径（默认 `data/games.yaml`） |

## 配置

### 企业微信通知

1. 手机/电脑端注册[企业微信](https://work.weixin.qq.com/)（个人可免费注册）
2. 创建一个只有自己的群 → 群设置 → 群机器人 → 添加机器人 → 复制 webhook 地址
3. 写入 `config.local.yaml`（已被 gitignore，不会入库）：

```yaml
notify:
  webhook_url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
```

### 判定阈值

`config.yaml`（模板，入库）— 各地区阈值与抓取条数：

```yaml
limit: 100                     # 每榜单抓取条数（上限 100）
regions:
  cn:                          # 可增删地区（苹果支持任意地区码）
    grossing_new_top: 50       # 畅销榜新进前 N
    grossing_rise: 30          # 畅销榜上升 ≥ N 名
    grossing_top_band: 10      # 畅销榜前 N 内变动
    free_new_top: 20           # 免费榜新进前 N
    free_rise: 30              # 免费榜上升 ≥ N 名
notify:
  notify_when_quiet: false     # 无重大变化时是否仍推送
```

`config.local.yaml`（本地，不入库）覆盖 `config.yaml` 同名键，用于存放 webhook 等敏感配置。

### 游戏→公司映射表（data/games.yaml）

匹配优先级：**app_id > 精确名 > 包含名**。app_id 跨地区稳定（同一游戏在美日韩是同一个 id），
所以映射表优先用 id，标题不同的多语言版本用 `names` / `contains` 补充：

```yaml
companies:
  - company: 世纪华通
    code: "002602.SZ"
    market: A股
    games:
      - ids: [6478492012, 6443575749]     # 无尽冬日(中国区) / Whiteout Survival(海外)
        names: [无尽冬日, Whiteout Survival, "WOS: 화이트아웃 서바이벌"]
      - names: [Kingshot]                  # 还没有 id 时用名字匹配
        contains: [Kingshot]
```

维护要点：

- 报告的「**未映射新进**」清单会给出 app_id 和标题，把 id 加进对应游戏即可自动生效
- 名字里含 `: `（冒号+空格）时**必须加引号**，否则 YAML 解析报错
- 只收录 A 股/港股上市公司

## 定时任务（WSL2）

每日 9:00 自动运行（crontab 示例）：

```
0 9 * * * cd ~/ai-project/game-wind && ./bin/gamewind run >> logs/cron.log 2>&1
```

注意：

- **WSL 关闭时不会执行**。保持 WSL 常开（或另配 Windows 任务计划唤醒）
- WSL 里 cron 服务需要手动启动（每次开机后）：`sudo service cron start`
  （或在 `/etc/wsl.conf` 里 `[boot] systemd=true`，之后 `sudo systemctl enable --now cron`）

## 数据源

- 苹果官方 RSS：`https://itunes.apple.com/{cc}/rss/{topgrossingapplications|topfreeapplications}/genre=6014/limit=100/json`
- `genre=6014` 即「游戏」分类；榜单约每天更新数次，本工具每日固定时间快照一次
- 备用源 `rss.applemarketingtools.com` 已迁移至新域名且 v2 接口返回 404，不采用

## 流水估算说明

中国区畅销榜排名 → 日流水估算区间（行业常用量级，仅供参考）：
Top1 ≈ 400万+/日 · 2-3 ≈ 250-400万 · 4-10 ≈ 100-250万 · 11-20 ≈ 50-100万 · 21-50 ≈ 20-50万 · 51-100 ≈ 10-20万。
海外榜单不做流水估算。

## 常见问题

- **推送超长**：企业微信单条 markdown 上限 4096 字节，超长会自动按行截断并注明省略条数，完整报告在 `data/reports/`
- **重复推送**：同一天相同内容不会推送第二次（`data/state.json` 记录指纹）
- **首次运行**：无历史对比，只建立基线并推送一条确认消息
- **抓取失败**：单个榜单失败重试 2 次，仍失败会在报告「抓取状态」注明，不影响其他榜单

## 许可证

[MIT](LICENSE) © 2026 chenyinbo
