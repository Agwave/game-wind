#!/usr/bin/env bash
# 每周定时任务示例：用 Claude Code 按 data/update_game_yaml.md 规范更新 data/games.yaml。
# 本示例只负责更新与校验，不含提交推送等本地策略（如需要可自行在 claude 提示词中追加）。
#
# 使用方法：
#   1. 复制为本地脚本：
#        cp scripts/update_games_weekly_example.sh scripts/update_games_weekly.sh
#   2. 把下方「需要填写」的占位符替换成你的实际值
#   3. 加入 crontab（每周日 23:00 示例）：
#        0 23 * * 0 /你的项目绝对路径/scripts/update_games_weekly.sh
#   4. 运行日志：logs/update_games.log；失败时脚本以非零退出码通知 cron
set -euo pipefail

# ===== 需要按本机实际情况填写 =====
PROJECT_DIR=/path/to/game-wind   # 项目绝对路径
CLAUDE_BIN=claude                # claude 命令；若不在 PATH，写完整路径
# 若 go / golangci-lint 不在 PATH（例如通过版本管理器管理），在此加载环境，例如：
# source "$HOME/your-env.sh"
# =================================

LOG="$PROJECT_DIR/logs/update_games.log"
mkdir -p "$PROJECT_DIR/logs"
cd "$PROJECT_DIR"

{
  echo "=== $(date '+%F %T') 开始：claude 更新映射表 ==="
  # 无头模式运行；--dangerously-skip-permissions 用于无人值守时跳过权限询问，
  # 若已在 Claude Code 权限配置中放行所需工具（Bash/Read/Edit/Write/Web 等）可去掉
  "$CLAUDE_BIN" -p --dangerously-skip-permissions '你是 game-wind 的数据维护员。任务：按 data/update_game_yaml.md 更新 data/games.yaml。

步骤：
1. 完整阅读 data/update_game_yaml.md，严格执行其中全部规范：对照最新榜单快照（必要时先 fetch）、联网核实归属、按第四节规范修改 data/games.yaml、跑完第六节校验（lint 必须 0 issues、测试全部 ok）。
2. 若对照榜单后无需任何修改，说明原因即可。

最后用一句话汇报改动内容或结论。'
  echo "=== $(date '+%F %T') claude 结束 ==="
} >> "$LOG" 2>&1
