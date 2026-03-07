#!/usr/bin/env bash
# scripts/install.sh
# Navi 一键安装脚本（Linux / macOS）
#
# 用法：
#   curl -fsSL https://raw.githubusercontent.com/fengzhanhuaer/Navi/main/scripts/install.sh | bash
#   或指定版本：
#   VERSION=v1.0.0 bash install.sh

set -e

REPO="fengzhanhuaer/Navi"   # GitHub 仓库
INSTALL_DIR="/opt/navi"
SERVICE_NAME="navi"
PORT="15020"

# ── 颜色输出 ────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# ── 检测平台 ────────────────────────────────────
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac
    case "$OS" in
        linux)  PLATFORM="linux-${ARCH}" ;;
        darwin) PLATFORM="darwin-${ARCH}" ;;
        *) error "Unsupported OS: $OS" ;;
    esac
    info "Detected platform: $PLATFORM"
}

# ── 获取最新版本号 ──────────────────────────────
get_latest_version() {
    if [ -n "$VERSION" ]; then
        info "Using specified version: $VERSION"
        return
    fi
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases" | grep -m 1 '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    [ -z "$VERSION" ] && error "Failed to get latest version. Check $REPO releases."
    info "Latest version: $VERSION"
}

# ── 下载二进制 ──────────────────────────────────
download_binary() {
    BINARY_NAME="navi-${PLATFORM}"
    if [ "$OS" = "windows" ]; then
        BINARY_NAME="navi-${PLATFORM}.exe"
    fi

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"
    TMP_FILE="$(mktemp /tmp/navi-download.XXXXXX)"

    info "Downloading $DOWNLOAD_URL ..."
    info "  → Temp: $TMP_FILE"

    # 下载到临时文件（加重试，避免写入目标目录出错）
    if ! curl -fsSL --retry 3 --retry-delay 2 "$DOWNLOAD_URL" -o "$TMP_FILE"; then
        rm -f "$TMP_FILE"
        error "Download failed. Please check network or verify release ${VERSION} asset ${BINARY_NAME} exists."
    fi

    # 验证下载文件有效（大小大于 1MB）
    FILE_SIZE=$(stat -c%s "$TMP_FILE" 2>/dev/null || stat -f%z "$TMP_FILE" 2>/dev/null || echo 0)
    if [ "$FILE_SIZE" -lt 1048576 ]; then
        rm -f "$TMP_FILE"
        error "Downloaded file is too small (${FILE_SIZE} bytes). The release may not exist yet."
    fi

    mkdir -p "$INSTALL_DIR"
    mv "$TMP_FILE" "${INSTALL_DIR}/navi"
    chmod +x "${INSTALL_DIR}/navi"
    info "Saved to ${INSTALL_DIR}/navi (${FILE_SIZE} bytes)"
}

# ── 创建配置文件 ────────────────────────────────
setup_config() {
    ENV_FILE="${INSTALL_DIR}/.env"
    if [ ! -f "$ENV_FILE" ]; then
        cat > "$ENV_FILE" <<EOF
# Navi 配置文件
PORT=${PORT}
DATA_DIR=${INSTALL_DIR}/data

# Cloudflare D1 备份（可选）
# CF_ACCOUNT_ID=
# CF_D1_DATABASE_ID=
# CF_API_TOKEN=
# SYNC_INTERVAL_MIN=5
EOF
        info "Created config: $ENV_FILE"
    else
        warn "Config already exists: $ENV_FILE (skipped)"
    fi
}

# ── 注册 systemd 服务 ───────────────────────────
setup_systemd() {
    if ! command -v systemctl &>/dev/null; then
        warn "systemd not found, skipping service setup"
        warn "Run manually: ${INSTALL_DIR}/navi"
        return
    fi

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Navi Personal Navigation Page
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/navi
EnvironmentFile=${INSTALL_DIR}/.env
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME"
    systemctl restart "$SERVICE_NAME"
    info "Service '${SERVICE_NAME}' started"
}

# ── 主流程 ──────────────────────────────────────
main() {
    echo ""
    echo "  🧭  Navi Installer"
    echo "  ────────────────────────────────────────"
    echo ""

    [ "$(id -u)" -ne 0 ] && error "Please run as root: sudo bash install.sh"

    detect_platform
    get_latest_version
    download_binary
    setup_config
    setup_systemd

    echo ""
    info "✅ Navi installed successfully!"
    info "   Version : $VERSION"
    info "   Port    : $PORT"
    info "   Config  : ${INSTALL_DIR}/.env"
    info "   Data    : ${INSTALL_DIR}/data/"
    echo ""
    info "Access: http://YOUR_SERVER_IP:${PORT}"
    info "Manage: systemctl {start|stop|restart|status} ${SERVICE_NAME}"
    echo ""
}

main "$@"
