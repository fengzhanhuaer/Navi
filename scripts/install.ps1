# scripts/install.ps1
# Navi 一键安装脚本（Windows PowerShell）
#
# 用法（PowerShell 以管理员运行）：
#   irm https://raw.githubusercontent.com/YOUR_USER/Navi/main/scripts/install.ps1 | iex
#   或指定版本：
#   $env:VERSION="v1.0.0"; irm .../install.ps1 | iex

$ErrorActionPreference = "Stop"

$REPO = "fengzhanhuaer/Navi"   # GitHub 仓库
$INSTALL_DIR = "C:\navi"
$PORT = "15020"
$SERVICE = "Navi"

function Write-Info { param($msg) Write-Host "[INFO]  $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "[WARN]  $msg" -ForegroundColor Yellow }
function Write-Err { param($msg) Write-Host "[ERROR] $msg" -ForegroundColor Red; exit 1 }

# ── 获取最新版本 ──────────────────────────────────
function Get-LatestVersion {
    if ($env:VERSION) { return $env:VERSION }
    $release = Invoke-RestMethod "https://api.github.com/repos/$REPO/releases/latest"
    if (-not $release.tag_name) { Write-Err "Cannot get latest version from $REPO" }
    return $release.tag_name
}

# ── 下载二进制 ────────────────────────────────────
function Download-Binary {
    param($version)
    $url = "https://github.com/$REPO/releases/download/$version/navi-windows-amd64.exe"
    $target = "$INSTALL_DIR\navi.exe"

    Write-Info "Downloading $url ..."
    New-Item -ItemType Directory -Force $INSTALL_DIR | Out-Null
    Invoke-WebRequest -Uri $url -OutFile $target -UseBasicParsing
    Write-Info "Saved to $target"
}

# ── 创建配置文件 ──────────────────────────────────
function Setup-Config {
    $envFile = "$INSTALL_DIR\.env"
    if (-not (Test-Path $envFile)) {
        @"
# Navi 配置文件
PORT=$PORT
DATA_DIR=$INSTALL_DIR\data

# Cloudflare D1 备份（可选）
# CF_ACCOUNT_ID=
# CF_D1_DATABASE_ID=
# CF_API_TOKEN=
# SYNC_INTERVAL_MIN=5
"@ | Set-Content $envFile -Encoding UTF8
        Write-Info "Created config: $envFile"
    }
    else {
        Write-Warn "Config already exists: $envFile (skipped)"
    }
}

# ── 注册为 Windows 服务（使用 NSSM 或任务计划程序）──
function Setup-Service {
    param($version)

    # 优先使用 NSSM（如果存在）
    $nssm = Get-Command nssm -ErrorAction SilentlyContinue
    if ($nssm) {
        Write-Info "Using NSSM to register service..."
        & nssm install $SERVICE "$INSTALL_DIR\navi.exe"
        & nssm set $SERVICE AppDirectory $INSTALL_DIR
        & nssm set $SERVICE AppEnvironmentExtra "PORT=$PORT" "DATA_DIR=$INSTALL_DIR\data"
        & nssm set $SERVICE Start SERVICE_AUTO_START
        & nssm start $SERVICE
        Write-Info "Service '$SERVICE' registered and started via NSSM"
        return
    }

    # 否则用任务计划程序（开机自启）
    Write-Warn "NSSM not found, using Task Scheduler instead"
    $action = New-ScheduledTaskAction -Execute "$INSTALL_DIR\navi.exe" -WorkingDirectory $INSTALL_DIR
    $trigger = New-ScheduledTaskTrigger -AtStartup
    $settings = New-ScheduledTaskSettingsSet -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
    $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -RunLevel Highest

    Register-ScheduledTask -TaskName $SERVICE `
        -Action $action -Trigger $trigger `
        -Settings $settings -Principal $principal `
        -Force | Out-Null

    Start-ScheduledTask -TaskName $SERVICE
    Write-Info "Task '$SERVICE' registered and started"
}

# ── 主流程 ────────────────────────────────────────
function Main {
    Write-Host ""
    Write-Host "  🧭  Navi Installer (Windows)" -ForegroundColor Cyan
    Write-Host "  ────────────────────────────────────────"
    Write-Host ""

    $version = Get-LatestVersion
    Write-Info "Version: $version"

    Download-Binary $version
    Setup-Config
    Setup-Service $version

    Write-Host ""
    Write-Info "✅ Navi installed successfully!"
    Write-Info "   Version  : $version"
    Write-Info "   Port     : $PORT"
    Write-Info "   Install  : $INSTALL_DIR"
    Write-Info "   Config   : $INSTALL_DIR\.env"
    Write-Host ""
    Write-Info "Access: http://localhost:$PORT"
    Write-Host ""
}

Main
