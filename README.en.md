# game-wind · A-Share/HK-Listed Game Company Monitor

[中文](README.md) | [English](README.en.md)

Automatically fetches the iOS App Store **top-grossing and top-free game charts** from **China, US, Japan, and Korea**
every day, compares them against historical snapshots, and flags noteworthy movements — a key game's revenue surging,
a new game breaking out, or a title rising across multiple regions at once. Generates a Markdown report and pushes it to WeCom (WeChat Work).

> **Scope**: Only tracks **A-share companies and HK-listed companies eligible for Stock Connect (港股通)**
> — including their overseas games. Unlisted companies (miHoYo, Lilith, etc.) and foreign publishers are not tracked;
> games missing from the mapping table are not reported individually — maintain them via the `data/update_game_yaml.md` prompt.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Command Reference](#command-reference)
- [Configuration](#configuration)
- [Scheduling](#scheduling)
- [Data Source](#data-source)
- [Revenue Estimates](#revenue-estimates)
- [FAQ](#faq)
- [License](#license)

## Features

- 📡 **Daily fetch**: 4 regions × 2 charts (top-grossing / top-free), top 100 each (Apple's official RSS, free, no auth)
- 📊 **Major-change detection** (thresholds configurable per region in `config.yaml`):
  - Top-grossing: new entry in top 50 / rank up ≥30 / any change within top 10
  - Top-free: new entry in top 20 / rank up ≥30
- 🏆 **Cross-region signals**: the same game changing across multiple regions at once (shown in the summary section)
- 🗂️ **Three-section report**: title → per-company breakdown (every whitelisted company, top 5 per chart with rank & change ↑↓new—, "not on charts" marked) → summary (notable moves + cross-region signals)
- 💰 **Revenue estimates**: CN top-grossing rank → rough daily revenue band (reference only)
- 📝 **Reports on disk**: `data/reports/YYYY-MM-DD.md`, snapshots at `data/YYYY-MM-DD.json`
- 📲 **WeCom push**: the full report, split across multiple messages line-by-line when too long; stays quiet by default when nothing major happens; no duplicate pushes on the same day

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

- Hand `data/update_game_yaml.md` to an LLM to update the table per the spec (read the latest snapshot → verify ownership → run checks)
- The first entry in each game's `names` is its **canonical name** (shown consistently across regions): prefer Chinese, otherwise the official name
- `core: true` marks a company's key revenue-driving games
- Names containing `: ` (colon + space) **must be quoted**, or YAML parsing fails
- Only track A-share companies and HK-listed companies eligible for Stock Connect

## Scheduling

The tool has **no built-in scheduler** — an external timer triggers `gamewind run` (fetch → analyze → report → push to WeCom). First build the binary and create the log directory:

```bash
go build -o bin/gamewind ./cmd/gamewind
mkdir -p logs
```

### Native Linux

**Recommended: systemd timer** (supports `Persistent` catch-up for missed runs; logs go to journald):

`/etc/systemd/system/gamewind.service`:

```ini
[Unit]
Description=game-wind daily run
After=network-online.target

[Service]
Type=oneshot
WorkingDirectory=<absolute project path>
ExecStart=<absolute project path>/bin/gamewind run
```

`/etc/systemd/system/gamewind.timer`:

```ini
[Unit]
Description=game-wind daily 09:00

[Timer]
OnCalendar=*-*-* 09:00:00
Persistent=true
RandomizedDelaySec=120

[Install]
WantedBy=timers.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gamewind.timer
systemctl list-timers gamewind.timer   # show next fire time
journalctl -u gamewind.service         # show run logs
```

**Or crontab** (simplest; replace `/path/to/game-wind` with your actual path):

```bash
crontab -e
# daily at 09:00
0 9 * * * cd /path/to/game-wind && ./bin/gamewind run >> logs/cron.log 2>&1
```

**Anacron for laptops that are often off** (catches up on next boot; append to `/etc/anacrontab`):

```
1  5  gamewind.daily  cd /path/to/game-wind && ./bin/gamewind run >> logs/cron.log 2>&1
```

### WSL2

WSL2 is Linux, so the commands above work identically, with two caveats:

1. **Keep cron / systemd resident** (WSL doesn't enable systemd by default):
   - cron: run `sudo service cron start` after each boot
   - or set `[boot] systemd=true` in `/etc/wsl.conf` → restart with `wsl --shutdown` → `sudo systemctl enable --now cron`, then the systemd timer approach above works directly
2. **WSL must be running at the scheduled time**, otherwise it won't fire; systemd's `Persistent=true` catches up on the next start.

To guarantee triggering from the Windows side (even when WSL is off), use Task Scheduler calling:

```
wsl.exe -d <distro> bash -lc "cd /path/to/game-wind && ./bin/gamewind run >> logs/cron.log 2>&1"
```

(Set the trigger to daily 09:00 and tick "run task as soon as possible after a scheduled start is missed".)

## Data Source

- Apple's official RSS: `https://itunes.apple.com/{cc}/rss/{topgrossingapplications|topfreeapplications}/genre=6014/limit=100/json`
- `genre=6014` is the "Games" category; charts refresh several times a day, this tool snapshots once daily at a fixed time
- The old backup source `rss.applemarketingtools.com` was migrated to a new domain whose v2 API returns 404 — not used

## Revenue Estimates

CN top-grossing rank → estimated daily revenue band (industry-scale reference only):
Top1 ≈ ¥4M+/day · 2-3 ≈ ¥2.5-4M · 4-10 ≈ ¥1-2.5M · 11-20 ≈ ¥0.5-1M · 21-50 ≈ ¥0.2-0.5M · 51-100 ≈ ¥0.1-0.2M.
No estimates for overseas charts.

## FAQ

- **Push too long**: WeCom markdown messages are capped at 4096 bytes; oversized content is split across multiple messages line-by-line so nothing is lost — the full report is always saved to `data/reports/`
- **Duplicate pushes**: identical content on the same day is not pushed twice (fingerprint stored in `data/state.json`)
- **First run**: no history to compare, only a baseline is created plus a confirmation message
- **Fetch failures**: each chart retries twice; persistent failures are noted in the report's fetch status without affecting other charts

## License

[MIT](LICENSE) © 2026 chenyinbo
