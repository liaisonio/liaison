#!/bin/bash

# Liaison Service Installation Script
# This script installs and configures the Liaison systemd service

set -e

# Detect locale and set language
detect_language() {
    # Check LANG environment variable
    if [[ "$LANG" =~ ^zh ]] || [[ "$LANG" =~ ^.*\.UTF-8$ ]] && [[ "$LANG" =~ zh ]]; then
        echo "zh_CN"
        return
    fi
    
    # Check locale command
    if command -v locale >/dev/null 2>&1; then
        local locale_output=$(locale 2>/dev/null | grep -i "LANG=" | head -1)
        if [[ "$locale_output" =~ zh ]]; then
            echo "zh_CN"
            return
        fi
    fi
    
    # Check system timezone (China timezone)
    if [[ -f /etc/timezone ]]; then
        local tz=$(cat /etc/timezone 2>/dev/null)
        if [[ "$tz" =~ Asia/Shanghai ]] || [[ "$tz" =~ Asia/Beijing ]] || [[ "$tz" =~ Asia/Chongqing ]]; then
            echo "zh_CN"
            return
        fi
    fi
    
    # Default to English
    echo "en_US"
}

LANG_CODE=$(detect_language)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Language strings
if [[ "$LANG_CODE" == "zh_CN" ]]; then
    # Chinese strings
    MSG_INSTALLING="正在安装 Liaison 服务..."
    MSG_MUST_ROOT="此脚本必须以 root 权限运行"
    MSG_CREATING_USER="正在创建服务用户和组..."
    MSG_USER_CREATED="已创建用户:"
    MSG_USER_EXISTS="用户已存在:"
    MSG_CREATING_DIRS="正在创建目录..."
    MSG_INSTALLING_SERVICES="正在安装 systemd 服务文件..."
    MSG_SERVICE_INSTALLED="服务已安装"
    MSG_COPYING_BINARIES="正在复制二进制文件..."
    MSG_BINARIES_COPIED="二进制文件已复制"
    MSG_BINARY_FOUND="二进制文件已找到"
    MSG_WARNING_BIN_NOT_FOUND="警告: bin 目录未找到，跳过二进制文件复制"
    MSG_DETECTING_IP="正在检测公网 IP 地址..."
    MSG_IP_DETECTED="自动检测到的公网 IP:"
    MSG_IP_WARNING="⚠️  如果此 IP 不正确，您可以稍后编辑"
    MSG_IP_MANUAL="警告: 无法自动检测公网 IP。"
    MSG_IP_MANUAL_INSTR="请稍后在 $CONFIG_DIR/liaison.yaml 中手动设置公网 IP。"
    MSG_GENERATING_JWT="正在生成 JWT 密钥..."
    MSG_JWT_GENERATED="JWT 密钥已生成"
    MSG_RENDERING_CONFIG="正在从模板渲染配置文件..."
    MSG_CONFIG_RENDERED="配置文件已渲染"
    MSG_COPYING_WEB="正在复制前端文件..."
    MSG_WEB_COPIED="前端文件已复制"
    MSG_WARNING_WEB_NOT_FOUND="警告: web 目录未找到，跳过前端文件复制"
    MSG_COPYING_EDGE="正在复制 edge 二进制文件和脚本..."
    MSG_EDGE_COPIED="Edge 文件已复制"
    MSG_WARNING_EDGE_NOT_FOUND="警告: edge 目录未找到，跳过 edge 文件复制"
    MSG_GENERATING_CERTS="正在生成 TLS 证书..."
    MSG_CERTS_GENERATED="TLS 证书已生成"
    MSG_CERTS_EXIST="证书已存在，跳过生成"
    MSG_WARNING_OPENSSL_NOT_FOUND="警告: 未找到 openssl，跳过证书生成"
    MSG_WARNING_OPENSSL_INSTALL="您可能需要手动生成证书或安装 openssl"
    MSG_RELOADING_SYSTEMD="正在重新加载 systemd 守护进程..."
    MSG_ENABLING_SERVICES="正在启用服务..."
    MSG_SERVICE_ENABLED="服务已启用"
    MSG_STARTING_SERVICES="正在启动服务..."
    MSG_STARTING_FRONTIER="正在启动 frontier 服务..."
    MSG_FRONTIER_STARTED="frontier 服务已启动"
    MSG_WAITING_FRONTIER="等待 3 秒以便 frontier 初始化..."
    MSG_WARNING_FRONTIER_FAILED="警告: frontier 服务启动失败"
    MSG_STARTING_LIAISON="正在启动 liaison 服务..."
    MSG_LIAISON_STARTED="liaison 服务已启动"
    MSG_WARNING_LIAISON_FAILED="警告: liaison 服务启动失败"
    MSG_GENERATING_PASSWORD="正在生成初始密码..."
    MSG_USER_CREATED_SUCCESS="默认用户创建成功"
    MSG_WARNING_PASSWORD_FAILED="警告: 密码存储到数据库失败"
    MSG_PASSWORD_MANUAL="您可以稍后使用以下命令手动设置密码:"
    MSG_WARNING_PASSWORD_GENERATE="警告: 生成随机密码失败"
    MSG_WARNING_PASSWORD_TOOL="警告: 未找到 password-generator，跳过密码生成"
    MSG_INSTALLATION_COMPLETE="安装成功完成！"
    MSG_INSTALLATION_SUMMARY="安装摘要"
    MSG_DEFAULT_CREDENTIALS="默认凭据:"
    MSG_USERNAME="用户名:"
    MSG_PASSWORD="密码:"
    MSG_PORT_CONFIG="端口配置:"
    MSG_MANAGER_PORT="管理页面端口:"
    MSG_FRONTIER_PORT="Frontier Edge 端口:"
    MSG_PUBLIC_IP="公网 IP 地址 (自动检测):"
    MSG_SERVICE_STATUS="服务状态:"
    MSG_RUNNING="运行中"
    MSG_NOT_RUNNING="未运行"
    MSG_ACCESS_URL="访问地址:"
    MSG_NEXT_STEPS="后续步骤:"
    MSG_REVIEW_CONFIG="1. 检查 $CONFIG_DIR/ 中的配置文件"
    MSG_UPDATE_IP="2. 如果检测到的公网 IP (${PUBLIC_ADDR}) 不正确，"
    MSG_EDIT_CONFIG="   编辑 $CONFIG_DIR/liaison.yaml 并更新 'server_url' 字段"
    MSG_IP_NOT_DETECTED="2. 无法自动检测公网 IP。"
    MSG_EDIT_IP="   请编辑 $CONFIG_DIR/liaison.yaml 并手动设置 'server_url' 字段"
    MSG_CHANGE_PASSWORD="3. ⚠️  请在首次登录后更改默认密码！"
    MSG_CHECK_STATUS="4. 检查服务状态: systemctl status $SERVICE_NAME frontier"
    MSG_VIEW_LOGS="5. 查看日志: journalctl -u $SERVICE_NAME -f (或 journalctl -u frontier -f)"
    MSG_CHECK_STATUS_SINGLE="3. 检查服务状态: systemctl status $SERVICE_NAME"
    MSG_VIEW_LOGS_SINGLE="4. 查看日志: journalctl -u $SERVICE_NAME -f"
    MSG_SERVICE_MANAGEMENT="服务管理命令:"
    MSG_CONFIG_MANAGER_PORT="配置 Liaison 管理页面端口..."
    MSG_ENTER_MANAGER_PORT="请输入 Liaison 管理页面端口号 (默认:"
    MSG_MANAGER_PORT_SET="管理页面将监听端口:"
    MSG_CONFIG_FRONTIER_PORT="配置 Frontier edge 连接端口..."
    MSG_ENTER_FRONTIER_PORT="请输入 Frontier edge 连接端口号 (默认:"
    MSG_FRONTIER_PORT_SET="Frontier edge 将监听端口:"
    MSG_INVALID_PORT="⚠️  输入的端口号无效，使用默认端口"
    MSG_COUNTDOWN="倒计时"
    MSG_USE_DEFAULT="直接回车使用默认值"
    MSG_USING_DEFAULT="使用默认值"
    MSG_PORT_INPUT_HINT="提示: 请输入端口号 (1-65535)，或直接按回车使用默认值"
    MSG_PORT_PROMPT="端口号"
    MSG_CONFIG_PUBLIC_ADDR="配置公网地址或域名..."
    MSG_ENTER_PUBLIC_ADDR="请输入公网 IP 地址或域名 (默认:"
    MSG_PUBLIC_ADDR_SET="公网地址设置为:"
    MSG_ADDR_INPUT_HINT="提示: 请输入公网 IP 地址或域名，或直接按回车使用检测到的 IP"
    MSG_ADDR_PROMPT="公网地址"
    MSG_INPUT_RECEIVED="已接收输入"
else
    # English strings
    MSG_INSTALLING="Installing Liaison Service..."
    MSG_MUST_ROOT="This script must be run as root"
    MSG_CREATING_USER="Creating service user and group..."
    MSG_USER_CREATED="Created user:"
    MSG_USER_EXISTS="User already exists:"
    MSG_CREATING_DIRS="Creating directories..."
    MSG_INSTALLING_SERVICES="Installing systemd service files..."
    MSG_SERVICE_INSTALLED="service installed"
    MSG_COPYING_BINARIES="Copying binaries..."
    MSG_BINARIES_COPIED="Binaries copied"
    MSG_BINARY_FOUND="binary found"
    MSG_WARNING_BIN_NOT_FOUND="Warning: bin directory not found, skipping binary copy"
    MSG_DETECTING_IP="Detecting public IP address..."
    MSG_IP_DETECTED="Auto-detected public IP:"
    MSG_IP_WARNING="⚠️  If this IP is incorrect, you can laterly edit"
    MSG_IP_MANUAL="Warning: Could not detect public IP automatically."
    MSG_IP_MANUAL_INSTR="Please manually set the public IP in $CONFIG_DIR/liaison.yaml after installation."
    MSG_GENERATING_JWT="Generating JWT secret key..."
    MSG_JWT_GENERATED="JWT secret key generated"
    MSG_RENDERING_CONFIG="Rendering configuration files from templates..."
    MSG_CONFIG_RENDERED="rendered"
    MSG_COPYING_WEB="Copying web frontend files..."
    MSG_WEB_COPIED="Web frontend files copied"
    MSG_WARNING_WEB_NOT_FOUND="Warning: web directory not found, skipping web files copy"
    MSG_COPYING_EDGE="Copying edge binaries and scripts for all platforms..."
    MSG_EDGE_COPIED="Edge files copied"
    MSG_WARNING_EDGE_NOT_FOUND="Warning: edge directory not found, skipping edge files copy"
    MSG_GENERATING_CERTS="Generating TLS certificates..."
    MSG_CERTS_GENERATED="TLS certificates generated in"
    MSG_CERTS_EXIST="Certificates already exist, skipping generation"
    MSG_WARNING_OPENSSL_NOT_FOUND="Warning: openssl not found, skipping certificate generation"
    MSG_WARNING_OPENSSL_INSTALL="You may need to generate certificates manually or install openssl"
    MSG_RELOADING_SYSTEMD="Reloading systemd daemon..."
    MSG_ENABLING_SERVICES="Enabling services..."
    MSG_SERVICE_ENABLED="service enabled"
    MSG_STARTING_SERVICES="Starting services..."
    MSG_STARTING_FRONTIER="Starting frontier service..."
    MSG_FRONTIER_STARTED="frontier service started"
    MSG_WAITING_FRONTIER="Waiting 3 seconds for frontier to initialize..."
    MSG_WARNING_FRONTIER_FAILED="Warning: frontier service failed to start"
    MSG_STARTING_LIAISON="Starting liaison service..."
    MSG_LIAISON_STARTED="liaison service started"
    MSG_WARNING_LIAISON_FAILED="Warning: liaison service failed to start"
    MSG_GENERATING_PASSWORD="Generating initial password..."
    MSG_USER_CREATED_SUCCESS="Default user created successfully"
    MSG_WARNING_PASSWORD_FAILED="Warning: Failed to store password in database"
    MSG_PASSWORD_MANUAL="You can manually set the password later using:"
    MSG_WARNING_PASSWORD_GENERATE="Warning: Failed to generate random password"
    MSG_WARNING_PASSWORD_TOOL="Warning: password-generator not found, skipping password generation"
    MSG_INSTALLATION_COMPLETE="Installation completed successfully!"
    MSG_INSTALLATION_SUMMARY="Installation Summary"
    MSG_DEFAULT_CREDENTIALS="Default Credentials:"
    MSG_USERNAME="Username:"
    MSG_PASSWORD="Password:"
    MSG_PORT_CONFIG="Port Configuration:"
    MSG_MANAGER_PORT="Management Page Port:"
    MSG_FRONTIER_PORT="Frontier Edge Port:"
    MSG_PUBLIC_IP="Public IP Address (Auto-detected):"
    MSG_SERVICE_STATUS="Service Status:"
    MSG_RUNNING="Running"
    MSG_NOT_RUNNING="Not running"
    MSG_ACCESS_URL="Access URL:"
    MSG_NEXT_STEPS="Next steps:"
    MSG_REVIEW_CONFIG="1. Review configuration files in $CONFIG_DIR/"
    MSG_UPDATE_IP="2. If the detected public IP (${PUBLIC_ADDR}) is incorrect,"
    MSG_EDIT_CONFIG="   edit $CONFIG_DIR/liaison.yaml and update the 'server_url' field"
    MSG_IP_NOT_DETECTED="2. The public IP could not be auto-detected."
    MSG_EDIT_IP="   Please edit $CONFIG_DIR/liaison.yaml and set the 'server_url' field manually"
    MSG_CHANGE_PASSWORD="3. ⚠️  Please change the default password after first login!"
    MSG_CHECK_STATUS="4. Check service status: systemctl status $SERVICE_NAME frontier"
    MSG_VIEW_LOGS="5. View logs: journalctl -u $SERVICE_NAME -f (or journalctl -u frontier -f)"
    MSG_CHECK_STATUS_SINGLE="3. Check service status: systemctl status $SERVICE_NAME"
    MSG_VIEW_LOGS_SINGLE="4. View logs: journalctl -u $SERVICE_NAME -f"
    MSG_SERVICE_MANAGEMENT="Service management commands:"
    MSG_CONFIG_MANAGER_PORT="Configuring Liaison management page port..."
    MSG_ENTER_MANAGER_PORT="Enter the port number for Liaison management page (default:"
    MSG_MANAGER_PORT_SET="Management page will listen on port:"
    MSG_CONFIG_FRONTIER_PORT="Configuring Frontier edge connection port..."
    MSG_ENTER_FRONTIER_PORT="Enter the port number for Frontier edge connections (default:"
    MSG_FRONTIER_PORT_SET="Frontier edge will listen on port:"
    MSG_INVALID_PORT="⚠️  Invalid port number, using default port"
    MSG_COUNTDOWN="Countdown"
    MSG_USE_DEFAULT="Press Enter to use default"
    MSG_USING_DEFAULT="Using default value"
    MSG_PORT_INPUT_HINT="Hint: Enter a port number (1-65535), or press Enter to use default value"
    MSG_PORT_PROMPT="Port"
    MSG_CONFIG_PUBLIC_ADDR="Configuring public address or domain name..."
    MSG_ENTER_PUBLIC_ADDR="Enter the public IP address or domain name (default:"
    MSG_PUBLIC_ADDR_SET="Public address set to:"
    MSG_ADDR_INPUT_HINT="Hint: Enter a public IP address or domain name, or press Enter to use detected IP"
    MSG_ADDR_PROMPT="Public address"
    MSG_INPUT_RECEIVED="Input received"
fi

# Configuration
SERVICE_NAME="liaison"
SERVICE_USER="liaison"
SERVICE_GROUP="liaison"
DEFAULT_EMAIL="default@liaison.com"
INSTALL_DIR="/opt/liaison"
CONFIG_DIR="/opt/liaison/conf"
DATA_DIR="/opt/liaison/data"
LOG_DIR="/opt/liaison/logs"
BIN_DIR="/opt/liaison/bin"
WEB_DIR="/opt/liaison/web"
EDGE_DIR="/opt/liaison/edge"
CERTS_DIR="/opt/liaison/certs"

echo -e "${GREEN}${MSG_INSTALLING}${NC}"

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}${MSG_MUST_ROOT}${NC}"
   exit 1
fi

# Create service user and group
echo -e "${YELLOW}${MSG_CREATING_USER}${NC}"
if ! id "$SERVICE_USER" &>/dev/null; then
    useradd --system --no-create-home --shell /bin/false "$SERVICE_USER"
    echo -e "${GREEN}${MSG_USER_CREATED} $SERVICE_USER${NC}"
else
    echo -e "${YELLOW}${MSG_USER_EXISTS} $SERVICE_USER${NC}"
fi

# Create directories
echo -e "${YELLOW}${MSG_CREATING_DIRS}${NC}"
mkdir -p "$INSTALL_DIR" "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR" "$BIN_DIR" "$WEB_DIR" "$EDGE_DIR" "$CERTS_DIR"
chown -R "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR"
chmod 755 "$INSTALL_DIR"

# Set specific permissions
chmod 750 "$DATA_DIR" "$LOG_DIR"
chmod 755 "$BIN_DIR" "$CONFIG_DIR" "$WEB_DIR" "$EDGE_DIR"
chmod 750 "$CERTS_DIR"

# Copy service files
echo -e "${YELLOW}${MSG_INSTALLING_SERVICES}${NC}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [[ -d "$SCRIPT_DIR/systemd" ]]; then
    # Copy liaison.service
    if [[ -f "$SCRIPT_DIR/systemd/liaison.service" ]]; then
        cp "$SCRIPT_DIR/systemd/liaison.service" "/etc/systemd/system/"
        chmod 644 "/etc/systemd/system/liaison.service"
        echo -e "${GREEN}liaison.${MSG_SERVICE_INSTALLED}${NC}"
    fi
    # Copy frontier.service
    if [[ -f "$SCRIPT_DIR/systemd/frontier.service" ]]; then
        cp "$SCRIPT_DIR/systemd/frontier.service" "/etc/systemd/system/"
        chmod 644 "/etc/systemd/system/frontier.service"
        echo -e "${GREEN}frontier.${MSG_SERVICE_INSTALLED}${NC}"
    fi
elif [[ -f "$SCRIPT_DIR/liaison.service" ]]; then
    cp "$SCRIPT_DIR/liaison.service" "/etc/systemd/system/"
    chmod 644 "/etc/systemd/system/liaison.service"
else
    echo -e "${RED}Error: liaison.service not found${NC}"
    exit 1
fi

# Copy binaries
echo -e "${YELLOW}${MSG_COPYING_BINARIES}${NC}"
if [[ -d "$SCRIPT_DIR/bin" ]]; then
    cp -f "$SCRIPT_DIR/bin/"* "$BIN_DIR/"
    chown "$SERVICE_USER:$SERVICE_GROUP" "$BIN_DIR"/*
    chmod 755 "$BIN_DIR"/*
    echo -e "${GREEN}${MSG_BINARIES_COPIED}${NC}"
    # Check if frontier binary exists
    if [[ -f "$BIN_DIR/frontier" ]]; then
        echo -e "${GREEN}  - frontier ${MSG_BINARY_FOUND}${NC}"
    fi
else
    echo -e "${YELLOW}${MSG_WARNING_BIN_NOT_FOUND}${NC}"
fi

# Function to read input with countdown
read_with_countdown() {
    local prompt="$1"
    local default_value="$2"
    local input_hint="${3:-$MSG_PORT_INPUT_HINT}"
    local input_prompt="${4:-$MSG_PORT_PROMPT}"
    local timeout=30
    local input=""
    local time_unit="秒"
    local remaining=$timeout
    
    # Set time unit based on language
    if [[ "$LANG_CODE" != "zh_CN" ]]; then
        time_unit="s"
    fi
    
    # Show prominent header (output to stderr so it's visible)
    echo "" >&2
    echo -e "${BOLD}${YELLOW}═══════════════════════════════════════════════════════════════${NC}" >&2
    echo -e "${BOLD}${CYAN}${prompt} ${default_value})${NC}" >&2
    echo -e "${BOLD}${YELLOW}═══════════════════════════════════════════════════════════════${NC}" >&2
    echo -e "${YELLOW}${input_hint}${NC}" >&2
    echo "" >&2
    
    # Save terminal settings
    local old_stty=$(stty -g 2>/dev/null || true)
    
    # Show countdown on a separate line above input (won't interfere with typing)
    echo -e "${YELLOW}${MSG_COUNTDOWN}: ${remaining}${time_unit} (${MSG_USE_DEFAULT})${NC}" >&2
    
    # Show input prompt on the line below countdown
    echo -ne "${BOLD}${CYAN}>>> ${input_prompt} [${default_value}]: ${NC}" >&2
    
    # Use a background process to update countdown (only updates the countdown line)
    # Use save/restore cursor position to avoid affecting input line
    (
        local count=$((timeout - 1))
        while [ $count -gt 0 ]; do
            sleep 1
            # Save cursor position, move up, update countdown, restore cursor
            echo -ne "\033[s\033[1A\033[K${YELLOW}${MSG_COUNTDOWN}: ${count}${time_unit} (${MSG_USE_DEFAULT})${NC}\033[u" >&2
            count=$((count - 1))
        done
        # Final message when timeout
        sleep 1
        echo -ne "\033[s\033[1A\033[K${GREEN}${MSG_USING_DEFAULT} ${default_value}${NC}\033[u" >&2
    ) &
    local countdown_pid=$!
    
    # Read input (with full timeout, from /dev/tty to avoid interference)
    if read -r -t $timeout input </dev/tty 2>/dev/null; then
        # User provided input, kill countdown process
        kill $countdown_pid 2>/dev/null || true
        wait $countdown_pid 2>/dev/null || true
        stty "$old_stty" 2>/dev/null || true
        # Clear countdown line
        echo -ne "\033[1A\033[K" >&2
        echo "" >&2
        # If user just pressed Enter, return default value
        if [ -z "$input" ]; then
            echo -e "${GREEN}✓ ${MSG_USING_DEFAULT} ${default_value}${NC}" >&2
            echo "$default_value"  # Output to stdout for command substitution
        else
            echo -e "${GREEN}✓ ${MSG_INPUT_RECEIVED}: ${input}${NC}" >&2
            echo "$input"  # Output to stdout for command substitution
        fi
        return 0
    else
        # Timeout reached, wait for countdown process to finish
        wait $countdown_pid 2>/dev/null || true
        stty "$old_stty" 2>/dev/null || true
        echo "" >&2
        echo "$default_value"  # Output to stdout for command substitution
        return 0
    fi
}

# Get management page port
echo ""
echo -e "${YELLOW}${MSG_CONFIG_MANAGER_PORT}${NC}"
DEFAULT_MANAGER_PORT="443"
MANAGER_PORT=$(read_with_countdown "${MSG_ENTER_MANAGER_PORT}" "$DEFAULT_MANAGER_PORT")

# Validate port number (1-65535)
# Only show error if user provided a non-empty, invalid value
if [ -z "$MANAGER_PORT" ]; then
    # Empty value, use default
    MANAGER_PORT="$DEFAULT_MANAGER_PORT"
elif ! [[ "$MANAGER_PORT" =~ ^[0-9]+$ ]] || [ "$MANAGER_PORT" -lt 1 ] || [ "$MANAGER_PORT" -gt 65535 ]; then
    # Invalid value provided by user
    echo -e "${RED}${MSG_INVALID_PORT} ${DEFAULT_MANAGER_PORT}${NC}"
    MANAGER_PORT="$DEFAULT_MANAGER_PORT"
fi
echo -e "${GREEN}${MSG_MANAGER_PORT_SET} ${BOLD}${CYAN}${MANAGER_PORT}${NC}${GREEN}${NC}"

# Get Frontier edge port
echo ""
echo -e "${YELLOW}${MSG_CONFIG_FRONTIER_PORT}${NC}"
DEFAULT_FRONTIER_PORT="30012"
FRONTIER_PORT=$(read_with_countdown "${MSG_ENTER_FRONTIER_PORT}" "$DEFAULT_FRONTIER_PORT")

# Validate port number (1-65535)
# Only show error if user provided a non-empty, invalid value
if [ -z "$FRONTIER_PORT" ]; then
    # Empty value, use default
    FRONTIER_PORT="$DEFAULT_FRONTIER_PORT"
elif ! [[ "$FRONTIER_PORT" =~ ^[0-9]+$ ]] || [ "$FRONTIER_PORT" -lt 1 ] || [ "$FRONTIER_PORT" -gt 65535 ]; then
    # Invalid value provided by user
    echo -e "${RED}${MSG_INVALID_PORT} ${DEFAULT_FRONTIER_PORT}${NC}"
    FRONTIER_PORT="$DEFAULT_FRONTIER_PORT"
fi
echo -e "${GREEN}${MSG_FRONTIER_PORT_SET} ${BOLD}${CYAN}${FRONTIER_PORT}${NC}${GREEN}${NC}"
FRONTIER_CONTROLPLANE_PORT="${FRONTIER_CONTROLPLANE_PORT:-30010}"

# Get public IP address or domain
echo ""
echo -e "${YELLOW}${MSG_DETECTING_IP}${NC}"
DETECTED_PUBLIC_ADDR=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || curl -s --max-time 5 ifconfig.co 2>/dev/null || echo "")
if [ -z "$DETECTED_PUBLIC_ADDR" ] || [ "$DETECTED_PUBLIC_ADDR" = "localhost" ]; then
    echo -e "${YELLOW}${MSG_IP_MANUAL}${NC}"
    echo -e "${YELLOW}${MSG_IP_MANUAL_INSTR}${NC}"
    PUBLIC_ADDR="localhost"
else
    echo -e "${GREEN}${MSG_IP_DETECTED} ${BOLD}${CYAN}$DETECTED_PUBLIC_ADDR${NC}${GREEN}${NC}"
    # Prompt user to input public address (IP or domain) with countdown
    echo ""
    echo -e "${YELLOW}${MSG_CONFIG_PUBLIC_ADDR}${NC}"
    PUBLIC_ADDR=$(read_with_countdown "${MSG_ENTER_PUBLIC_ADDR}" "$DETECTED_PUBLIC_ADDR" "$MSG_ADDR_INPUT_HINT" "$MSG_ADDR_PROMPT")
    # If user input is empty, use detected IP
    if [ -z "$PUBLIC_ADDR" ]; then
        PUBLIC_ADDR="$DETECTED_PUBLIC_ADDR"
    fi
    echo -e "${GREEN}${MSG_PUBLIC_ADDR_SET} ${BOLD}${CYAN}${PUBLIC_ADDR}${NC}${GREEN}${NC}"
    echo -e "${YELLOW}${MSG_IP_WARNING} $CONFIG_DIR/liaison.yaml ${NC}"
fi

# Generate server URL based on port
if [[ "$MANAGER_PORT" == "443" ]]; then
    SERVER_URL="https://${PUBLIC_ADDR}"
elif [[ "$MANAGER_PORT" == "80" ]]; then
    SERVER_URL="http://${PUBLIC_ADDR}"
else
    SERVER_URL="https://${PUBLIC_ADDR}:${MANAGER_PORT}"
fi

# Generate random JWT secret (32 characters minimum for security)
echo -e "${YELLOW}${MSG_GENERATING_JWT}${NC}"
JWT_SECRET=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32 2>/dev/null)
if [[ -z "$JWT_SECRET" ]]; then
    JWT_SECRET=$(head -c 32 /dev/urandom | base64 | tr -d "=+/" | cut -c1-32 2>/dev/null)
fi
if [[ -z "$JWT_SECRET" ]]; then
    JWT_SECRET=$(date +%s | sha256sum | base64 | tr -d "=+/" | cut -c1-32)
fi
if [[ ${#JWT_SECRET} -lt 32 ]]; then
    # Pad to 32 characters if needed
    JWT_SECRET="${JWT_SECRET}$(openssl rand -hex 16 | head -c $((32 - ${#JWT_SECRET})))"
fi
echo -e "${GREEN}${MSG_JWT_GENERATED}${NC}"

# Render configuration templates
echo -e "${YELLOW}${MSG_RENDERING_CONFIG}${NC}"
if [[ -d "$SCRIPT_DIR/conf" ]]; then
    # Render liaison.yaml from template
    if [[ -f "$SCRIPT_DIR/conf/liaison.yaml.template" ]]; then
        # Replace ${PUBLIC_ADDR}, ${JWT_SECRET}, ${MANAGER_PORT}, ${FRONTIER_PORT}, ${FRONTIER_CONTROLPLANE_PORT}, and ${SERVER_URL} in the template
        sed -e "s|\${PUBLIC_ADDR}|${PUBLIC_ADDR}|g" \
            -e "s|\${JWT_SECRET}|${JWT_SECRET}|g" \
            -e "s|\${MANAGER_PORT}|${MANAGER_PORT}|g" \
            -e "s|\${FRONTIER_PORT}|${FRONTIER_PORT}|g" \
            -e "s|\${FRONTIER_CONTROLPLANE_PORT}|${FRONTIER_CONTROLPLANE_PORT}|g" \
            -e "s|\${SERVER_URL}|${SERVER_URL}|g" \
            "$SCRIPT_DIR/conf/liaison.yaml.template" > "$CONFIG_DIR/liaison.yaml"
        chown "$SERVICE_USER:$SERVICE_GROUP" "$CONFIG_DIR/liaison.yaml"
        chmod 644 "$CONFIG_DIR/liaison.yaml"
        echo -e "${GREEN}liaison.yaml ${MSG_CONFIG_RENDERED} with public IP: ${BOLD}${CYAN}$PUBLIC_ADDR${NC}${GREEN}${NC}"
        echo -e "${GREEN}${MSG_JWT_GENERATED}${NC}"
    fi
    
    # Render frontier.yaml from template
    if [[ -f "$SCRIPT_DIR/conf/frontier.yaml.template" ]]; then
        # Replace ${PUBLIC_ADDR}, ${FRONTIER_PORT}, and ${FRONTIER_CONTROLPLANE_PORT} if they exist in the template
        sed -e "s|\${PUBLIC_ADDR}|${PUBLIC_ADDR}|g" \
            -e "s|\${FRONTIER_PORT}|${FRONTIER_PORT}|g" \
            -e "s|\${FRONTIER_CONTROLPLANE_PORT}|${FRONTIER_CONTROLPLANE_PORT}|g" \
            "$SCRIPT_DIR/conf/frontier.yaml.template" > "$CONFIG_DIR/frontier.yaml"
        chown "$SERVICE_USER:$SERVICE_GROUP" "$CONFIG_DIR/frontier.yaml"
        chmod 644 "$CONFIG_DIR/frontier.yaml"
        echo -e "${GREEN}frontier.yaml ${MSG_CONFIG_RENDERED}${NC}"
    fi
fi

# Copy configuration files (fallback if templates don't exist)
if [[ -d "$SCRIPT_DIR/etc" ]]; then
    for yaml_file in "$SCRIPT_DIR/etc/"*.yaml; do
        if [[ -f "$yaml_file" ]]; then
            filename=$(basename "$yaml_file")
            # Only copy if not already rendered from template
            if [[ ! -f "$CONFIG_DIR/$filename" ]]; then
                cp -f "$yaml_file" "$CONFIG_DIR/"
                chown "$SERVICE_USER:$SERVICE_GROUP" "$CONFIG_DIR/$filename"
                chmod 644 "$CONFIG_DIR/$filename"
            fi
        fi
    done
    if [[ -f "$CONFIG_DIR"/*.yaml ]]; then
        echo -e "${GREEN}Configuration files copied${NC}"
    fi
fi

# Copy web frontend files
echo -e "${YELLOW}${MSG_COPYING_WEB}${NC}"
if [[ -d "$SCRIPT_DIR/web" ]]; then
    cp -r "$SCRIPT_DIR/web/"* "$WEB_DIR/" 2>/dev/null || true
    chown -R "$SERVICE_USER:$SERVICE_GROUP" "$WEB_DIR"
    chmod -R 755 "$WEB_DIR"
    echo -e "${GREEN}${MSG_WEB_COPIED}${NC}"
else
    echo -e "${YELLOW}${MSG_WARNING_WEB_NOT_FOUND}${NC}"
fi

# Copy edge binaries and scripts for all platforms
echo -e "${YELLOW}${MSG_COPYING_EDGE}${NC}"
if [[ -d "$SCRIPT_DIR/edge" ]]; then
    cp -f "$SCRIPT_DIR/edge/"* "$EDGE_DIR/" 2>/dev/null || true
    chown "$SERVICE_USER:$SERVICE_GROUP" "$EDGE_DIR"/* 2>/dev/null || true
    # Set executable permission for scripts, read-only for tar.gz files
    chmod 755 "$EDGE_DIR"/install.sh 2>/dev/null || true
    chmod 755 "$EDGE_DIR"/uninstall.sh 2>/dev/null || true
    chmod 644 "$EDGE_DIR"/*.tar.gz 2>/dev/null || true
    echo -e "${GREEN}${MSG_EDGE_COPIED}${NC}"
else
    echo -e "${YELLOW}${MSG_WARNING_EDGE_NOT_FOUND}${NC}"
fi

# Generate TLS certificates
echo -e "${YELLOW}${MSG_GENERATING_CERTS}${NC}"
if command -v openssl >/dev/null 2>&1; then
    if [[ ! -f "$CERTS_DIR/server.crt" ]] || [[ ! -f "$CERTS_DIR/server.key" ]]; then
        openssl req -x509 -newkey rsa:4096 \
            -keyout "$CERTS_DIR/server.key" \
            -out "$CERTS_DIR/server.crt" \
            -days 365 \
            -nodes \
            -subj "/C=CN/ST=Beijing/L=Beijing/O=Liaison/OU=IT/CN=localhost" \
            2>/dev/null
        chmod 600 "$CERTS_DIR/server.key"
        chmod 644 "$CERTS_DIR/server.crt"
        chown "$SERVICE_USER:$SERVICE_GROUP" "$CERTS_DIR/server.key" "$CERTS_DIR/server.crt"
        echo -e "${GREEN}${MSG_CERTS_GENERATED} $CERTS_DIR${NC}"
    else
        echo -e "${YELLOW}${MSG_CERTS_EXIST}${NC}"
    fi
else
    echo -e "${YELLOW}${MSG_WARNING_OPENSSL_NOT_FOUND}${NC}"
    echo -e "${YELLOW}${MSG_WARNING_OPENSSL_INSTALL}${NC}"
fi

# Generate initial password
# Reload systemd
echo -e "${YELLOW}${MSG_RELOADING_SYSTEMD}${NC}"
systemctl daemon-reload

# Enable services
echo -e "${YELLOW}${MSG_ENABLING_SERVICES}${NC}"
systemctl enable "$SERVICE_NAME"
if [[ -f "/etc/systemd/system/frontier.service" ]]; then
    systemctl enable frontier
    echo -e "${GREEN}frontier ${MSG_SERVICE_ENABLED}${NC}"
fi

# Start services in order: frontier first, then liaison
echo -e "${YELLOW}${MSG_STARTING_SERVICES}${NC}"
LIAISON_STARTED=false
FRONTIER_STARTED=false
if [[ -f "/etc/systemd/system/frontier.service" ]]; then
    echo -e "${YELLOW}${MSG_STARTING_FRONTIER}${NC}"
    # Reload systemd to pick up any service file changes
    systemctl daemon-reload
    systemctl start frontier
    sleep 2  # Give service a moment to start
    if systemctl is-active --quiet frontier; then
        echo -e "${GREEN}${MSG_FRONTIER_STARTED}${NC}"
        FRONTIER_STARTED=true
        echo -e "${YELLOW}${MSG_WAITING_FRONTIER}${NC}"
        sleep 3
    else
        echo -e "${RED}${MSG_WARNING_FRONTIER_FAILED}${NC}"
        systemctl status frontier --no-pager -l || true
        echo -e "${YELLOW}Trying to check logs: journalctl -u frontier -n 20 --no-pager${NC}"
        journalctl -u frontier -n 20 --no-pager || true
    fi
fi

echo -e "${YELLOW}${MSG_STARTING_LIAISON}${NC}"
# Reload systemd to pick up any service file changes
systemctl daemon-reload
systemctl start "$SERVICE_NAME"
sleep 2  # Give service a moment to start

# Wait for liaison service to be fully active and database initialized
MAX_WAIT=30
WAIT_COUNT=0
while [[ $WAIT_COUNT -lt $MAX_WAIT ]]; do
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        # Check if database file exists (liaison creates it on startup)
        DB_FILE="/opt/liaison/data/liaison.db"
        if [[ -f "$DB_FILE" ]]; then
            # Give it a bit more time to ensure database schema is initialized
            sleep 2
            break
        fi
    fi
    sleep 1
    WAIT_COUNT=$((WAIT_COUNT + 1))
done

if systemctl is-active --quiet "$SERVICE_NAME"; then
    echo -e "${GREEN}${MSG_LIAISON_STARTED}${NC}"
    LIAISON_STARTED=true
    
    # Now create default user and password after liaison service is running
    echo -e "${YELLOW}${MSG_GENERATING_PASSWORD}${NC}"
    INITIAL_PASSWORD=""
    # Temporarily disable exit on error for password generation
    set +e
    if [[ -f "$BIN_DIR/password-generator" ]]; then
        # Generate random password directly in the script
        # Using /dev/urandom for better randomness
        INITIAL_PASSWORD=$(openssl rand -base64 16 | tr -d "=+/" | cut -c1-16 2>/dev/null)
        if [[ -z "$INITIAL_PASSWORD" ]]; then
            # Fallback: use /dev/urandom if openssl is not available
            INITIAL_PASSWORD=$(head -c 16 /dev/urandom | base64 | tr -d "=+/" | cut -c1-16 2>/dev/null)
        fi
        if [[ -z "$INITIAL_PASSWORD" ]]; then
            # Last resort: use date + random number
            INITIAL_PASSWORD="$(date +%s | sha256sum | base64 | tr -d "=+/" | cut -c1-16)"
        fi
        
        if [[ -n "$INITIAL_PASSWORD" ]]; then
            # Use password-generator to hash and store the password
            # Wait a bit more to ensure database is fully ready
            sleep 1
            "$BIN_DIR/password-generator" -password "$INITIAL_PASSWORD" -email "$DEFAULT_EMAIL" -create >/dev/null 2>&1
            PASSWORD_EXIT_CODE=$?
            set -e  # Re-enable exit on error
            
            if [[ $PASSWORD_EXIT_CODE -eq 0 ]]; then
                echo -e "${GREEN}${MSG_USER_CREATED_SUCCESS}${NC}"
            else
                echo -e "${YELLOW}${MSG_WARNING_PASSWORD_FAILED}${NC}"
                echo -e "${YELLOW}${MSG_PASSWORD_MANUAL}${NC}"
                echo -e "${YELLOW}  $BIN_DIR/password-generator -password <password> -email $DEFAULT_EMAIL -create${NC}"
                INITIAL_PASSWORD=""  # Clear password if creation failed
            fi
        else
            set -e  # Re-enable exit on error
            echo -e "${YELLOW}${MSG_WARNING_PASSWORD_GENERATE}${NC}"
        fi
    else
        set -e  # Re-enable exit on error
        echo -e "${YELLOW}${MSG_WARNING_PASSWORD_TOOL}${NC}"
    fi
else
    echo -e "${RED}${MSG_WARNING_LIAISON_FAILED}${NC}"
    systemctl status "$SERVICE_NAME" --no-pager -l || true
    echo -e "${YELLOW}Trying to check logs: journalctl -u $SERVICE_NAME -n 20 --no-pager${NC}"
    journalctl -u "$SERVICE_NAME" -n 20 --no-pager || true
fi

# Check frontier status
if [[ -f "/etc/systemd/system/frontier.service" ]]; then
    if systemctl is-active --quiet frontier; then
        FRONTIER_STARTED=true
    fi
fi

echo -e "${GREEN}${MSG_INSTALLATION_COMPLETE}${NC}"
echo ""
echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}${GREEN}  ${MSG_INSTALLATION_SUMMARY}${NC}"
echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
if [[ -n "$INITIAL_PASSWORD" ]]; then
    echo -e "${BOLD}${YELLOW}  ${MSG_DEFAULT_CREDENTIALS}${NC}"
    echo -e "${BOLD}${CYAN}    ${MSG_USERNAME} ${DEFAULT_EMAIL}${NC}"
    echo -e "${BOLD}${CYAN}    ${MSG_PASSWORD} ${INITIAL_PASSWORD}${NC}"
    echo ""
fi
echo -e "${BOLD}${YELLOW}  ${MSG_PORT_CONFIG}${NC}"
echo -e "${BOLD}${CYAN}    ${MSG_MANAGER_PORT} ${MANAGER_PORT}${NC}"
echo -e "${BOLD}${CYAN}    ${MSG_FRONTIER_PORT} ${FRONTIER_PORT}${NC}"
echo ""
echo -e "${BOLD}${YELLOW}  ${MSG_PUBLIC_IP}${NC}"
echo -e "${BOLD}${CYAN}    ${PUBLIC_ADDR}${NC}"
if [[ "$PUBLIC_ADDR" != "localhost" ]]; then
    echo -e "${YELLOW}    ${MSG_IP_WARNING}${NC}"
    echo -e "${YELLOW}       $CONFIG_DIR/liaison.yaml${NC}"
    echo -e "${YELLOW}       ${MSG_EDIT_CONFIG}${NC}"
else
    echo -e "${YELLOW}    ${MSG_IP_NOT_DETECTED}${NC}"
    echo -e "${YELLOW}       ${MSG_EDIT_IP}${NC}"
fi
echo ""
echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
# Service status
echo -e "${BOLD}${YELLOW}  ${MSG_SERVICE_STATUS}${NC}"
if [[ "$FRONTIER_STARTED" == "true" ]]; then
    echo -e "${GREEN}    ✓ frontier: ${MSG_RUNNING}${NC}"
else
    echo -e "${YELLOW}    ✗ frontier: ${MSG_NOT_RUNNING}${NC}"
fi
if [[ "$LIAISON_STARTED" == "true" ]]; then
    echo -e "${GREEN}    ✓ liaison: ${MSG_RUNNING}${NC}"
else
    echo -e "${YELLOW}    ✗ liaison: ${MSG_NOT_RUNNING}${NC}"
fi
echo ""
# Access information
if [[ "$LIAISON_STARTED" == "true" ]]; then
    if [[ "$PUBLIC_ADDR" != "localhost" ]]; then
        echo -e "${BOLD}${YELLOW}  ${MSG_ACCESS_URL}${NC}"
        if [[ "$MANAGER_PORT" == "443" ]]; then
            echo -e "${BOLD}${CYAN}    https://${PUBLIC_ADDR}${NC}"
        elif [[ "$MANAGER_PORT" == "80" ]]; then
            echo -e "${BOLD}${CYAN}    http://${PUBLIC_ADDR}${NC}"
        else
            echo -e "${BOLD}${CYAN}    https://${PUBLIC_ADDR}:${MANAGER_PORT}${NC}"
        fi
        echo ""
    else
        echo -e "${BOLD}${YELLOW}  Access URL (Local):${NC}"
        if [[ "$MANAGER_PORT" == "443" ]]; then
            echo -e "${BOLD}${CYAN}    https://localhost${NC}"
        elif [[ "$MANAGER_PORT" == "80" ]]; then
            echo -e "${BOLD}${CYAN}    http://localhost${NC}"
        else
            echo -e "${BOLD}${CYAN}    https://localhost:${MANAGER_PORT}${NC}"
        fi
        echo ""
    fi
fi
echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${YELLOW}${MSG_NEXT_STEPS}${NC}"
echo "${MSG_REVIEW_CONFIG}"
if [[ "$PUBLIC_ADDR" != "localhost" ]]; then
    echo "${MSG_UPDATE_IP}"
    echo "${MSG_EDIT_CONFIG}"
else
    echo "${MSG_IP_NOT_DETECTED}"
    echo "${MSG_EDIT_IP}"
fi
if [[ -n "$INITIAL_PASSWORD" ]]; then
    echo "${MSG_CHANGE_PASSWORD}"
fi
if [[ -f "/etc/systemd/system/frontier.service" ]]; then
    echo "${MSG_CHECK_STATUS}"
    echo "${MSG_VIEW_LOGS}"
else
    echo "${MSG_CHECK_STATUS_SINGLE}"
    echo "${MSG_VIEW_LOGS_SINGLE}"
fi
echo ""
echo -e "${GREEN}${MSG_SERVICE_MANAGEMENT}${NC}"
if [[ -f "/etc/systemd/system/frontier.service" ]]; then
    echo "  Start:   systemctl start $SERVICE_NAME frontier"
    echo "  Stop:    systemctl stop $SERVICE_NAME frontier"
    echo "  Restart: systemctl restart $SERVICE_NAME frontier"
    echo "  Status:  systemctl status $SERVICE_NAME frontier"
    echo "  Logs:    journalctl -u $SERVICE_NAME -f (or journalctl -u frontier -f)"
else
    echo "  Start:   systemctl start $SERVICE_NAME"
    echo "  Stop:    systemctl stop $SERVICE_NAME"
    echo "  Restart: systemctl restart $SERVICE_NAME"
    echo "  Status:  systemctl status $SERVICE_NAME"
    echo "  Logs:    journalctl -u $SERVICE_NAME -f"
fi
