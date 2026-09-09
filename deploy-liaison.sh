#!/bin/bash

# Liaison 部署脚本
# 功能：
#   1. 构建 Linux 版本的 liaison 和 liaison-edge
#   2. 构建前端（可选）
#   3. 删除远程机器上的旧版本
#   4. 上传新版本到远程机器
#   5. 重启远程服务
#
# 用法：
#   ./deploy-liaison.sh                    # 部署所有（前端、liaison、edge）
#   ./deploy-liaison.sh --web              # 仅部署前端
#   ./deploy-liaison.sh --liaison          # 仅部署 liaison
#   ./deploy-liaison.sh --edge             # 仅部署 edge
#   ./deploy-liaison.sh --web --liaison    # 部署前端和 liaison
#   ./deploy-liaison.sh --liaison-only     # 仅部署 liaison（兼容旧参数）

set -e

# 配置 - Manager
MANAGER_HOST="${MANAGER_HOST:-}"
MANAGER_USER="${MANAGER_USER:-root}"
MANAGER_PORT="${MANAGER_PORT:-22}"
MANAGER_BIN_PATH="/opt/liaison/bin"
MANAGER_WEB_PATH="/opt/liaison/web"
MANAGER_SERVICE="liaison"
MANAGER_BIN="./bin/liaison"

# 配置 - Edge
EDGE_HOST="${EDGE_HOST:-}"
EDGE_USER="${EDGE_USER:-root}"
EDGE_PORT="${EDGE_PORT:-22}"
EDGE_BIN_PATH="/opt/liaison/bin"
EDGE_SERVICE="liaison-edge"
EDGE_BIN="./bin/liaison-edge"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 解析参数
DEPLOY_WEB=false
DEPLOY_LIAISON=false
DEPLOY_EDGE=false
DEPLOY_LIAISON_ONLY=false

# 兼容旧参数 --liaison-only
if [[ "$1" == "--liaison-only" ]] || [[ "$1" == "-l" ]]; then
    DEPLOY_LIAISON=true
    DEPLOY_LIAISON_ONLY=true
    shift
fi

# 解析新参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --web)
            DEPLOY_WEB=true
            shift
            ;;
        --liaison)
            DEPLOY_LIAISON=true
            shift
            ;;
        --edge)
            DEPLOY_EDGE=true
            shift
            ;;
        --liaison-only|-l)
            DEPLOY_LIAISON=true
            DEPLOY_LIAISON_ONLY=true
            shift
            ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  --web              部署前端"
            echo "  --liaison          部署 liaison"
            echo "  --edge             部署 edge"
            echo "  --liaison-only     仅部署 liaison（兼容旧参数）"
            echo "  --help, -h          显示帮助信息"
            echo ""
            echo "示例:"
            echo "  MANAGER_HOST=manager.example.com EDGE_HOST=edge.example.com $0"
            echo "  MANAGER_HOST=manager.example.com $0 --web"
            echo "  $0                  # 部署所有（前端、liaison、edge）"
            echo "  $0 --web            # 仅部署前端"
            echo "  $0 --liaison        # 仅部署 liaison"
            echo "  $0 --web --liaison  # 部署前端和 liaison"
            exit 0
            ;;
        *)
            echo -e "${RED}错误: 未知参数 $1${NC}"
            echo "使用 --help 查看帮助信息"
            exit 1
            ;;
    esac
done

# 如果没有指定任何选项，默认部署所有
if [ "$DEPLOY_WEB" = false ] && [ "$DEPLOY_LIAISON" = false ] && [ "$DEPLOY_EDGE" = false ]; then
    DEPLOY_WEB=true
    DEPLOY_LIAISON=true
    DEPLOY_EDGE=true
fi

# Require only the hosts needed by the selected deployment, before any build or SSH.
if { [ "$DEPLOY_WEB" = true ] || [ "$DEPLOY_LIAISON" = true ]; } && [ -z "$MANAGER_HOST" ]; then
    echo "错误: 请通过 MANAGER_HOST 指定部署目标。" >&2
    exit 1
fi
if [ "$DEPLOY_EDGE" = true ] && [ -z "$EDGE_HOST" ]; then
    echo "错误: 请通过 EDGE_HOST 指定部署目标。" >&2
    exit 1
fi

# 显示部署计划
echo -e "${GREEN}部署计划:${NC}"
if [ "$DEPLOY_WEB" = true ]; then
    echo -e "  ${GREEN}✓${NC} 前端"
fi
if [ "$DEPLOY_LIAISON" = true ]; then
    echo -e "  ${GREEN}✓${NC} Liaison (Manager)"
fi
if [ "$DEPLOY_EDGE" = true ]; then
    echo -e "  ${GREEN}✓${NC} Liaison Edge"
fi
echo ""

# 检查 Docker 是否可用（仅在需要构建时检查）
if [ "$DEPLOY_LIAISON" = true ] || [ "$DEPLOY_EDGE" = true ]; then
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}错误: Docker 未安装或不在 PATH 中${NC}"
        exit 1
    fi

    if ! docker info &> /dev/null; then
        echo -e "${RED}错误: Docker daemon 未运行或无法访问${NC}"
        echo -e "${YELLOW}请确保 Docker Desktop 已启动，或者检查 Docker 权限${NC}"
        exit 1
    fi
fi

# 计算步骤总数
STEP_TOTAL=0
if [ "$DEPLOY_WEB" = true ]; then
    STEP_TOTAL=$((STEP_TOTAL + 2))
fi
if [ "$DEPLOY_LIAISON" = true ]; then
    STEP_TOTAL=$((STEP_TOTAL + 4))
fi
if [ "$DEPLOY_EDGE" = true ]; then
    STEP_TOTAL=$((STEP_TOTAL + 3))
fi

CURRENT_STEP=0

# ========== 构建前端 ==========
if [ "$DEPLOY_WEB" = true ]; then
    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo -e "${YELLOW}[${CURRENT_STEP}/${STEP_TOTAL}] 构建前端...${NC}"
    if ! make build-web; then
        echo -e "${RED}错误: 前端构建失败${NC}"
        exit 1
    fi
    
    if [ ! -d "web/dist" ] || [ -z "$(ls -A web/dist 2>/dev/null)" ]; then
        echo -e "${RED}错误: 前端构建失败，找不到 web/dist 目录或目录为空${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ 前端构建完成${NC}"
fi

# ========== 构建 Liaison ==========
if [ "$DEPLOY_LIAISON" = true ]; then
    CURRENT_STEP=$((CURRENT_STEP + 1))
    if [ "$DEPLOY_LIAISON_ONLY" = true ]; then
        echo -e "${YELLOW}[${CURRENT_STEP}/${STEP_TOTAL}] 构建 Linux 版本的 liaison...${NC}"
        if ! make -f Makefile.local build-liaison-linux; then
            echo -e "${RED}错误: 构建失败${NC}"
            echo -e "${YELLOW}提示: 请检查 Docker 是否正常运行，或者查看上面的错误信息${NC}"
            exit 1
        fi
        
        if [ ! -f "$MANAGER_BIN" ]; then
            echo -e "${RED}错误: 构建失败，找不到 $MANAGER_BIN${NC}"
            exit 1
        fi
    else
        echo -e "${YELLOW}[${CURRENT_STEP}/${STEP_TOTAL}] 构建 Linux 版本的 liaison...${NC}"
        if ! make -f Makefile.local build-liaison-linux; then
            echo -e "${RED}错误: 构建失败${NC}"
            echo -e "${YELLOW}提示: 请检查 Docker 是否正常运行，或者查看上面的错误信息${NC}"
            exit 1
        fi
        
        if [ ! -f "$MANAGER_BIN" ]; then
            echo -e "${RED}错误: 构建失败，找不到 $MANAGER_BIN${NC}"
            exit 1
        fi
    fi
    echo -e "${GREEN}✓ Liaison 构建完成${NC}"
fi

# ========== 构建 Edge ==========
if [ "$DEPLOY_EDGE" = true ]; then
    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo -e "${YELLOW}[${CURRENT_STEP}/${STEP_TOTAL}] 构建 Linux 版本的 liaison-edge...${NC}"
    if ! make -f Makefile.local build-edge-linux; then
        echo -e "${RED}错误: 构建失败${NC}"
        echo -e "${YELLOW}提示: 请检查 Docker 是否正常运行，或者查看上面的错误信息${NC}"
        exit 1
    fi
    
    if [ ! -f "$EDGE_BIN" ]; then
        echo -e "${RED}错误: 构建失败，找不到 $EDGE_BIN${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Edge 构建完成${NC}"
fi

# ========== 部署前端 ==========
if [ "$DEPLOY_WEB" = true ]; then
    echo -e "${GREEN}========== 部署前端 (${MANAGER_HOST}) ==========${NC}"
    
    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo -e "${YELLOW}[${CURRENT_STEP}/${STEP_TOTAL}] 上传前端文件到远程机器...${NC}"
    
    # 创建远程目录（如果不存在）
    ssh -p ${MANAGER_PORT} ${MANAGER_USER}@${MANAGER_HOST} "mkdir -p ${MANAGER_WEB_PATH}" || {
        echo -e "${RED}错误: 无法创建远程目录${NC}"
        exit 1
    }
    
    # 上传前端文件
    rsync -avz --delete -e "ssh -p ${MANAGER_PORT}" web/dist/ ${MANAGER_USER}@${MANAGER_HOST}:${MANAGER_WEB_PATH}/ || {
        # 如果 rsync 不可用，使用 scp
        echo -e "${YELLOW}rsync 不可用，使用 scp 上传...${NC}"
        ssh -p ${MANAGER_PORT} ${MANAGER_USER}@${MANAGER_HOST} "rm -rf ${MANAGER_WEB_PATH}/*" || true
        scp -P ${MANAGER_PORT} -r web/dist/* ${MANAGER_USER}@${MANAGER_HOST}:${MANAGER_WEB_PATH}/ || {
            echo -e "${RED}错误: 前端文件上传失败${NC}"
            exit 1
        }
    }
    
    echo -e "${GREEN}✓ 前端文件上传完成${NC}"
    
    # 如果同时部署了 liaison，需要重启服务以加载新的前端文件
    if [ "$DEPLOY_LIAISON" = true ]; then
        echo -e "${YELLOW}提示: 前端文件已更新，liaison 服务重启后将生效${NC}"
    fi
fi

# ========== 部署 Manager ==========
if [ "$DEPLOY_LIAISON" = true ]; then
    echo -e "${GREEN}========== 部署 Manager (${MANAGER_HOST}) ==========${NC}"

    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo -e "${YELLOW}[${CURRENT_STEP}/${STEP_TOTAL}] 删除 Manager 机器上的旧版本...${NC}"
    ssh -p ${MANAGER_PORT} ${MANAGER_USER}@${MANAGER_HOST} "rm -f ${MANAGER_BIN_PATH}/liaison" || {
        echo -e "${YELLOW}警告: 删除旧版本失败（可能文件不存在）${NC}"
    }
    echo -e "${GREEN}✓ 旧版本已删除${NC}"

    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo -e "${YELLOW}[${CURRENT_STEP}/${STEP_TOTAL}] 上传 Manager 新版本到远程机器...${NC}"
    scp -P ${MANAGER_PORT} ${MANAGER_BIN} ${MANAGER_USER}@${MANAGER_HOST}:${MANAGER_BIN_PATH}/
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ 上传完成${NC}"
    else
        echo -e "${RED}错误: 上传失败${NC}"
        exit 1
    fi

    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo -e "${YELLOW}[${CURRENT_STEP}/${STEP_TOTAL}] 设置执行权限并重启 Manager 服务...${NC}"
    ssh -p ${MANAGER_PORT} ${MANAGER_USER}@${MANAGER_HOST} "chmod +x ${MANAGER_BIN_PATH}/liaison && systemctl restart ${MANAGER_SERVICE}"
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Manager 服务已重启${NC}"
    else
        echo -e "${RED}错误: 重启服务失败${NC}"
        exit 1
    fi

    # 检查服务状态
    echo -e "${YELLOW}检查 Manager 服务状态...${NC}"
    ssh -p ${MANAGER_PORT} ${MANAGER_USER}@${MANAGER_HOST} "systemctl status ${MANAGER_SERVICE} --no-pager -l" || true
fi

# ========== 部署 Edge ==========
if [ "$DEPLOY_EDGE" = true ]; then
    echo -e "${GREEN}========== 部署 Edge (${EDGE_HOST}) ==========${NC}"

    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo -e "${YELLOW}[${CURRENT_STEP}/${STEP_TOTAL}] 删除 Edge 机器上的旧版本...${NC}"
    ssh -p ${EDGE_PORT} ${EDGE_USER}@${EDGE_HOST} "rm -f ${EDGE_BIN_PATH}/liaison-edge" || {
        echo -e "${YELLOW}警告: 删除旧版本失败（可能文件不存在）${NC}"
    }
    echo -e "${GREEN}✓ 旧版本已删除${NC}"

    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo -e "${YELLOW}[${CURRENT_STEP}/${STEP_TOTAL}] 上传 Edge 新版本到远程机器...${NC}"
    scp -P ${EDGE_PORT} ${EDGE_BIN} ${EDGE_USER}@${EDGE_HOST}:${EDGE_BIN_PATH}/
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ 上传完成${NC}"
    else
        echo -e "${RED}错误: 上传失败${NC}"
        exit 1
    fi

    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo -e "${YELLOW}[${CURRENT_STEP}/${STEP_TOTAL}] 设置执行权限并重启 Edge 服务...${NC}"
    ssh -p ${EDGE_PORT} ${EDGE_USER}@${EDGE_HOST} "chmod +x ${EDGE_BIN_PATH}/liaison-edge && systemctl restart ${EDGE_SERVICE}"
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Edge 服务已重启${NC}"
    else
        echo -e "${RED}错误: 重启服务失败${NC}"
        exit 1
    fi

    # 检查服务状态
    echo -e "${YELLOW}检查 Edge 服务状态...${NC}"
    ssh -p ${EDGE_PORT} ${EDGE_USER}@${EDGE_HOST} "systemctl status ${EDGE_SERVICE} --no-pager -l" || true
fi

echo -e "${GREEN}部署完成！${NC}"
