#!/bin/bash

# Liaison Package Script
# This script packages all binaries, configs, services, frontend, and edge binaries into a tar.gz

set -e

# Disable copying extended attributes on macOS
export COPYFILE_DISABLE=1

# Detect tar command: prefer gtar (GNU tar) if available, otherwise use tar
if command -v gtar >/dev/null 2>&1; then
    TAR_CMD="gtar"
else
    TAR_CMD="tar"
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get version
VERSION=$(cat VERSION 2>/dev/null || echo "unknown")
PACK_DIR="liaison-${VERSION}-linux-amd64"

echo -e "${GREEN}Packaging Liaison ${VERSION}...${NC}"

# Check required files
if [ ! -f "bin/liaison" ]; then
    echo -e "${RED}Error: bin/liaison not found. Please run 'make build-linux' first.${NC}"
    exit 1
fi

if [ ! -f "bin/liaison-edge" ]; then
    echo -e "${RED}Error: bin/liaison-edge not found. Please run 'make build-linux' first.${NC}"
    exit 1
fi

# Check edge packages (at least one should exist)
if [ ! -f "packages/edge/liaison-edge-linux-amd64.tar.gz" ] && \
   [ ! -f "packages/edge/liaison-edge-linux-arm64.tar.gz" ] && \
   [ ! -f "packages/edge/liaison-edge-darwin-amd64.tar.gz" ] && \
   [ ! -f "packages/edge/liaison-edge-darwin-arm64.tar.gz" ] && \
   [ ! -f "packages/edge/liaison-edge-windows-amd64.tar.gz" ]; then
    echo -e "${YELLOW}Warning: No edge packages found in packages/edge/. Run 'make package-edge-all' first.${NC}"
fi

# Clean up old package
rm -rf "$PACK_DIR" "${PACK_DIR}.tar.gz"

# Create package directory structure
echo -e "${YELLOW}Creating package directory structure...${NC}"
mkdir -p "$PACK_DIR/bin" \
         "$PACK_DIR/etc" \
         "$PACK_DIR/systemd" \
         "$PACK_DIR/web" \
         "$PACK_DIR/edge" \
         "$PACK_DIR/conf"

# Copy binaries (exclude extended attributes)
echo -e "${YELLOW}Copying binaries...${NC}"
# Use cp -X on macOS to exclude extended attributes, or regular cp on Linux
if [[ "$(uname)" == "Darwin" ]]; then
    cp -X bin/liaison "$PACK_DIR/bin/" 2>/dev/null || cp bin/liaison "$PACK_DIR/bin/"
    cp -X bin/liaison-edge "$PACK_DIR/bin/" 2>/dev/null || cp bin/liaison-edge "$PACK_DIR/bin/"
    if [ -f "bin/frontier" ]; then
        cp -X bin/frontier "$PACK_DIR/bin/" 2>/dev/null || cp bin/frontier "$PACK_DIR/bin/"
        echo -e "${GREEN}  - frontier${NC}"
    else
        echo -e "${YELLOW}Warning: bin/frontier not found, skipping${NC}"
    fi
    if [ -f "bin/password-generator" ]; then
        cp -X bin/password-generator "$PACK_DIR/bin/" 2>/dev/null || cp bin/password-generator "$PACK_DIR/bin/"
        echo -e "${GREEN}  - password-generator${NC}"
    fi
else
    cp bin/liaison "$PACK_DIR/bin/"
    cp bin/liaison-edge "$PACK_DIR/bin/"
    if [ -f "bin/frontier" ]; then
        cp bin/frontier "$PACK_DIR/bin/"
        echo -e "${GREEN}  - frontier${NC}"
    else
        echo -e "${YELLOW}Warning: bin/frontier not found, skipping${NC}"
    fi
    if [ -f "bin/password-generator" ]; then
        cp bin/password-generator "$PACK_DIR/bin/"
        echo -e "${GREEN}  - password-generator${NC}"
    fi
fi
# Remove any macOS resource fork files that might have been copied
find "$PACK_DIR/bin" -name "._*" -delete 2>/dev/null || true

# Copy edge packages (tar.gz files) for all platforms
echo -e "${YELLOW}Copying edge packages for all platforms...${NC}"
if [[ "$(uname)" == "Darwin" ]]; then
    cp -X packages/edge/liaison-edge-linux-amd64.tar.gz "$PACK_DIR/edge/" 2>/dev/null && echo -e "${GREEN}  - liaison-edge-linux-amd64.tar.gz${NC}" || true
    cp -X packages/edge/liaison-edge-linux-arm64.tar.gz "$PACK_DIR/edge/" 2>/dev/null && echo -e "${GREEN}  - liaison-edge-linux-arm64.tar.gz${NC}" || true
    cp -X packages/edge/liaison-edge-darwin-amd64.tar.gz "$PACK_DIR/edge/" 2>/dev/null && echo -e "${GREEN}  - liaison-edge-darwin-amd64.tar.gz${NC}" || true
    cp -X packages/edge/liaison-edge-darwin-arm64.tar.gz "$PACK_DIR/edge/" 2>/dev/null && echo -e "${GREEN}  - liaison-edge-darwin-arm64.tar.gz${NC}" || true
    cp -X packages/edge/liaison-edge-windows-amd64.tar.gz "$PACK_DIR/edge/" 2>/dev/null && echo -e "${GREEN}  - liaison-edge-windows-amd64.tar.gz${NC}" || true
    # Copy edge install and uninstall scripts
    if [ -f "dist/edge/install.sh" ]; then
        cp -X dist/edge/install.sh "$PACK_DIR/edge/" 2>/dev/null || cp dist/edge/install.sh "$PACK_DIR/edge/"
        echo -e "${GREEN}  - install.sh${NC}"
    fi
    if [ -f "dist/edge/uninstall.sh" ]; then
        cp -X dist/edge/uninstall.sh "$PACK_DIR/edge/" 2>/dev/null || cp dist/edge/uninstall.sh "$PACK_DIR/edge/"
        echo -e "${GREEN}  - uninstall.sh${NC}"
    fi
    # Copy Windows install and uninstall scripts
    if [ -f "dist/edge/install.ps1" ]; then
        cp -X dist/edge/install.ps1 "$PACK_DIR/edge/" 2>/dev/null || cp dist/edge/install.ps1 "$PACK_DIR/edge/"
        echo -e "${GREEN}  - install.ps1${NC}"
    fi
    if [ -f "dist/edge/install.bat" ]; then
        cp -X dist/edge/install.bat "$PACK_DIR/edge/" 2>/dev/null || cp dist/edge/install.bat "$PACK_DIR/edge/"
        echo -e "${GREEN}  - install.bat${NC}"
    fi
    if [ -f "dist/edge/uninstall.ps1" ]; then
        cp -X dist/edge/uninstall.ps1 "$PACK_DIR/edge/" 2>/dev/null || cp dist/edge/uninstall.ps1 "$PACK_DIR/edge/"
        echo -e "${GREEN}  - uninstall.ps1${NC}"
    fi
else
    cp packages/edge/liaison-edge-linux-amd64.tar.gz "$PACK_DIR/edge/" 2>/dev/null && echo -e "${GREEN}  - liaison-edge-linux-amd64.tar.gz${NC}" || true
    cp packages/edge/liaison-edge-linux-arm64.tar.gz "$PACK_DIR/edge/" 2>/dev/null && echo -e "${GREEN}  - liaison-edge-linux-arm64.tar.gz${NC}" || true
    cp packages/edge/liaison-edge-darwin-amd64.tar.gz "$PACK_DIR/edge/" 2>/dev/null && echo -e "${GREEN}  - liaison-edge-darwin-amd64.tar.gz${NC}" || true
    cp packages/edge/liaison-edge-darwin-arm64.tar.gz "$PACK_DIR/edge/" 2>/dev/null && echo -e "${GREEN}  - liaison-edge-darwin-arm64.tar.gz${NC}" || true
    cp packages/edge/liaison-edge-windows-amd64.tar.gz "$PACK_DIR/edge/" 2>/dev/null && echo -e "${GREEN}  - liaison-edge-windows-amd64.tar.gz${NC}" || true
    # Copy edge install and uninstall scripts
    if [ -f "dist/edge/install.sh" ]; then
        cp dist/edge/install.sh "$PACK_DIR/edge/"
        echo -e "${GREEN}  - install.sh${NC}"
    fi
    if [ -f "dist/edge/uninstall.sh" ]; then
        cp dist/edge/uninstall.sh "$PACK_DIR/edge/"
        echo -e "${GREEN}  - uninstall.sh${NC}"
    fi
    # Copy Windows install and uninstall scripts
    if [ -f "dist/edge/install.ps1" ]; then
        cp dist/edge/install.ps1 "$PACK_DIR/edge/"
        echo -e "${GREEN}  - install.ps1${NC}"
    fi
    if [ -f "dist/edge/install.bat" ]; then
        cp dist/edge/install.bat "$PACK_DIR/edge/"
        echo -e "${GREEN}  - install.bat${NC}"
    fi
    if [ -f "dist/edge/uninstall.ps1" ]; then
        cp dist/edge/uninstall.ps1 "$PACK_DIR/edge/"
        echo -e "${GREEN}  - uninstall.ps1${NC}"
    fi
fi

# Copy frontend files
echo -e "${YELLOW}Copying frontend files...${NC}"
if [ ! -d "web/dist" ]; then
    echo -e "${RED}Error: web/dist directory not found. Please run 'make build-web' first.${NC}"
    exit 1
fi
if [ ! "$(ls -A web/dist 2>/dev/null)" ]; then
    echo -e "${RED}Error: web/dist directory is empty. Please run 'make build-web' first.${NC}"
    exit 1
fi
# Use rsync if available (best option for excluding files)
if command -v rsync >/dev/null 2>&1; then
    rsync -av --exclude='._*' --exclude='.DS_Store' web/dist/ "$PACK_DIR/web/" 2>/dev/null
else
    # Fallback: copy and then clean up (COPYFILE_DISABLE is already exported)
    cp -r web/dist/* "$PACK_DIR/web/" 2>/dev/null || cp -r web/dist/* "$PACK_DIR/web/" 2>/dev/null
    find "$PACK_DIR/web" -name "._*" -delete 2>/dev/null || true
    find "$PACK_DIR/web" -name ".DS_Store" -delete 2>/dev/null || true
fi
echo -e "${GREEN}Frontend files copied${NC}"

# Copy configuration templates
echo -e "${YELLOW}Copying configuration templates...${NC}"
if [[ "$(uname)" == "Darwin" ]]; then
    if [ -f "dist/liaison/conf/liaison.yaml.template" ]; then
        cp -X dist/liaison/conf/liaison.yaml.template "$PACK_DIR/conf/" 2>/dev/null || cp dist/liaison/conf/liaison.yaml.template "$PACK_DIR/conf/"
    fi
    if [ -f "dist/liaison/conf/frontier.yaml.template" ]; then
        cp -X dist/liaison/conf/frontier.yaml.template "$PACK_DIR/conf/" 2>/dev/null || cp dist/liaison/conf/frontier.yaml.template "$PACK_DIR/conf/"
    fi
    if [ -f "etc/liaison.yaml" ]; then
        cp -X etc/liaison.yaml "$PACK_DIR/etc/" 2>/dev/null || cp etc/liaison.yaml "$PACK_DIR/etc/"
    fi
    if [ -f "etc/liaison-edge.yaml" ]; then
        cp -X etc/liaison-edge.yaml "$PACK_DIR/etc/" 2>/dev/null || cp etc/liaison-edge.yaml "$PACK_DIR/etc/"
    fi
else
    if [ -f "dist/liaison/conf/liaison.yaml.template" ]; then
        cp dist/liaison/conf/liaison.yaml.template "$PACK_DIR/conf/"
    fi
    if [ -f "dist/liaison/conf/frontier.yaml.template" ]; then
        cp dist/liaison/conf/frontier.yaml.template "$PACK_DIR/conf/"
    fi
    if [ -f "etc/liaison.yaml" ]; then
        cp etc/liaison.yaml "$PACK_DIR/etc/"
    fi
    if [ -f "etc/liaison-edge.yaml" ]; then
        cp etc/liaison-edge.yaml "$PACK_DIR/etc/"
    fi
fi

# Copy systemd service files
echo -e "${YELLOW}Copying systemd service files...${NC}"
if [[ "$(uname)" == "Darwin" ]]; then
    if [ -f "dist/liaison/systemd/liaison.service" ]; then
        cp -X dist/liaison/systemd/liaison.service "$PACK_DIR/systemd/" 2>/dev/null || cp dist/liaison/systemd/liaison.service "$PACK_DIR/systemd/"
    fi
    if [ -f "dist/liaison/systemd/frontier.service" ]; then
        cp -X dist/liaison/systemd/frontier.service "$PACK_DIR/systemd/" 2>/dev/null || cp dist/liaison/systemd/frontier.service "$PACK_DIR/systemd/"
    fi
else
    if [ -f "dist/liaison/systemd/liaison.service" ]; then
        cp dist/liaison/systemd/liaison.service "$PACK_DIR/systemd/"
    fi
    if [ -f "dist/liaison/systemd/frontier.service" ]; then
        cp dist/liaison/systemd/frontier.service "$PACK_DIR/systemd/"
    fi
fi

# Copy install and uninstall scripts
echo -e "${YELLOW}Copying install scripts...${NC}"
if [[ "$(uname)" == "Darwin" ]]; then
    if [ -f "dist/liaison/install.sh" ]; then
        cp -X dist/liaison/install.sh "$PACK_DIR/" 2>/dev/null || cp dist/liaison/install.sh "$PACK_DIR/"
    fi
    if [ -f "dist/liaison/uninstall.sh" ]; then
        cp -X dist/liaison/uninstall.sh "$PACK_DIR/" 2>/dev/null || cp dist/liaison/uninstall.sh "$PACK_DIR/"
    fi
else
    if [ -f "dist/liaison/install.sh" ]; then
        cp dist/liaison/install.sh "$PACK_DIR/"
    fi
    if [ -f "dist/liaison/uninstall.sh" ]; then
        cp dist/liaison/uninstall.sh "$PACK_DIR/"
    fi
fi

# Copy documentation
if [[ "$(uname)" == "Darwin" ]]; then
    if [ -f "dist/liaison/README.md" ]; then
        cp -X dist/liaison/README.md "$PACK_DIR/" 2>/dev/null || cp dist/liaison/README.md "$PACK_DIR/" 2>/dev/null || true
    fi
    if [ -f "README.md" ]; then
        cp -X README.md "$PACK_DIR/" 2>/dev/null || cp README.md "$PACK_DIR/" 2>/dev/null || true
    fi
    if [ -f "VERSION" ]; then
        cp -X VERSION "$PACK_DIR/" 2>/dev/null || cp VERSION "$PACK_DIR/"
    fi
    if [ -f "LICENSE" ]; then
        cp -X LICENSE "$PACK_DIR/" 2>/dev/null || cp LICENSE "$PACK_DIR/"
    fi
else
    if [ -f "dist/liaison/README.md" ]; then
        cp dist/liaison/README.md "$PACK_DIR/" 2>/dev/null || true
    fi
    if [ -f "README.md" ]; then
        cp README.md "$PACK_DIR/" 2>/dev/null || true
    fi
    if [ -f "VERSION" ]; then
        cp VERSION "$PACK_DIR/"
    fi
    if [ -f "LICENSE" ]; then
        cp LICENSE "$PACK_DIR/"
    fi
fi

# Set permissions
echo -e "${YELLOW}Setting permissions...${NC}"
chmod +x "$PACK_DIR/bin"/*
# Edge packages are tar.gz files, no need to chmod +x
chmod +x "$PACK_DIR/install.sh" 2>/dev/null || true
chmod +x "$PACK_DIR/uninstall.sh" 2>/dev/null || true

# Remove macOS-specific files and extended attributes before packaging
echo -e "${YELLOW}Cleaning up macOS-specific files and extended attributes...${NC}"
find "$PACK_DIR" -name "._*" -delete 2>/dev/null || true
find "$PACK_DIR" -name ".DS_Store" -delete 2>/dev/null || true

# Remove extended attributes (xattr) from all files if on macOS
if [[ "$(uname)" == "Darwin" ]]; then
    echo -e "${YELLOW}Removing extended attributes from all files...${NC}"
    if command -v xattr >/dev/null 2>&1; then
        # Remove extended attributes from all files and directories recursively
        # Use -r flag for recursive removal and -d to clear all attributes
        find "$PACK_DIR" -type f -print0 | xargs -0 xattr -c 2>/dev/null || true
        find "$PACK_DIR" -type d -print0 | xargs -0 xattr -c 2>/dev/null || true
        echo -e "${GREEN}Extended attributes removed${NC}"
    else
        echo -e "${YELLOW}Warning: xattr command not found, cannot remove extended attributes${NC}"
    fi
fi

# Create tar.gz (exclude macOS metadata and extended attributes)
echo -e "${YELLOW}Creating tar.gz archive using ${TAR_CMD}...${NC}"
# For macOS, ensure COPYFILE_DISABLE is set and use appropriate tar options
if [[ "$(uname)" == "Darwin" ]]; then
    # macOS BSD tar respects COPYFILE_DISABLE environment variable
    # Use explicit exclusion patterns and ensure no extended attributes
    if [[ "$TAR_CMD" == "gtar" ]]; then
        # GNU tar on macOS
        COPYFILE_DISABLE=1 $TAR_CMD --no-xattrs --exclude='._*' --exclude='.DS_Store' -czf "${PACK_DIR}.tar.gz" "$PACK_DIR"
    else
        # BSD tar on macOS
        COPYFILE_DISABLE=1 $TAR_CMD --disable-copyfile --exclude='._*' --exclude='.DS_Store' -czf "${PACK_DIR}.tar.gz" "$PACK_DIR"
    fi
else
    # For GNU tar (Linux), use --no-xattrs if available
    if $TAR_CMD --help 2>&1 | grep -q "no-xattrs"; then
        $TAR_CMD --no-xattrs --exclude='._*' --exclude='.DS_Store' -czf "${PACK_DIR}.tar.gz" "$PACK_DIR"
    else
        $TAR_CMD --exclude='._*' --exclude='.DS_Store' -czf "${PACK_DIR}.tar.gz" "$PACK_DIR"
    fi
fi

# Clean up
rm -rf "$PACK_DIR"

echo -e "${GREEN}✅ Package created: ${PACK_DIR}.tar.gz${NC}"
echo -e "${GREEN}Package size: $(du -h ${PACK_DIR}.tar.gz | cut -f1)${NC}"
