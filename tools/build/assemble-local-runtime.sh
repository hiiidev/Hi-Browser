#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TARGET="${1:-}"

if [[ -z "$TARGET" ]]; then
  case "$(uname -s)" in
    Darwin) os_name="darwin" ;;
    Linux) os_name="linux" ;;
    *) echo "[ERROR] unsupported local build host: $(uname -s)" >&2; exit 1 ;;
  esac
  case "$(uname -m)" in
    arm64|aarch64) arch_name="arm64" ;;
    x86_64|amd64) arch_name="amd64" ;;
    *) echo "[ERROR] unsupported local build architecture: $(uname -m)" >&2; exit 1 ;;
  esac
  TARGET="$os_name-$arch_name"
fi

XRAY_SRC="$ROOT_DIR/bin/$TARGET/xray"
SINGBOX_SRC="$ROOT_DIR/bin/$TARGET/sing-box"
for runtime in "$XRAY_SRC" "$SINGBOX_SRC"; do
  if [[ ! -f "$runtime" ]]; then
    echo "[ERROR] missing proxy runtime: $runtime" >&2
    exit 1
  fi
done

case "$TARGET" in
  darwin-*)
    APP_BUNDLE="$(find "$ROOT_DIR/build/bin" -maxdepth 1 -type d -name '*.app' -print -quit)"
    if [[ -z "$APP_BUNDLE" || ! -d "$APP_BUNDLE/Contents/MacOS" ]]; then
      echo "[ERROR] macOS app bundle not found under $ROOT_DIR/build/bin" >&2
      exit 1
    fi
    INSTALL_ROOT="$APP_BUNDLE/Contents/MacOS"
    ;;
  linux-*)
    if [[ ! -f "$ROOT_DIR/build/bin/ant-chrome" ]]; then
      echo "[ERROR] Linux executable not found: $ROOT_DIR/build/bin/ant-chrome" >&2
      exit 1
    fi
    INSTALL_ROOT="$ROOT_DIR/build/bin"
    ;;
  *)
    echo "[ERROR] unsupported local runtime target: $TARGET" >&2
    exit 1
    ;;
esac

mkdir -p "$INSTALL_ROOT/bin"
cp "$XRAY_SRC" "$INSTALL_ROOT/bin/xray"
cp "$SINGBOX_SRC" "$INSTALL_ROOT/bin/sing-box"
chmod 0755 "$INSTALL_ROOT/bin/xray" "$INSTALL_ROOT/bin/sing-box"

if [[ "$TARGET" == darwin-* ]]; then
  cp "$ROOT_DIR/publish/config.init.mac.yaml" "$INSTALL_ROOT/config.yaml"
else
  cp "$ROOT_DIR/publish/config.init.linux.yaml" "$INSTALL_ROOT/config.yaml"
fi
cp "$ROOT_DIR/browser-core-manifest.json" "$INSTALL_ROOT/browser-core-manifest.json"

if [[ "$TARGET" == darwin-* ]] && command -v codesign >/dev/null 2>&1; then
  codesign --force --deep --sign - "$APP_BUNDLE"
  codesign --verify --deep --strict "$APP_BUNDLE"
fi

echo "Local runtime assembled: $INSTALL_ROOT"
"$INSTALL_ROOT/bin/xray" version | sed -n '1p'
"$INSTALL_ROOT/bin/sing-box" version | sed -n '1p'
