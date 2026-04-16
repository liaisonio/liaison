#!/bin/bash

# Liaison Edge 安装脚本
# 此脚本会根据操作系统自动下载并安装对应的 Edge 安装包

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 默认配置
SERVER_HTTP_ADDR=""  # HTTP下载地址（host:port，用于下载安装包）
SERVER_EDGE_ADDR=""  # Edge连接地址（host:port，用于建立长连接）
INSTALL_DIR="/opt/liaison"
BIN_DIR="/opt/liaison/bin"
CONFIG_DIR="/opt/liaison/conf"
LOG_DIR="/opt/liaison/logs"

# 解析参数
ACCESS_KEY=""
SECRET_KEY=""

show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --access-key=KEY        Access key (required)"
    echo "  --secret-key=KEY        Secret key (required)"
    echo "  --server-http-addr=ADDR HTTP download address (host:port, for downloading packages)"
    echo "  --server-edge-addr=ADDR Edge connection address (host:port, for establishing connection)"
    echo "  -h, --help              Show this help message"
    echo ""
    echo "Example:"
    echo "  $0 --access-key=xxx --secret-key=yyy --server-http-addr=example.com:443 --server-edge-addr=example.com:30012"
    exit 0
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --access-key=*)
            ACCESS_KEY="${1#*=}"
            shift
            ;;
        --secret-key=*)
            SECRET_KEY="${1#*=}"
            shift
            ;;
        --server-http-addr=*)
            SERVER_HTTP_ADDR="${1#*=}"
            shift
            ;;
        --server-edge-addr=*)
            SERVER_EDGE_ADDR="${1#*=}"
            shift
            ;;
        -h|--help)
            show_help
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# 验证必需参数
if [[ -z "$SERVER_HTTP_ADDR" ]] || [[ -z "$SERVER_EDGE_ADDR" ]]; then
    echo -e "${RED}Error: --server-http-addr and --server-edge-addr are required${NC}"
    echo "Use --help for usage information"
    exit 1
fi

if [ -z "$ACCESS_KEY" ] || [ -z "$SECRET_KEY" ]; then
    echo -e "${RED}Error: --access-key and --secret-key are required${NC}"
    echo "Use --help for usage information"
    exit 1
fi

echo -e "${GREEN}Starting Liaison Edge installation...${NC}"

# 检测操作系统
detect_os() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        # 检测 Linux 架构
        ARCH=$(uname -m)
        if [ "$ARCH" == "x86_64" ]; then
            echo "linux-amd64"
        elif [ "$ARCH" == "aarch64" ] || [ "$ARCH" == "arm64" ]; then
            echo "linux-arm64"
        else
            echo "linux-$ARCH"
        fi
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        # 检测 macOS 架构
        ARCH=$(uname -m)
        if [ "$ARCH" == "x86_64" ]; then
            echo "darwin-amd64"
        elif [ "$ARCH" == "arm64" ]; then
            echo "darwin-arm64"
        else
            echo "darwin-$ARCH"
        fi
    elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]] || [[ "$OSTYPE" == "win32" ]]; then
        echo "windows-amd64"
    else
        echo -e "${RED}Unsupported operating system: $OSTYPE${NC}"
        exit 1
    fi
}

OS_ARCH=$(detect_os)
echo -e "${GREEN}Detected OS/Arch: $OS_ARCH${NC}"

# 根据操作系统设置正确的路径
if [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]] || [[ "$OSTYPE" == "win32" ]]; then
    # Windows 系统：使用标准 Windows 路径
    # 如果使用 Git Bash/Cygwin，cygpath 可以将路径转换为 Windows 格式
    if command -v cygpath >/dev/null 2>&1; then
        # 使用 cygpath 转换路径（/c/Program Files/Liaison）
        INSTALL_DIR=$(cygpath -w "/c/Program Files/Liaison" 2>/dev/null || echo "C:\\Program Files\\Liaison")
        BIN_DIR=$(cygpath -w "/c/Program Files/Liaison/bin" 2>/dev/null || echo "C:\\Program Files\\Liaison\\bin")
        CONFIG_DIR=$(cygpath -w "/c/Program Files/Liaison/conf" 2>/dev/null || echo "C:\\Program Files\\Liaison\\conf")
        LOG_DIR=$(cygpath -w "/c/Program Files/Liaison/logs" 2>/dev/null || echo "C:\\Program Files\\Liaison\\logs")
    else
        # 没有 cygpath，使用标准 Windows 路径
        INSTALL_DIR="C:\\Program Files\\Liaison"
        BIN_DIR="C:\\Program Files\\Liaison\\bin"
        CONFIG_DIR="C:\\Program Files\\Liaison\\conf"
        LOG_DIR="C:\\Program Files\\Liaison\\logs"
    fi
    echo -e "${YELLOW}Windows 安装路径: ${INSTALL_DIR}${NC}"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS 系统：使用标准 macOS 路径
    BIN_DIR="/usr/local/bin"
    CONFIG_DIR="${HOME}/Library/Application Support/liaison"
    LOG_DIR="${HOME}/Library/Logs/liaison"
    INSTALL_DIR="/usr/local"  # 用于工作目录等
    echo -e "${YELLOW}macOS 安装路径:${NC}"
    echo -e "${YELLOW}  二进制: ${BIN_DIR}${NC}"
    echo -e "${YELLOW}  配置: ${CONFIG_DIR}${NC}"
    echo -e "${YELLOW}  日志: ${LOG_DIR}${NC}"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux 系统：使用标准 Linux 路径
    BIN_DIR="/usr/local/bin"
    CONFIG_DIR="/etc/liaison"
    DATA_DIR="/var/lib/liaison"
    LOG_DIR="/var/log/liaison"
    INSTALL_DIR="/usr/local"  # 用于工作目录等
    echo -e "${YELLOW}Linux 安装路径:${NC}"
    echo -e "${YELLOW}  二进制: ${BIN_DIR}${NC}"
    echo -e "${YELLOW}  配置: ${CONFIG_DIR}${NC}"
    echo -e "${YELLOW}  数据: ${DATA_DIR}${NC}"
    echo -e "${YELLOW}  日志: ${LOG_DIR}${NC}"
fi

# 确定安装包文件名（统一使用 tar.gz 格式）
PACKAGE_NAME="liaison-edge-${OS_ARCH}.tar.gz"

# 创建临时目录
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# 下载安装包
echo -e "${YELLOW}Downloading installation package...${NC}"
# 从HTTP地址中提取host和port，构建下载URL
HTTP_HOST="${SERVER_HTTP_ADDR%%:*}"
HTTP_PORT="${SERVER_HTTP_ADDR##*:}"
if [[ "$HTTP_PORT" == "$SERVER_HTTP_ADDR" ]]; then
    # 如果没有端口，使用默认端口443
    HTTP_HOST="$SERVER_HTTP_ADDR"
    HTTP_PORT="443"
fi

PACKAGE_URL="https://${SERVER_HTTP_ADDR}/packages/edge/${PACKAGE_NAME}"

echo -e "${YELLOW}HTTPS download address: ${SERVER_HTTP_ADDR}${NC}"
echo -e "${YELLOW}Edge connection address: ${SERVER_EDGE_ADDR}${NC}"
echo -e "${YELLOW}Package URL: ${PACKAGE_URL}${NC}"

if command -v curl >/dev/null 2>&1; then
    HTTP_CODE=$(curl -k -sSL -o "${TMP_DIR}/${PACKAGE_NAME}" -w "%{http_code}" "${PACKAGE_URL}")
elif command -v wget >/dev/null 2>&1; then
    wget -q -O "${TMP_DIR}/${PACKAGE_NAME}" "${PACKAGE_URL}" || HTTP_CODE="404"
    if [ $? -eq 0 ]; then
        HTTP_CODE="200"
    else
        HTTP_CODE="404"
    fi
else
    echo -e "${RED}Error: curl or wget is required${NC}"
    exit 1
fi

if [ "$HTTP_CODE" != "200" ]; then
    echo -e "${RED}Error: Failed to download package (HTTP $HTTP_CODE)${NC}"
    echo -e "${YELLOW}Package URL: $PACKAGE_URL${NC}"
    exit 1
fi

# 解压安装包
echo -e "${YELLOW}Extracting package...${NC}"
cd "$TMP_DIR"
if ! tar -xzf "${PACKAGE_NAME}"; then
    echo -e "${RED}Error: Failed to extract package${NC}"
    exit 1
fi

# 安装
echo -e "${YELLOW}Installing...${NC}"

# 查找解压后的二进制文件
BINARY_NAME="liaison-edge"
if [[ "$OS_ARCH" == "windows"* ]]; then
    BINARY_NAME="liaison-edge.exe"
fi

if [ ! -f "${TMP_DIR}/${BINARY_NAME}" ]; then
    echo -e "${RED}Error: Binary file ${BINARY_NAME} not found in package${NC}"
    exit 1
fi

# Linux/macOS/Windows 安装
# 创建必要的目录
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS: 二进制需要 sudo，配置和日志在用户目录下不需要 sudo
    if ! sudo -n true 2>/dev/null; then
        echo -e "${YELLOW}需要 sudo 权限来安装二进制文件，请输入密码:${NC}"
    fi
    sudo mkdir -p "$BIN_DIR"
    mkdir -p "${CONFIG_DIR}"
    mkdir -p "${LOG_DIR}"
    # 复制二进制文件（需要 sudo）
    sudo cp "${TMP_DIR}/${BINARY_NAME}" "${BIN_DIR}/${BINARY_NAME}"
    sudo chmod +x "${BIN_DIR}/${BINARY_NAME}"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux 需要 sudo 权限创建系统目录
    if ! sudo -n true 2>/dev/null; then
        echo -e "${YELLOW}需要 sudo 权限来创建系统目录，请输入密码:${NC}"
    fi
    sudo mkdir -p "$BIN_DIR"
    sudo mkdir -p "$CONFIG_DIR"
    sudo mkdir -p "$DATA_DIR"
    sudo mkdir -p "$LOG_DIR"
    # 复制二进制文件
    sudo cp "${TMP_DIR}/${BINARY_NAME}" "${BIN_DIR}/${BINARY_NAME}"
    sudo chmod +x "${BIN_DIR}/${BINARY_NAME}"
else
    # Windows 安装
    mkdir -p "$BIN_DIR"
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$LOG_DIR"
    # 复制二进制文件
    cp "${TMP_DIR}/${BINARY_NAME}" "${BIN_DIR}/${BINARY_NAME}"
    chmod +x "${BIN_DIR}/${BINARY_NAME}"
fi

# 从模板渲染配置文件
echo -e "${YELLOW}Rendering configuration file from template...${NC}"
# 尝试从多个位置查找模板文件：
# 1. 从解压的安装包中（TMP_DIR）
# 2. 从脚本所在目录（如果脚本是从文件系统运行的）
TEMPLATE_FILE=""
if [[ -f "${TMP_DIR}/liaison-edge.yaml.template" ]]; then
    TEMPLATE_FILE="${TMP_DIR}/liaison-edge.yaml.template"
elif [[ -f "$(dirname "${BASH_SOURCE[0]}")/liaison-edge.yaml.template" ]]; then
    TEMPLATE_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/liaison-edge.yaml.template"
fi

if [[ -n "$TEMPLATE_FILE" && -f "$TEMPLATE_FILE" ]]; then
    # 替换模板中的变量（使用Edge连接地址）
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS: 配置文件在用户目录下，不需要 sudo
        # 确保目录存在
        mkdir -p "${CONFIG_DIR}"
        sed -e "s|\${SERVER_ADDR}|${SERVER_EDGE_ADDR}|g" \
            -e "s|\${ACCESS_KEY}|${ACCESS_KEY}|g" \
            -e "s|\${SECRET_KEY}|${SECRET_KEY}|g" \
            -e "s|\${LOG_DIR}|${LOG_DIR}|g" \
            "$TEMPLATE_FILE" > "${CONFIG_DIR}/liaison-edge.yaml"
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        # Linux 需要 sudo 权限写入 /etc 目录
        sed -e "s|\${SERVER_ADDR}|${SERVER_EDGE_ADDR}|g" \
            -e "s|\${ACCESS_KEY}|${ACCESS_KEY}|g" \
            -e "s|\${SECRET_KEY}|${SECRET_KEY}|g" \
            -e "s|\${LOG_DIR}|${LOG_DIR}|g" \
            "$TEMPLATE_FILE" | sudo tee "${CONFIG_DIR}/liaison-edge.yaml" > /dev/null
    else
        sed -e "s|\${SERVER_ADDR}|${SERVER_EDGE_ADDR}|g" \
            -e "s|\${ACCESS_KEY}|${ACCESS_KEY}|g" \
            -e "s|\${SECRET_KEY}|${SECRET_KEY}|g" \
            -e "s|\${LOG_DIR}|${LOG_DIR}|g" \
            "$TEMPLATE_FILE" > "${CONFIG_DIR}/liaison-edge.yaml"
    fi
    echo -e "${GREEN}Configuration file rendered from template${NC}"
    echo -e "${GREEN}Edge will connect to: ${SERVER_EDGE_ADDR}${NC}"
else
    echo -e "${YELLOW}Template file not found, creating default configuration...${NC}"
    # 如果模板文件不存在，使用默认配置
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS: 配置文件在用户目录下，不需要 sudo
        # 确保目录存在
        mkdir -p "${CONFIG_DIR}"
        tee "${CONFIG_DIR}/liaison-edge.yaml" > /dev/null <<EOF
manager:
  dial:
    addrs:
      - ${SERVER_EDGE_ADDR}
    network: tcp
    tls:
      enable: true
      insecure_skip_verify: true
  auth:
    access_key: "${ACCESS_KEY}"
    secret_key: "${SECRET_KEY}"
log:
  level: info
  file: ${LOG_DIR}/liaison-edge.log
  maxsize: 100
  maxrolls: 10
EOF
    else
        cat > "${CONFIG_DIR}/liaison-edge.yaml" <<EOF
manager:
  dial:
    addrs:
      - ${SERVER_EDGE_ADDR}
    network: tcp
    tls:
      enable: true
      insecure_skip_verify: true
  auth:
    access_key: "${ACCESS_KEY}"
    secret_key: "${SECRET_KEY}"
log:
  level: info
  file: ${LOG_DIR}/liaison-edge.log
  maxsize: 100
  maxrolls: 10
EOF
    fi
    echo -e "${GREEN}Edge will connect to: ${SERVER_EDGE_ADDR}${NC}"
fi

echo -e "${GREEN}Installation completed!${NC}"
echo -e "${GREEN}Edge binary: ${BIN_DIR}/${BINARY_NAME}${NC}"
echo -e "${GREEN}Config file: ${CONFIG_DIR}/liaison-edge.yaml${NC}"
echo ""

# 根据操作系统提供不同的后台运行方式选择
setup_service() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        # Linux 系统
        echo -e "${YELLOW}请选择后台运行方式:${NC}"
        echo "1) systemd 服务（推荐，支持开机自启、自动重启）"
        echo "2) nohup 后台运行（简单方式）"
        echo "3) screen 会话（适合调试）"
        echo "4) 跳过，稍后手动启动"
        echo ""
        if [ -t 0 ]; then
            # 交互式运行，可以读取用户输入
            read -p "请输入选项 [1-4] (默认: 1): " choice
            choice=${choice:-1}
        else
            # 非交互式运行，使用默认值（systemd 服务）
            echo -e "${YELLOW}非交互式运行，使用默认选项: systemd 服务${NC}"
            choice=1
        fi
        
        case $choice in
            1)
                echo -e "${YELLOW}设置 systemd 服务...${NC}"
                CURRENT_USER=$(whoami)
                SERVICE_FILE="/etc/systemd/system/liaison-edge.service"
                
                # 检查是否有 sudo 权限
                if ! sudo -n true 2>/dev/null; then
                    echo -e "${YELLOW}需要 sudo 权限来创建 systemd 服务，请输入密码:${NC}"
                fi
                sudo tee "${SERVICE_FILE}" > /dev/null <<EOF
[Unit]
Description=Liaison Edge Service
After=network.target

[Service]
Type=simple
User=${CURRENT_USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BIN_DIR}/${BINARY_NAME} -c "${CONFIG_DIR}/liaison-edge.yaml"
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
                sudo systemctl daemon-reload
                sudo systemctl enable liaison-edge
                sudo systemctl start liaison-edge
                echo -e "${GREEN}systemd 服务已创建并启动${NC}"
                echo -e "${YELLOW}查看状态: sudo systemctl status liaison-edge${NC}"
                echo -e "${YELLOW}查看日志: sudo journalctl -u liaison-edge -f${NC}"
                ;;
            2)
                echo -e "${YELLOW}使用 nohup 后台运行...${NC}"
                nohup "${BIN_DIR}/${BINARY_NAME}" -c "${CONFIG_DIR}/liaison-edge.yaml" > "${LOG_DIR}/liaison-edge.log" 2>&1 &
                PID=$!
                echo -e "${GREEN}Edge 已在后台启动 (PID: $PID)${NC}"
                ;;
            3)
                echo -e "${YELLOW}使用 screen 会话运行...${NC}"
                if command -v screen >/dev/null 2>&1; then
                    screen -dmS liaison-edge "${BIN_DIR}/${BINARY_NAME}" -c "${CONFIG_DIR}/liaison-edge.yaml"
                    echo -e "${GREEN}Edge 已在 screen 会话中启动${NC}"
                    echo -e "${YELLOW}查看会话: screen -r liaison-edge${NC}"
                else
                    echo -e "${RED}错误: screen 未安装，请先安装 screen${NC}"
                    echo "  Ubuntu/Debian: sudo apt-get install screen"
                    echo "  CentOS/RHEL: sudo yum install screen"
                fi
                ;;
            4)
                echo -e "${YELLOW}跳过服务设置${NC}"
                ;;
            *)
                echo -e "${RED}无效选项${NC}"
                ;;
        esac
        
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS 系统
        echo -e "${YELLOW}请选择后台运行方式:${NC}"
        echo "1) launchd 服务（推荐，支持开机自启、自动重启）"
        echo "2) nohup 后台运行（简单方式）"
        echo "3) screen 会话（适合调试）"
        echo "4) 跳过，稍后手动启动"
        echo ""
        if [ -t 0 ]; then
            # 交互式运行，可以读取用户输入
            read -p "请输入选项 [1-4] (默认: 1): " choice
            choice=${choice:-1}
        else
            # 非交互式运行，使用默认值（systemd 服务）
            echo -e "${YELLOW}非交互式运行，使用默认选项: systemd 服务${NC}"
            choice=1
        fi
        
        case $choice in
            1)
                echo -e "${YELLOW}设置 launchd 服务...${NC}"
                PLIST_FILE="$HOME/Library/LaunchAgents/com.liaison.edge.plist"
                mkdir -p "$HOME/Library/LaunchAgents"
                
                cat > "${PLIST_FILE}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.liaison.edge</string>
    <key>ProgramArguments</key>
    <array>
        <string>${BIN_DIR}/${BINARY_NAME}</string>
        <string>-c</string>
        <string>${CONFIG_DIR}/liaison-edge.yaml</string>
    </array>
    <key>WorkingDirectory</key>
    <string>${INSTALL_DIR}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${LOG_DIR}/liaison-edge.log</string>
    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/liaison-edge.error.log</string>
</dict>
</plist>
EOF
                # 使用新的 launchctl bootstrap API (macOS 10.11+)
                # 先尝试卸载（如果已存在）
                launchctl bootout "gui/$(id -u)/com.liaison.edge" 2>/dev/null || \
                launchctl unload "${PLIST_FILE}" 2>/dev/null || true
                # 使用 bootstrap 加载服务
                launchctl bootstrap "gui/$(id -u)" "${PLIST_FILE}" 2>/dev/null || \
                launchctl load -w "${PLIST_FILE}" 2>/dev/null || {
                    echo -e "${YELLOW}警告: 无法自动加载 launchd 服务，请手动运行:${NC}"
                    echo -e "${YELLOW}  launchctl bootstrap gui/$(id -u) ${PLIST_FILE}${NC}"
                    echo -e "${YELLOW}  launchctl start gui/$(id -u)/com.liaison.edge${NC}"
                }
                # 启动服务
                launchctl kickstart "gui/$(id -u)/com.liaison.edge" 2>/dev/null || \
                launchctl start com.liaison.edge 2>/dev/null || true
                echo -e "${GREEN}launchd 服务已创建并启动${NC}"
                echo -e "${YELLOW}查看状态: launchctl list | grep liaison${NC}"
                ;;
            2)
                echo -e "${YELLOW}使用 nohup 后台运行...${NC}"
                nohup "${BIN_DIR}/${BINARY_NAME}" -c "${CONFIG_DIR}/liaison-edge.yaml" > "${LOG_DIR}/liaison-edge.log" 2>&1 &
                PID=$!
                echo -e "${GREEN}Edge 已在后台启动 (PID: $PID)${NC}"
                ;;
            3)
                echo -e "${YELLOW}使用 screen 会话运行...${NC}"
                if command -v screen >/dev/null 2>&1; then
                    screen -dmS liaison-edge "${BIN_DIR}/${BINARY_NAME}" -c "${CONFIG_DIR}/liaison-edge.yaml"
                    echo -e "${GREEN}Edge 已在 screen 会话中启动${NC}"
                    echo -e "${YELLOW}查看会话: screen -r liaison-edge${NC}"
                else
                    echo -e "${RED}错误: screen 未安装，请先安装 screen${NC}"
                    echo "  brew install screen"
                fi
                ;;
            4)
                echo -e "${YELLOW}跳过服务设置${NC}"
                ;;
            *)
                echo -e "${RED}无效选项${NC}"
                ;;
        esac
        
    elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]] || [[ "$OSTYPE" == "win32" ]]; then
        # Windows 系统
        echo -e "${YELLOW}请选择后台运行方式:${NC}"
        echo "1) nohup 后台运行（Git Bash/Cygwin）"
        echo "2) 跳过，稍后手动启动"
        echo ""
        if [ -t 0 ]; then
            # 交互式运行，可以读取用户输入
            read -p "请输入选项 [1-2] (默认: 1): " choice
            choice=${choice:-1}
        else
            # 非交互式运行，使用默认值（nohup）
            echo -e "${YELLOW}非交互式运行，使用默认选项: nohup 后台运行${NC}"
            choice=1
        fi
        
        case $choice in
            1)
                echo -e "${YELLOW}使用 nohup 后台运行...${NC}"
                nohup "${BIN_DIR}/${BINARY_NAME}" -c "${CONFIG_DIR}/liaison-edge.yaml" > "${LOG_DIR}/liaison-edge.log" 2>&1 &
                PID=$!
                echo -e "${GREEN}Edge 已在后台启动 (PID: $PID)${NC}"
                ;;
            2)
                echo -e "${YELLOW}跳过服务设置${NC}"
                ;;
            *)
                echo -e "${RED}无效选项${NC}"
                ;;
        esac
    else
        # 其他系统
        echo -e "${YELLOW}使用 nohup 后台运行...${NC}"
        nohup "${BIN_DIR}/liaison-edge" -c "${CONFIG_DIR}/liaison-edge.yaml" > "${LOG_DIR}/liaison-edge.log" 2>&1 &
        PID=$!
        echo -e "${GREEN}Edge 已在后台启动 (PID: $PID)${NC}"
    fi
}

# 询问是否设置后台运行
# 检查标准输入是否是终端（交互式运行）
if [ -t 0 ]; then
    # 交互式运行，可以读取用户输入
    echo -e "${YELLOW}是否现在设置后台运行？${NC}"
    read -p "请输入 [y/N] (默认: y): " setup
    setup=${setup:-y}
else
    # 非交互式运行（通过管道），使用默认值
    echo -e "${YELLOW}非交互式运行模式，使用默认设置（自动设置后台运行）${NC}"
    setup="y"
fi

if [[ "$setup" =~ ^[Yy]$ ]]; then
    setup_service
else
    echo -e "${YELLOW}跳过服务设置${NC}"
fi

