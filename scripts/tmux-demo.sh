#!/usr/bin/env bash
# tmux-demo.sh — Launch all three tools in a tmux session for visual testing.
# Run from repo root: ./bin/tmux-demo.sh
#
# Layout:
#   ┌──────────────────────┬──────────────────────┐
#   │   media-streams      │   media-calendar      │
#   │   (single shot)      │   (single shot)       │
#   ├──────────────────────┴───────────────────────┤
#   │               arr-feed                        │
#   │              (single shot)                    │
#   └───────────────────────────────────────────────┘

set -euo pipefail

SESSION="calms-toolkit-demo"

cleanup() {
    tmux kill-session -t "$SESSION" 2>/dev/null || true
}
trap cleanup EXIT

cleanup

# Build binaries
cd "$(git rev-parse --show-toplevel 2>/dev/null || echo "$(dirname "$0")/..")"
make -s build 2>/dev/null

BIN="$(pwd)/bin"

tmux new-session -d -s "$SESSION"

# Top-left: media-streams (snapshot mode)
tmux send-keys -t "$SESSION:0" "$BIN/media-streams 2>&1; echo '--- STREAMS DONE ---'" Enter

# Top-right: media-calendar (snapshot mode)
tmux split-window -h
tmux send-keys -t "$SESSION:0" "$BIN/media-calendar 2>&1; echo '--- CALENDAR DONE ---'" Enter

# Bottom: arr-feed (snapshot mode, no watch)
tmux split-window -v
tmux send-keys -t "$SESSION:0" "$BIN/arr-feed 2>&1; echo '--- FEED DONE ---'" Enter

# Balance panes
tmux select-layout -t "$SESSION:0" main-horizontal 2>/dev/null || true

tmux attach-session -t "$SESSION"
