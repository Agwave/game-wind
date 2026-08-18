# game-wind · A-Share/HK-Listed Game Company Monitor

[中文](README.md) | [English](README.en.md)

Automatically fetches the iOS App Store **top-grossing and top-free game charts** from **China, US, Japan, and Korea**
every day, compares them against historical snapshots, and flags noteworthy movements — a key game's revenue surging,
a new game breaking out, or a title rising across multiple regions at once. Generates a Markdown report and pushes it to WeCom (WeChat Work).

> **Scope**: Only tracks **A-share / HK-listed Chinese game companies** (including their overseas games).
> Unlisted companies (miHoYo, Lilith, etc.) and foreign publishers are not tracked, but appear in the
> report's "unmapped" list for reference.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Command Reference](#command-reference)
- [Configuration](#configuration)
- [Scheduling (WSL2)](#scheduling-wsl2)
- [Data Source](#data-source)
- [Revenue Estimates](#revenue-estimates)
- [FAQ](#faq)
- [License](#license)

## Features

- 📡 **Daily fetch**: 4 regions × 2 charts (top-grossing / top-free), top 100 each (Apple's official RSS, free, no auth)
- 📊 **Major-change detection** (thresholds configurable per region in `config.yaml`):
  - Top-grossing: new entry in top 50 / rank up ≥30 / any change within top 10
  - Top-free: new entry in top 20 / rank up ≥30
- 🏆 **Cross-region signals**: the same game changing across multiple regions at once
- 💰 **Revenue estimates**: CN top-grossing rank → rough daily revenue band (reference only)
- 📝 **Reports on disk**: `data/reports/YYYY-MM-DD.md`, snapshots at `data/YYYY-MM-DD.json`
- 📲 **WeCom push**: one message per region, auto-truncated when too long; stays quiet by default when nothing major happens; no duplicate pushes on the same day

## Quick Start

```bash
# 1. Toolchain (Go 1.22, managed via g)
g install 1.22.12 && g use 1.22.12

# 2. Build
go build -o bin/gamewind ./cmd/gamewind

# 3. First run (creates a baseline, no push)
./bin/gamewind run --dry-run

# 4. Manual run (fetch → analyze → report → push)
./bin/gamewind run
```

## Command Reference

| Command | Description |
|---|---|
| `gamewind run [--dry-run] [--date YYYY-MM-DD]` | Fetch → analyze → report → push |
| `gamewind fetch` | Fetch and store only |
| `gamewind report [--date] [--dry-run]` | Generate report / push from an existing snapshot |
| `gamewind list` | List existing snapshots |
| `--data-dir DIR` | Data directory (default `data/`) |
| `--games FILE` | Mapping table path (default `data/games.yaml`) |

## Configuration

### WeCom notifications

1. Register [WeCom (WeChat Work)](https://work.weixin.qq.com/) (free for personal use)
2. Create a group with only yourself → group settings → group bot → add bot → copy the webhook URL
3. Put it in `config.local.yaml` (gitignored, never committed):

```yaml
notify:
  webhook_url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
```

### Detection thresholds

`config.yaml` (template, tracked) — per-region thresholds and fetch limit:

```yaml
limit: 100                     # entries per chart (max 100)
regions:
  cn:                          # add/remove regions freely (any Apple country code)
    grossing_new_top: 50       # new entry into top N of top-grossing
    grossing_rise: 30          # rank up ≥ N on top-grossing
    grossing_top_band: 10      # any change within top N
    free_new_top: 20           # new entry into top N of top-free
    free_rise: 30              # rank up ≥ N on top-free
notify:
  notify_when_quiet: false     # push even when nothing major changed
```

`config.local.yaml` (local, gitignored) overrides keys in `config.yaml` — used for sensitive settings like the webhook.

### Game → company mapping (data/games.yaml)

Match priority: **app_id > exact name > substring**. App IDs are stable across regions (the same game
shares one ID in US/JP/KR), so prefer IDs; use `names` / `contains` for localized titles:

```yaml
companies:
  - company: Century Huatong (世纪华通)
    code: "002602.SZ"
    market: A-share
    games:
      - ids: [6478492012, 6443575749]     # 无尽冬日 (CN) / Whiteout Survival (overseas)
        names: [无尽冬日, Whiteout Survival, "WOS: 화이트아웃 서바이벌"]
      - names: [Kingshot]                  # match by name when the ID is unknown
        contains: [Kingshot]
```

Maintenance tips:

- The report's "unmapped new entries" list includes app IDs — add them to the table and matching takes effect immediately
- Names containing `: ` (colon + space) **must be quoted**, or YAML parsing fails
- Only track A-share / HK-listed companies

## Scheduling (WSL2)

Runs daily at 9:00 (crontab example):

```
0 9 * * * cd ~/ai-project/game-wind && ./bin/gamewind run >> logs/cron.log 2>&1
```

Notes:

- **The job does not run while WSL is shut down**. Keep WSL running (or schedule a Windows Task to wake it)
- The cron service in WSL must be started manually after each boot: `sudo service cron start`
  (or enable systemd via `[boot] systemd=true` in `/etc/wsl.conf`, then `sudo systemctl enable --now cron`)

## Data Source

- Apple's official RSS: `https://itunes.apple.com/{cc}/rss/{topgrossingapplications|topfreeapplications}/genre=6014/limit=100/json`
- `genre=6014` is the "Games" category; charts refresh several times a day, this tool snapshots once daily at a fixed time
- The old backup source `rss.applemarketingtools.com` was migrated to a new domain whose v2 API returns 404 — not used

## Revenue Estimates

CN top-grossing rank → estimated daily revenue band (industry-scale reference only):
Top1 ≈ ¥4M+/day · 2-3 ≈ ¥2.5-4M · 4-10 ≈ ¥1-2.5M · 11-20 ≈ ¥0.5-1M · 21-50 ≈ ¥0.2-0.5M · 51-100 ≈ ¥0.1-0.2M.
No estimates for overseas charts.

## FAQ

- **Push too long**: WeCom markdown messages are capped at 4096 bytes; oversized content is truncated line-by-line with a note about skipped entries — the full report is always saved to `data/reports/`
- **Duplicate pushes**: identical content on the same day is not pushed twice (fingerprint stored in `data/state.json`)
- **First run**: no history to compare, only a baseline is created plus a confirmation message
- **Fetch failures**: each chart retries twice; persistent failures are noted in the report's fetch status without affecting other charts

## License

[MIT](LICENSE) © 2026 chenyinbo
