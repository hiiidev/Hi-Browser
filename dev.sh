#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODE="${1:-stable}"
DEV_SESSION_KEY="$(printf '%s' "$ROOT_DIR" | cksum | awk '{print $1}')"
DEV_PID_FILE="${TMPDIR:-/tmp}/hi-browser-dev-$(id -u)-${DEV_SESSION_KEY}.pids"
DEV_SESSION_ID="$$"
frontend_pid=""
wails_pid=""

usage() {
  cat <<'EOF'
Usage:
  ./dev.sh [stable|live|help]

Modes:
  stable   Default. Build frontend static assets and start Wails without Vite dev server.
  live     Start the frontend dev server and connect Wails to it.
  help     Show this help.
EOF
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "[ERROR] Missing required command: $cmd" >&2
    exit 1
  fi
}

process_cwd() {
  local pid="$1"

  if [[ -e "/proc/$pid/cwd" ]]; then
    readlink "/proc/$pid/cwd" 2>/dev/null || true
    return
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -a -p "$pid" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p' | head -n 1
  fi
}

is_project_dev_process() {
  local pid="$1"
  local kind="$2"
  local command cwd belongs_to_repo

  command="$(ps -p "$pid" -o command= 2>/dev/null || true)"
  cwd="$(process_cwd "$pid")"
  belongs_to_repo=0
  if [[ "$command" == *"$ROOT_DIR"* || "$cwd" == "$ROOT_DIR" || "$cwd" == "$ROOT_DIR/"* ]]; then
    belongs_to_repo=1
  fi
  [[ "$belongs_to_repo" -eq 1 ]] || return 1

  case "$kind" in
    frontend)
      [[ "$command" == *"npm run dev:raw"* || "$command" == *"vite/bin/vite.js"* || "$command" == *"dev-watcher.mjs"* ]]
      ;;
    wails)
      [[ "$command" == *"wails dev"* ]]
      ;;
    *)
      return 1
      ;;
  esac
}

terminate_process_tree() {
  local pid="$1"
  local child

  [[ "$pid" =~ ^[0-9]+$ ]] || return 0
  kill -0 "$pid" >/dev/null 2>&1 || return 0

  while read -r child; do
    [[ -n "$child" ]] && terminate_process_tree "$child"
  done < <(pgrep -P "$pid" 2>/dev/null || true)

  kill -TERM "$pid" >/dev/null 2>&1 || true
  for _ in $(seq 1 20); do
    kill -0 "$pid" >/dev/null 2>&1 || return 0
    sleep 0.1
  done
  kill -KILL "$pid" >/dev/null 2>&1 || true
}

write_dev_pid_file() {
  printf 'owner %s\n' "$DEV_SESSION_ID" >"$DEV_PID_FILE"
  [[ -n "$frontend_pid" ]] && printf 'frontend %s\n' "$frontend_pid" >>"$DEV_PID_FILE"
  [[ -n "$wails_pid" ]] && printf 'wails %s\n' "$wails_pid" >>"$DEV_PID_FILE"
  return 0
}

remove_owned_dev_pid_file() {
  local kind owner

  [[ -f "$DEV_PID_FILE" ]] || return 0
  read -r kind owner <"$DEV_PID_FILE" || true
  if [[ "$kind" == "owner" && "$owner" == "$DEV_SESSION_ID" ]]; then
    rm -f "$DEV_PID_FILE"
  fi
}

cleanup_recorded_dev_processes() {
  local kind pid recorded_owner current_kind current_owner

  [[ -f "$DEV_PID_FILE" ]] || return 0
  while read -r kind pid; do
    if [[ "$kind" == "owner" ]]; then
      recorded_owner="$pid"
      continue
    fi
    [[ "$kind" == "frontend" || "$kind" == "wails" ]] || continue
    if is_project_dev_process "$pid" "$kind"; then
      echo "Cleaning stale $kind process (PID $pid)..."
      terminate_process_tree "$pid"
    fi
  done <"$DEV_PID_FILE"
  [[ -f "$DEV_PID_FILE" ]] || return 0
  read -r current_kind current_owner <"$DEV_PID_FILE" || true
  if [[ -z "${recorded_owner:-}" || ( "$current_kind" == "owner" && "$current_owner" == "$recorded_owner" ) ]]; then
    rm -f "$DEV_PID_FILE"
  fi
}

cleanup_live_processes() {
  local exit_code=$?

  trap - EXIT INT TERM
  if [[ -n "$wails_pid" ]]; then
    terminate_process_tree "$wails_pid"
  fi
  if [[ -n "$frontend_pid" ]]; then
    terminate_process_tree "$frontend_pid"
  fi
  remove_owned_dev_pid_file
  exit "$exit_code"
}

resolve_runtime_platform() {
  local os_name arch_name

  case "$(uname -s)" in
    Darwin) os_name="darwin" ;;
    Linux) os_name="linux" ;;
    *)
      echo "[ERROR] Unsupported development host: $(uname -s)" >&2
      return 1
      ;;
  esac

  case "$(uname -m)" in
    arm64|aarch64) arch_name="arm64" ;;
    x86_64|amd64) arch_name="amd64" ;;
    *)
      echo "[ERROR] Unsupported development architecture: $(uname -m)" >&2
      return 1
      ;;
  esac

  printf '%s-%s\n' "$os_name" "$arch_name"
}

prepare_proxy_runtime_env() {
  local platform_dir xray_path singbox_path

  platform_dir="$(resolve_runtime_platform)"
  xray_path="$ROOT_DIR/bin/$platform_dir/xray"
  singbox_path="$ROOT_DIR/bin/$platform_dir/sing-box"

  if [[ -z "${XRAY_BINARY_PATH:-}" ]]; then
    if [[ ! -f "$xray_path" ]]; then
      echo "[ERROR] Missing Xray runtime: $xray_path" >&2
      return 1
    fi
    export XRAY_BINARY_PATH="$xray_path"
  fi

  if [[ -z "${SINGBOX_BINARY_PATH:-}" ]]; then
    if [[ ! -f "$singbox_path" ]]; then
      echo "[ERROR] Missing sing-box runtime: $singbox_path" >&2
      return 1
    fi
    export SINGBOX_BINARY_PATH="$singbox_path"
  fi

  echo "Xray runtime: $XRAY_BINARY_PATH"
  echo "sing-box runtime: $SINGBOX_BINARY_PATH"
}

is_tcp_port_busy() {
  local port="$1"
  local host="${WAILS_DEVSERVER_HOST:-127.0.0.1}"

  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$port" -sTCP:LISTEN -t 2>/dev/null | grep -q .
    return
  fi
  if command -v ss >/dev/null 2>&1; then
    ss -ltn "( sport = :$port )" 2>/dev/null | tail -n +2 | grep -q .
    return
  fi
  if command -v nc >/dev/null 2>&1; then
    nc -z "$host" "$port" >/dev/null 2>&1
    return
  fi

  echo "[ERROR] Port detection requires lsof, ss, or nc." >&2
  exit 1
}

resolve_wails_devserver() {
  local start_port="${WAILS_DEVSERVER_PORT:-34115}"
  local host="${WAILS_DEVSERVER_HOST:-127.0.0.1}"
  local port="$start_port"

  while is_tcp_port_busy "$port"; do
    port=$((port + 1))
  done

  WAILS_DEVSERVER_ADDRESS="$host:$port"
  export WAILS_DEVSERVER_ADDRESS
}

prepare_env() {
  require_cmd node
  require_cmd npm
  require_cmd go
  require_cmd wails
  require_cmd cksum
  require_cmd pgrep

  prepare_proxy_runtime_env

  if [[ -n "${DEV_PROXY_URL:-}" ]]; then
    export HTTP_PROXY="$DEV_PROXY_URL"
    export HTTPS_PROXY="$DEV_PROXY_URL"
    export http_proxy="$DEV_PROXY_URL"
    export https_proxy="$DEV_PROXY_URL"
  fi

  if [[ -n "${DEV_NO_PROXY:-}" ]]; then
    export NO_PROXY="$DEV_NO_PROXY"
    export no_proxy="$DEV_NO_PROXY"
  fi

  if [[ -n "${DEV_GOPROXY:-}" ]]; then
    export GOPROXY="$DEV_GOPROXY"
  elif [[ -z "${GOPROXY:-}" ]]; then
    export GOPROXY="https://goproxy.cn,direct"
  fi
}

wait_for_frontend() {
  local port="$1"

  for _ in $(seq 1 80); do
    if ! kill -0 "$frontend_pid" >/dev/null 2>&1; then
      echo "[ERROR] Frontend dev server exited before becoming ready." >&2
      wait "$frontend_pid" || true
      return 1
    fi
    if is_tcp_port_busy "$port"; then
      return 0
    fi
    sleep 0.25
  done

  echo "[ERROR] Timed out waiting for frontend dev server on port $port." >&2
  return 1
}

install_frontend_deps() {
  echo "Installing frontend dependencies..."
  npm install
}

build_frontend() {
  echo "Building frontend assets..."
  npm run build:clean
}

run_stable() {
  echo "========================================"
  echo "  Hi Browser - Dev Launcher"
  echo "========================================"
  echo
  echo "Current workdir: $ROOT_DIR"
  echo "Mode: stable"
  echo "Frontend mode: stable static assets"
  echo "Wails frontend dev server: disabled"
  echo

  prepare_env
  cleanup_recorded_dev_processes
  resolve_wails_devserver
  cd "$ROOT_DIR/frontend"
  install_frontend_deps
  build_frontend

  cd "$ROOT_DIR"
  echo "Starting Wails dev..."
  echo "Wails dev server: http://$WAILS_DEVSERVER_ADDRESS"
  exec wails dev -m -nosyncgomod -nogorebuild -noreload -s -skipbindings -assetdir frontend/dist -devserver "$WAILS_DEVSERVER_ADDRESS"
}

run_live() {
  local frontend_port="${FRONTEND_PORT:-5218}"
  local wails_exit_code=0

  trap cleanup_live_processes EXIT
  trap 'exit 130' INT TERM

  echo "========================================"
  echo "  Hi Browser - Dev Launcher"
  echo "========================================"
  echo
  echo "Current workdir: $ROOT_DIR"
  echo "Mode: live"
  echo "Frontend URL: http://127.0.0.1:$frontend_port"
  echo

  prepare_env
  cleanup_recorded_dev_processes
  if is_tcp_port_busy "$frontend_port"; then
    echo "[ERROR] Frontend port $frontend_port is already in use." >&2
    if command -v lsof >/dev/null 2>&1; then
      lsof -nP -iTCP:"$frontend_port" -sTCP:LISTEN >&2 || true
    fi
    echo "Stop the existing process or choose another FRONTEND_PORT." >&2
    return 1
  fi
  resolve_wails_devserver

  cd "$ROOT_DIR/frontend"
  install_frontend_deps
  npm run dev:raw -- --host 127.0.0.1 --port "$frontend_port" &
  frontend_pid="$!"
  write_dev_pid_file
  wait_for_frontend "$frontend_port"

  cd "$ROOT_DIR"
  echo "Starting Wails dev..."
  echo "Wails dev server: http://$WAILS_DEVSERVER_ADDRESS"
  wails dev -m -nosyncgomod -s -skipbindings -frontenddevserverurl "http://127.0.0.1:$frontend_port" -viteservertimeout 60 -devserver "$WAILS_DEVSERVER_ADDRESS" &
  wails_pid="$!"
  write_dev_pid_file
  wait "$wails_pid" || wails_exit_code=$?
  wails_pid=""
  return "$wails_exit_code"
}

case "$MODE" in
  stable)
    run_stable
    ;;
  live)
    run_live
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    echo "[ERROR] Unsupported mode: $MODE" >&2
    echo >&2
    usage >&2
    exit 1
    ;;
esac
