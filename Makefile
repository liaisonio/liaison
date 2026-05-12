# ============================================================================
# Liaison Makefile
# ============================================================================
#
# 这个 Makefile 提供了 Liaison 项目的构建、测试、打包等功能。
#
# 主要功能：
#   1. 本地开发构建（macOS）
#      - make build-local          # 构建 liaison 和 liaison-edge（本地）
#      - make build-liaison        # 仅构建 liaison
#      - make build-edge           # 仅构建 liaison-edge
#
#   2. Linux 构建（使用 Docker，支持 CGO 交叉编译）
#      - make build-linux          # 构建 Linux 版本的 liaison 和 liaison-edge
#      - make build-liaison-linux  # 仅构建 Linux 版本的 liaison
#      - make build-edge-linux     # 仅构建 Linux 版本的 liaison-edge
#
#   3. Edge 多架构构建
#      - make build-edge-all       # 构建所有架构的 edge 二进制文件
#      - make build-edge-linux-amd64    # Linux amd64
#      - make build-edge-linux-arm64    # Linux arm64
#      - make build-edge-darwin-amd64   # macOS amd64
#      - make build-edge-darwin-arm64   # macOS arm64 (Apple Silicon)
#      - make build-edge-windows-amd64   # Windows amd64
#
#   4. Edge 打包（生成 tar.gz 安装包）
#      - make package-edge-all     # 打包所有架构的 edge
#      - make package-edge-linux-amd64  # 打包 Linux amd64
#      - make package-edge-linux-arm64 # 打包 Linux arm64
#      - make package-edge-darwin-amd64 # 打包 macOS amd64
#      - make package-edge-darwin-arm64 # 打包 macOS arm64
#      - make package-edge-windows-amd64 # 打包 Windows amd64
#
#   5. 代码生成
#      - make gen                  # 生成 API 和 Swagger 文档
#      - make gen-api              # 仅生成 API 代码
#      - make gen-swagger          # 仅生成 Swagger 文档
#
#   6. 测试
#      - make test                 # 运行单元测试
#      - make test-e2e             # 运行 E2E 测试
#      - make test-all             # 运行所有测试
#
#   7. 工具构建
#      - make build-tools           # 构建所有工具（本地）
#      - make build-tools-linux     # 构建所有工具（Linux）
#
#   8. 前端构建
#      - make build-web            # 构建前端到 web/dist
#
#   9. 完整打包
#      - make package               # 打包完整的 Liaison 安装包（Linux）
#                                    包含：
#                                    - liaison (Linux amd64)
#                                    - liaison-edge (所有平台：linux-amd64, linux-arm64,
#                                                    darwin-amd64, darwin-arm64, windows-amd64)
#                                    - 前端文件 (web/dist)
#                                    - systemd 配置文件
#
#  10. Docker Compose 部署（仅服务端：liaison + frontier）
#      前置：先跑 `make package`（产出 bin/web/edge/frontier 等构件），Docker 镜像只做 COPY。
#      - make image-liaison          # 构建 liaison runtime 镜像
#      - make image-frontier         # 构建 frontier runtime 镜像
#      - make images                 # 同时构建两个镜像
#      - make compose-up             # 启动（需要 deploy/docker/.env）
#      - make compose-down           # 停止并移除容器
#      - make compose-logs           # 跟踪两个服务的日志
#      - make package-docker         # 输出离线分发包
#                                    liaison-<VERSION>-docker-amd64.tar.gz 含：
#                                    - images/{liaison,frontier}.tar（docker save）
#                                    - docker-compose.yaml（release 版，无 build: 段）
#                                    - .env.example（预钉镜像 tag）
#                                    - load.sh / README.md
#
# 注意事项：
#   - liaison 需要 CGO（SQLite），本地构建需要 CGO_ENABLED=1
#   - liaison-edge 使用纯 Go 实现，不需要 CGO，可以 CGO_ENABLED=0
#   - Linux 构建使用 Docker，自动处理依赖安装
#   - Edge 二进制文件不依赖 libpcap，可以在任何系统上运行
#
# ============================================================================

include ./Makefile.defs

# When running `make version=1.2.7` without explicit goals,
# use version update as the default action.
ifeq ($(strip $(MAKECMDGOALS)),)
ifneq ($(strip $(version)),)
.DEFAULT_GOAL := update-version
endif
endif

# ============================================================================
# Docker build variables and functions
# ============================================================================
DOCKER_IMAGE = golang:1.24.0
DOCKER_VOLUME = -v "$(shell pwd):/build"
DOCKER_WORKDIR = -w /build
DOCKER_BASE = docker run --rm $(DOCKER_VOLUME) $(DOCKER_WORKDIR)
DOCKER_HOST_UID := $(shell id -u 2>/dev/null || echo 0)
DOCKER_HOST_GID := $(shell id -g 2>/dev/null || echo 0)
GO_BUILD_FLAGS = -trimpath -ldflags '-s -w'

# Ensure Docker image exists (pull only if not present locally)
.PHONY: docker-image
docker-image:
	@if docker image inspect $(DOCKER_IMAGE) >/dev/null 2>&1; then \
		echo "✅ Docker image $(DOCKER_IMAGE) already exists locally"; \
	else \
		echo "📥 Pulling Docker image $(DOCKER_IMAGE)..."; \
		docker pull $(DOCKER_IMAGE) || exit 1; \
		echo "✅ Docker image $(DOCKER_IMAGE) pulled successfully"; \
	fi

# Function to build with CGO (for liaison and tools)
# Usage: $(call docker-build-cgo,platform,goos,goarch,output,source)
define docker-build-cgo
	@echo "Building $(4) for $(2)-$(3) using Docker..."
	@mkdir -p ./bin
	@$(DOCKER_BASE) \
		--platform $(1) \
		-e CGO_ENABLED=1 \
		-e GOOS=$(2) \
		-e GOARCH=$(3) \
		-e GOTOOLCHAIN=auto \
		$(DOCKER_IMAGE) sh -c "\
			apt-get update -qq && \
			DEBIAN_FRONTEND=noninteractive apt-get install -y -qq gcc libc6-dev libsqlite3-dev >/dev/null 2>&1 && \
			go env -w GOTOOLCHAIN=auto && \
			go mod download && \
			CC=gcc CGO_ENABLED=1 go build $(GO_BUILD_FLAGS) -o ./bin/$(4) $(5) && \
			chmod +x ./bin/$(4) && \
			chown $(DOCKER_HOST_UID):$(DOCKER_HOST_GID) ./bin/$(4)"
	@chmod +x ./bin/$(4)
	@echo "✅ Built: ./bin/$(4)"
endef

# Function to build without CGO (for edge)
# Usage: $(call docker-build-no-cgo,platform,goos,goarch,output,source)
define docker-build-no-cgo
	@echo "Building $(4) for $(2)-$(3) using Docker..."
	@mkdir -p ./bin
	@$(DOCKER_BASE) \
		--platform $(1) \
		-e GOOS=$(2) \
		-e GOARCH=$(3) \
		-e GOTOOLCHAIN=auto \
		$(DOCKER_IMAGE) sh -c "\
			go env -w GOTOOLCHAIN=auto && \
			go mod download && \
			CGO_ENABLED=0 go build $(GO_BUILD_FLAGS) -o ./bin/$(4) $(5) && \
			chmod +x ./bin/$(4) && \
			chown $(DOCKER_HOST_UID):$(DOCKER_HOST_GID) ./bin/$(4)"
	@chmod +x ./bin/$(4)
	@echo "✅ Built: ./bin/$(4)"
endef

# Function for local darwin builds
# Usage: $(call local-build-darwin,goarch,output,source)
define local-build-darwin
	@echo "Building $(2) for darwin-$(1)..."
	@mkdir -p ./bin
	@if [ "$$(uname)" != "Darwin" ]; then \
		echo "❌ Error: darwin CGO builds must run on macOS"; \
		exit 1; \
	fi
	@ARCH_FLAG="$$(if [ "$(1)" = "amd64" ]; then echo "x86_64"; else echo "arm64"; fi)" && \
		CGO_ENABLED=1 GOOS=darwin GOARCH=$(1) \
		CC="clang -arch $$ARCH_FLAG" \
		CXX="clang++ -arch $$ARCH_FLAG" \
		go build $(GO_BUILD_FLAGS) -o ./bin/$(2) $(3)
	@chmod +x ./bin/$(2)
	@echo "✅ Built: ./bin/$(2)"
endef

# ============================================================================
# Default targets
# ============================================================================
.PHONY: all
all: build-local test-deps

.PHONY: linux
linux: build-linux

# ============================================================================
# Docker images for code generation
# ============================================================================
.PHONY: image-gen-swagger
image-gen-swagger:
	docker buildx build -t liaison-gen-swagger:${VERSION} -f images/Dockerfile.liaison-swagger .

.PHONY: image-gen-api
image-gen-api:
	docker buildx build -t image-gen-api:${VERSION} -f images/Dockerfile.liaison-api .

# ============================================================================
# Local development builds (for macOS)
# ============================================================================
.PHONY: build-local
build-local: build-liaison build-edge

.PHONY: build-liaison
build-liaison:
	CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o ./bin/liaison cmd/manager/main.go

.PHONY: build-edge
build-edge:
	go build -trimpath -ldflags "-s -w" -o ./bin/liaison-edge cmd/edge/main.go

# Legacy aliases
.PHONY: liaison liaison-edge
liaison: build-liaison
liaison-edge: build-edge

# ============================================================================
# Linux builds using Docker (required for CGO cross-compilation from macOS)
# ============================================================================
.PHONY: build-linux
build-linux: build-liaison-linux build-edge-linux build-frontier-linux

.PHONY: build-liaison-linux
build-liaison-linux: docker-image
	$(call docker-build-cgo,linux/amd64,linux,amd64,liaison,cmd/manager/main.go)

.PHONY: build-edge-linux
build-edge-linux: docker-image
	$(call docker-build-no-cgo,linux/amd64,linux,amd64,liaison-edge,cmd/edge/main.go)

.PHONY: build-frontier-linux
build-frontier-linux:
	@mkdir -p ./bin
	@if [ -f "./bin/frontier" ]; then \
		echo "✅ frontier binary already exists, skipping download"; \
	else \
		echo "Downloading frontier-linux-amd64 from GitHub releases..."; \
		curl -L -o ./bin/frontier https://github.com/singchia/frontier/releases/download/v1.2.4/frontier-linux-amd64 && \
		chmod +x ./bin/frontier && \
		echo "✅ Downloaded: ./bin/frontier"; \
	fi

# Legacy aliases
.PHONY: liaison-linux liaison-edge-linux
liaison-linux: build-liaison-linux
liaison-edge-linux: build-edge-linux

# ============================================================================
# Code generation
# ============================================================================
.PHONY: gen-api
gen-api:
	docker run --rm -v ${PWD}/api/v1:/api/v1 image-gen-api:${VERSION}

.PHONY: gen-swagger
gen-swagger:
	docker run --rm -v ${PWD}:/liaison liaison-gen-swagger:${VERSION}

.PHONY: gen
gen: gen-api gen-swagger

# ============================================================================
# Testing
# ============================================================================
.PHONY: test
test:
	go test -v ./...

.PHONY: test-deps
test-deps:
	go get github.com/stretchr/testify/assert
	go get github.com/stretchr/testify/require

.PHONY: test-e2e
test-e2e:
	@echo "Running E2E tests..."
	@./test/e2e/run_simple_test.sh

.PHONY: test-e2e-full
test-e2e-full:
	@echo "Running full E2E tests..."
	@./test/e2e/run_e2e_test.sh

.PHONY: test-all
test-all: test test-e2e

# ============================================================================
# Tools
# ============================================================================
.PHONY: build-tools
build-tools: build-password-verifier build-password-generator

.PHONY: build-password-verifier
build-password-verifier:
	CGO_ENABLED=1 go build $(GO_BUILD_FLAGS) -o ./bin/password-verifier ./tools/password-verifier

.PHONY: build-password-generator
build-password-generator:
	CGO_ENABLED=1 go build $(GO_BUILD_FLAGS) -o ./bin/password-generator ./tools/password-generator

.PHONY: build-tools-linux
build-tools-linux: build-password-verifier-linux build-password-generator-linux

.PHONY: build-password-verifier-linux
build-password-verifier-linux: docker-image
	$(call docker-build-cgo,linux/amd64,linux,amd64,password-verifier,./tools/password-verifier)

.PHONY: build-password-generator-linux
build-password-generator-linux: docker-image
	$(call docker-build-cgo,linux/amd64,linux,amd64,password-generator,./tools/password-generator)

# Legacy aliases
.PHONY: tools tools-linux password-verifier password-generator password-verifier-linux password-generator-linux
tools: build-tools
tools-linux: build-tools-linux
password-verifier: build-password-verifier
password-generator: build-password-generator
password-verifier-linux: build-password-verifier-linux
password-generator-linux: build-password-generator-linux

# ============================================================================
# Edge binary builds for different architectures
# ============================================================================
.PHONY: build-edge-all
build-edge-all: build-edge-linux-amd64 build-edge-linux-arm64 build-edge-darwin-amd64 build-edge-darwin-arm64 build-edge-windows-amd64
	@echo "✅ All edge binaries built in ./bin/"

.PHONY: build-edge-linux-amd64
build-edge-linux-amd64: docker-image
	$(call docker-build-no-cgo,linux/amd64,linux,amd64,liaison-edge-linux-amd64,cmd/edge/main.go)

.PHONY: build-edge-linux-arm64
build-edge-linux-arm64: docker-image
	$(call docker-build-no-cgo,linux/arm64,linux,arm64,liaison-edge-linux-arm64,cmd/edge/main.go)

.PHONY: build-edge-darwin-amd64
build-edge-darwin-amd64:
	$(call local-build-darwin,amd64,liaison-edge-darwin-amd64,cmd/edge/main.go)

.PHONY: build-edge-darwin-arm64
build-edge-darwin-arm64:
	$(call local-build-darwin,arm64,liaison-edge-darwin-arm64,cmd/edge/main.go)

.PHONY: build-edge-windows-amd64
build-edge-windows-amd64: docker-image
	$(call docker-build-no-cgo,linux/amd64,windows,amd64,liaison-edge-windows-amd64.exe,cmd/edge/main.go)

# ============================================================================
# Tar command detection: prefer gtar (GNU tar) if available
# ============================================================================
TAR_CMD := $(shell command -v gtar 2>/dev/null || command -v tar 2>/dev/null || echo tar)

# ============================================================================
# Edge package creation (tar.gz with binary and config template)
# ============================================================================
.PHONY: package-edge-all
package-edge-all: package-edge-linux-amd64 package-edge-linux-arm64 package-edge-darwin-amd64 package-edge-darwin-arm64 package-edge-windows-amd64
	@echo "✅ All edge packages created in ./packages/edge/"

.PHONY: package-edge-linux-amd64
package-edge-linux-amd64: build-edge-linux-amd64
	@echo "Packaging liaison-edge-linux-amd64..."
	@mkdir -p ./packages/edge
	@TMP_DIR=$$(mktemp -d) && \
		PACKAGE_PATH="$$(pwd)/packages/edge/liaison-edge-linux-amd64.tar.gz" && \
		COPYFILE_DISABLE=1 cp ./bin/liaison-edge-linux-amd64 "$$TMP_DIR/liaison-edge" && \
		COPYFILE_DISABLE=1 cp ./dist/edge/liaison-edge.yaml.template "$$TMP_DIR/liaison-edge.yaml.template" && \
		cd "$$TMP_DIR" && \
		COPYFILE_DISABLE=1 $(TAR_CMD) --exclude='._*' --exclude='.DS_Store' -czf "$$PACKAGE_PATH" liaison-edge liaison-edge.yaml.template && \
		rm -rf "$$TMP_DIR" && \
		echo "✅ Package created: ./packages/edge/liaison-edge-linux-amd64.tar.gz"

.PHONY: package-edge-linux-arm64
package-edge-linux-arm64: build-edge-linux-arm64
	@echo "Packaging liaison-edge-linux-arm64..."
	@mkdir -p ./packages/edge
	@TMP_DIR=$$(mktemp -d) && \
		PACKAGE_PATH="$$(pwd)/packages/edge/liaison-edge-linux-arm64.tar.gz" && \
		COPYFILE_DISABLE=1 cp ./bin/liaison-edge-linux-arm64 "$$TMP_DIR/liaison-edge" && \
		COPYFILE_DISABLE=1 cp ./dist/edge/liaison-edge.yaml.template "$$TMP_DIR/liaison-edge.yaml.template" && \
		cd "$$TMP_DIR" && \
		COPYFILE_DISABLE=1 $(TAR_CMD) --exclude='._*' --exclude='.DS_Store' -czf "$$PACKAGE_PATH" liaison-edge liaison-edge.yaml.template && \
		rm -rf "$$TMP_DIR" && \
		echo "✅ Package created: ./packages/edge/liaison-edge-linux-arm64.tar.gz"

.PHONY: package-edge-darwin-amd64
package-edge-darwin-amd64: build-edge-darwin-amd64
	@echo "Packaging liaison-edge-darwin-amd64..."
	@mkdir -p ./packages/edge
	@TMP_DIR=$$(mktemp -d) && \
		PACKAGE_PATH="$$(pwd)/packages/edge/liaison-edge-darwin-amd64.tar.gz" && \
		COPYFILE_DISABLE=1 cp -X ./bin/liaison-edge-darwin-amd64 "$$TMP_DIR/liaison-edge" && \
		COPYFILE_DISABLE=1 cp -X ./dist/edge/liaison-edge.yaml.template "$$TMP_DIR/liaison-edge.yaml.template" && \
		cd "$$TMP_DIR" && \
		COPYFILE_DISABLE=1 $(TAR_CMD) --exclude='._*' --exclude='.DS_Store' -czf "$$PACKAGE_PATH" liaison-edge liaison-edge.yaml.template && \
		rm -rf "$$TMP_DIR" && \
		echo "✅ Package created: ./packages/edge/liaison-edge-darwin-amd64.tar.gz"

.PHONY: package-edge-darwin-arm64
package-edge-darwin-arm64: build-edge-darwin-arm64
	@echo "Packaging liaison-edge-darwin-arm64..."
	@mkdir -p ./packages/edge
	@TMP_DIR=$$(mktemp -d) && \
		PACKAGE_PATH="$$(pwd)/packages/edge/liaison-edge-darwin-arm64.tar.gz" && \
		COPYFILE_DISABLE=1 cp -X ./bin/liaison-edge-darwin-arm64 "$$TMP_DIR/liaison-edge" && \
		COPYFILE_DISABLE=1 cp -X ./dist/edge/liaison-edge.yaml.template "$$TMP_DIR/liaison-edge.yaml.template" && \
		cd "$$TMP_DIR" && \
		COPYFILE_DISABLE=1 $(TAR_CMD) --exclude='._*' --exclude='.DS_Store' -czf "$$PACKAGE_PATH" liaison-edge liaison-edge.yaml.template && \
		rm -rf "$$TMP_DIR" && \
		echo "✅ Package created: ./packages/edge/liaison-edge-darwin-arm64.tar.gz"

.PHONY: package-edge-windows-amd64
package-edge-windows-amd64: build-edge-windows-amd64
	@echo "Packaging liaison-edge-windows-amd64..."
	@mkdir -p ./packages/edge
	@TMP_DIR=$$(mktemp -d) && \
		PACKAGE_PATH="$$(pwd)/packages/edge/liaison-edge-windows-amd64.tar.gz" && \
		COPYFILE_DISABLE=1 cp ./bin/liaison-edge-windows-amd64.exe "$$TMP_DIR/liaison-edge.exe" && \
		COPYFILE_DISABLE=1 cp ./dist/edge/liaison-edge.yaml.template "$$TMP_DIR/liaison-edge.yaml.template" && \
		cd "$$TMP_DIR" && \
		COPYFILE_DISABLE=1 $(TAR_CMD) --exclude='._*' --exclude='.DS_Store' -czf "$$PACKAGE_PATH" liaison-edge.exe liaison-edge.yaml.template && \
		rm -rf "$$TMP_DIR" && \
		echo "✅ Package created: ./packages/edge/liaison-edge-windows-amd64.tar.gz"

# Legacy aliases
.PHONY: edge-packages edge-package-linux-amd64 edge-package-linux-arm64 edge-package-darwin-amd64 edge-package-darwin-arm64 edge-package-windows-amd64
edge-packages: package-edge-all
edge-package-linux-amd64: package-edge-linux-amd64
edge-package-linux-arm64: package-edge-linux-arm64
edge-package-darwin-amd64: package-edge-darwin-amd64
edge-package-darwin-arm64: package-edge-darwin-arm64
edge-package-windows-amd64: package-edge-windows-amd64

# ============================================================================
# Frontend build
# ============================================================================
.PHONY: build-web
build-web:
	@echo "Building frontend..."
	@if [ ! -d "web" ]; then \
		echo "❌ Error: web directory not found"; \
		exit 1; \
	fi
	@if [ ! -f "web/package.json" ]; then \
		echo "❌ Error: web/package.json not found"; \
		exit 1; \
	fi
	@echo "Cleaning old build artifacts..."
	@rm -rf web/dist
	@( \
		cd web && \
		export PNPM_HOME="$$HOME/.local/share/pnpm" 2>/dev/null || true && \
		export PATH="$$PNPM_HOME:$$PATH" 2>/dev/null || true && \
		echo "Installing dependencies..." && \
		pnpm install || (echo "❌ Error: pnpm install failed" && exit 1) && \
		echo "Building frontend..." && \
		pnpm run build || (echo "❌ Error: pnpm run build failed" && exit 1) && \
		echo "Build completed, checking output..." && \
		if [ -d "dist" ] && [ "$$(ls -A dist 2>/dev/null)" ]; then \
			echo "✅ Build output found in dist/"; \
			ls -1 dist | head -5; \
		else \
			echo "❌ Error: dist directory not found or empty in web/"; \
			ls -la . | head -10; \
			exit 1; \
		fi \
	) || (echo "❌ Error: Frontend build failed" && exit 1)
	@if [ -d "web/dist" ] && [ "$$(ls -A web/dist 2>/dev/null)" ]; then \
		echo "✅ Frontend built successfully in web/dist/"; \
		echo "   Build output: $$(ls -1 web/dist | head -5 | tr '\n' ' ')"; \
	else \
		echo "❌ Error: web/dist not found or empty after build"; \
		echo "   Checking web directory contents..."; \
		ls -la web/ | head -10 || true; \
		exit 1; \
	fi

# ============================================================================
# Full package (liaison + edge binaries for all platforms + frontend + systemd files)
# ============================================================================
.PHONY: package
package: build-linux build-edge-all package-edge-all build-web build-tools-linux
	@chmod +x dist/package.sh
	@./dist/package.sh

# Legacy alias
.PHONY: pack
pack: package

# ============================================================================
# Docker images (server-side: liaison + frontier)
# Images are "thin": they COPY pre-built artifacts produced by `make package`.
# Run `make package` first (or at least build-linux + build-web + build-tools-linux
# + build-edge-all + package-edge-all + build-frontier-linux).
# ============================================================================
DOCKER_COMPOSE_DIR = deploy/docker
LIAISON_IMAGE_REGISTRY ?= liaison
LIAISON_IMAGE_TAG ?= $(shell cat VERSION 2>/dev/null | tr -d 'v' || echo latest)

.PHONY: image-liaison
image-liaison:
	@if [ ! -f bin/liaison ] || [ ! -f bin/password-generator ] || [ ! -d web/dist ]; then \
		echo "❌ Missing artifacts. Run 'make package' first."; \
		exit 1; \
	fi
	docker build -t $(LIAISON_IMAGE_REGISTRY)/liaison:$(LIAISON_IMAGE_TAG) \
		-f $(DOCKER_COMPOSE_DIR)/Dockerfile.liaison .
	@echo "✅ Built $(LIAISON_IMAGE_REGISTRY)/liaison:$(LIAISON_IMAGE_TAG)"

.PHONY: image-frontier
image-frontier: build-frontier-linux
	docker build -t $(LIAISON_IMAGE_REGISTRY)/frontier:$(LIAISON_IMAGE_TAG) \
		-f $(DOCKER_COMPOSE_DIR)/Dockerfile.frontier .
	@echo "✅ Built $(LIAISON_IMAGE_REGISTRY)/frontier:$(LIAISON_IMAGE_TAG)"

.PHONY: images
images: image-liaison image-frontier

.PHONY: compose-up
compose-up:
	@if [ ! -f $(DOCKER_COMPOSE_DIR)/.env ]; then \
		echo "❌ $(DOCKER_COMPOSE_DIR)/.env not found. Copy .env.example and edit first."; \
		exit 1; \
	fi
	cd $(DOCKER_COMPOSE_DIR) && docker compose up -d

.PHONY: compose-down
compose-down:
	cd $(DOCKER_COMPOSE_DIR) && docker compose down

.PHONY: compose-logs
compose-logs:
	cd $(DOCKER_COMPOSE_DIR) && docker compose logs -f

.PHONY: package-docker
package-docker: images
	@chmod +x $(DOCKER_COMPOSE_DIR)/package-docker.sh
	@$(DOCKER_COMPOSE_DIR)/package-docker.sh

# ============================================================================
# Version sync
# Usage:
#   make update-version version=1.2.7
#   make version version=1.2.7
#   make version=1.2.7
# ============================================================================
.PHONY: update-version version
update-version version:
	@if [ -z "$(version)" ]; then \
		echo "❌ Error: missing version. Usage: make version=1.2.7"; \
		exit 1; \
	fi
	@chmod +x scripts/sync-release-version.sh
	@scripts/sync-release-version.sh "$(version)"
