#!/usr/bin/env bash
# Verification for T000 (supervisor-owned; workers never edit it). Exit 0 = pass.
# Phase 0 preflight from Plan.md, run by the supervisor itself.
. "$(dirname "$0")/_lib.sh"
expect_ok "go >= 1.24" <<'CMD'
go version && test "$(go env GOVERSION | sed 's/go1\.\([0-9]*\).*/\1/')" -ge 24
CMD
expect_ok "gcc and pkg-config" <<'CMD'
gcc --version && pkg-config --version
CMD
expect_ok "Gio system libraries (pkg-config; x11-xcb needs libx11-xcb-dev)" <<'CMD'
for p in wayland-client wayland-egl wayland-cursor xkbcommon xkbcommon-x11 x11 x11-xcb xcursor xfixes egl glesv2 vulkan; do
  pkg-config --exists "$p" || { echo "missing pkg-config module: $p"; missing=1; }
done
test -z "$missing"
CMD
expect_ok "tmux, Xvfb, xdotool, git" <<'CMD'
tmux -V && command -v Xvfb && command -v xdotool && git --version
CMD
expect_ok "module proxy reachable, pinned versions exist" 120 <<'CMD'
GOTOOLCHAIN=local go list -m gioui.org@v0.10.3 github.com/ledongthuc/pdf@v0.0.0-20260907135840-6c8c28e0e8a0
CMD
expect_ok "protected files unchanged" <<'CMD'
scripts/check-protected.sh
CMD
expect_ok "working tree clean (except Improvements.md, .README.md.swp)" <<'CMD'
test -z "$(git status --porcelain | grep -v -e ' Improvements.md$' -e ' \.README\.md\.swp$')"
CMD
finish
