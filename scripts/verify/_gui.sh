# GUI helpers for scripts/verify/<ID>.sh (supervisor-owned; workers never edit it).
# Source it INSIDE an expect_ok/expect_fail command (the cwd is the repo root):
#   . scripts/verify/_gui.sh; gui_setup || exit 1; ...
#
#   gui_setup [go build args]   temp dir $T, binary $T/fr (go build [args] ./cmd/fastread), pixel
#                               tool $T/xwdcheck (from _xwdcheck.go), own Xvfb on a free display
#                               $D (-displayfd, 1280x1024x24, -nolisten tcp). An EXIT trap kills
#                               every app started here and Xvfb, waits for them, removes $T.
#   gui_start <name> <args...>  starts "$T/fr <args>" on $D (WAYLAND_DISPLAY unset,
#                               XDG_STATE_HOME=$T/state, stdin /dev/null, stdout/stderr in
#                               $T/<name>.out/.err, hard limit 120 s); sets APP (pid) and WID
#                               (the window named "fastread", waited for <= 10 s).
#   gui_keys <names...>         focuses WID and types xdotool key names, 60 ms apart.
#   gui_wait [secs]             waits (default 5 s) for APP to exit; sets RC; 1 if still running.
#   gui_pix <label>             xwd dump of WID; facts in $T/<label>.txt (see _xwdcheck.go).
#   pv <label> <fact> [n]       field n (default 1) of a fact line, 0 if missing.
#   gui_frame <label> <W> <H> [minticks]
#                               polls <= 5 s until a dump shows size WxH, >= 80% black, >= 20
#                               red pixels whose box centre is within 2 px of W/2, and >=
#                               minticks (default 5) tick rows (only columns W/2-1 and W/2
#                               lit) above and below the red box.
#   gui_until <secs> <command>  polls a shell condition every 0.2 s; 1 on timeout.
#   free_display                prints a display number 900-999 with no X socket or lock file.

gui_cleanup() {
  local p
  for p in $GUI_PIDS; do kill -KILL "$p" 2>/dev/null; done
  for p in $GUI_PIDS; do wait "$p" 2>/dev/null; done
  if [ -n "$GUI_XVFB" ]; then
    kill "$GUI_XVFB" 2>/dev/null
    wait "$GUI_XVFB" 2>/dev/null
  fi
  if [ -n "$T" ]; then rm -rf "$T"; fi
}

gui_setup() {
  T="$(mktemp -d)"
  GUI_PIDS=""
  GUI_XVFB=""
  trap gui_cleanup EXIT
  trap 'exit 143' TERM
  trap 'exit 130' INT
  go build "$@" -o "$T/fr" ./cmd/fastread || return 1
  cp scripts/verify/_xwdcheck.go "$T/xwdcheck.go" && go build -o "$T/xwdcheck" "$T/xwdcheck.go" || return 1
  exec 3>"$T/dpy"
  Xvfb -displayfd 3 -screen 0 1280x1024x24 -nolisten tcp >"$T/xvfb.log" 2>&1 &
  GUI_XVFB=$!
  exec 3>&-
  local i
  for i in $(seq 100); do test -s "$T/dpy" && break; sleep 0.1; done
  D=":$(head -1 "$T/dpy")"
  test "$D" != ":" || { echo "Xvfb did not start"; cat "$T/xvfb.log"; return 1; }
}

gui_start() {
  local name="$1"
  shift
  env -u WAYLAND_DISPLAY DISPLAY="$D" XDG_STATE_HOME="$T/state" timeout -k 3 120 "$T/fr" "$@" \
    </dev/null >"$T/$name.out" 2>"$T/$name.err" &
  APP=$!
  GUI_PIDS="$GUI_PIDS $APP"
  WID="$(DISPLAY="$D" timeout 10 xdotool search --sync --name '^fastread$' 2>/dev/null | head -1)"
  test -n "$WID" || { echo "$name: no window named fastread within 10 s"; cat "$T/$name.err"; return 1; }
}

gui_keys() {
  DISPLAY="$D" timeout 10 xdotool windowfocus --sync "$WID" key --delay 60 "$@"
}

gui_wait() {
  local i n=$(( ${1:-5} * 20 ))
  for i in $(seq "$n"); do kill -0 "$APP" 2>/dev/null || break; sleep 0.05; done
  if kill -0 "$APP" 2>/dev/null; then echo "app still running after ${1:-5} s"; return 1; fi
  wait "$APP"
  RC=$?
}

gui_pix() {
  DISPLAY="$D" timeout 10 xwd -silent -id "$WID" >"$T/$1.xwd" 2>/dev/null &&
    "$T/xwdcheck" "$T/$1.xwd" >"$T/$1.txt"
}

pv() {
  local v
  v="$(awk -v k="$2" -v n="${3:-1}" '$1 == k { print $(n + 1) }' "$T/$1.txt" 2>/dev/null)"
  echo "${v:-0}"
}

gui_frame() {
  local l="$1" w="$2" h="$3" mt="${4:-5}" i c
  for i in $(seq 25); do
    if gui_pix "$l" && [ "$(pv "$l" size 1)" -eq "$w" ] && [ "$(pv "$l" size 2)" -eq "$h" ] &&
      [ "$(pv "$l" black)" -ge 80 ] && [ "$(pv "$l" red)" -ge 20 ]; then
      c=$(( $(pv "$l" redcentre2) - 2 * (w / 2) ))
      if [ "$c" -ge -4 ] && [ "$c" -le 4 ] &&
        [ "$(pv "$l" ticks 1)" -ge "$mt" ] && [ "$(pv "$l" ticks 2)" -ge "$mt" ]; then
        return 0
      fi
    fi
    sleep 0.2
  done
  echo "frame check '$l' failed (want ${w}x${h}, red centre at $((w / 2)), >= $mt tick rows each side):"
  cat "$T/$l.txt" 2>/dev/null
  return 1
}

gui_until() {
  local secs="$1" i
  shift
  for i in $(seq $(( secs * 5 ))); do
    eval "$*" && return 0
    sleep 0.2
  done
  return 1
}

free_display() {
  local i
  for i in $(seq 900 999); do
    test -e "/tmp/.X11-unix/X$i" || test -e "/tmp/.X$i-lock" || { echo "$i"; return 0; }
  done
  return 1
}
