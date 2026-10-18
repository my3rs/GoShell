APP_NAME    ?= GoShell
VERSION     ?= dev
VERSION_PKG := goshell/internal/version
VERSION_LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION)
MAIN_SRC    := ./cmd/goshell
OUT_DIR     := bin
ICON_ICO    := assets/brand/app-icon.ico
NATIVES_DIR := assets/natives
WINRES      ?= go run github.com/tc-hib/go-winres@v0.3.3

.PHONY: all help build run test clean build-windows build-windows-arm64 package-windows build-darwin build-linux ensure-winres fetch-natives

# 默认目标
all: help

## help: 显示所有可用目标与说明
help:
	@echo "GoShell 构建与打包工具:"
	@echo ""
	@echo "  make build               - 编译当前平台的二进制文件 ($(OUT_DIR)/$(APP_NAME))"
	@echo "  make run                 - 本地直接启动运行开发版"
	@echo "  make test                - 运行所有单元测试"
	@echo "  make build-windows       - 编译 Windows 64位程序并注入专属图标与清单 ($(OUT_DIR)/$(APP_NAME).exe)"
	@echo "  make build-windows-arm64 - 编译 Windows ARM64 架构程序 ($(OUT_DIR)/$(APP_NAME)-arm64.exe)"
	@echo "  make package-windows     - 编译 Windows 并打包为离线 zip 压缩包 (含 Ghostty 终端引擎)"
	@echo "  make build-darwin        - 编译 macOS 原生版本"
	@echo "  make build-linux         - 编译 Linux 64位原生版本"
	@echo "  make fetch-natives       - 重新下载各平台 Ghostty 终端引擎动态库"
	@echo "  make clean               - 清理编译输出和临时资源文件"
	@echo ""

## fetch-natives: 下载并验证全部平台的 Ghostty 原生引擎动态库
fetch-natives:
	go run tools/fetch_natives/main.go

## build: 构建当前主机系统二进制
build:
	@mkdir -p $(OUT_DIR)
	@echo "==> 编译本地版本..."
	go build -trimpath -ldflags="$(VERSION_LDFLAGS) -s -w" -o $(OUT_DIR)/goshell $(MAIN_SRC)
	@echo "==> 生成成功: $(OUT_DIR)/goshell"

## run: 本地启动运行
run:
	go run $(MAIN_SRC)

## test: 运行单元测试
test:
	go test -v ./...

## ensure-winres: 确保 go-winres 可用
ensure-winres:
	@echo "==> 使用 go-winres v0.3.3..."

## build-windows: 跨平台编译 Windows 64位程序（含图标与 GUI 清单，附带 Ghostty 离线引擎）
build-windows: ensure-winres
	@mkdir -p $(OUT_DIR)
	@echo "==> [1/3] 生成 Windows 图标与 GUI 清单资源..."
	@$(WINRES) simply \
		--icon $(ICON_ICO) \
		--manifest gui \
		--product-name "$(APP_NAME)" \
		--file-description "$(APP_NAME) Desktop Terminal" \
		--product-version "$(VERSION)" \
		--arch amd64 \
		--out cmd/goshell/rsrc
	@echo "==> [2/3] 编译 Windows 64位程序 (去除黑框控制台)..."
	@trap 'rm -f cmd/goshell/*.syso' EXIT; \
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
		-trimpath \
		-ldflags="-H windowsgui $(VERSION_LDFLAGS) -s -w" \
		-o $(OUT_DIR)/$(APP_NAME).exe \
		$(MAIN_SRC)
	@echo "==> [3/3] 部署离线终端引擎 ghostty-vt.dll 与 busybox.exe 到输出目录..."
	@cp $(NATIVES_DIR)/windows-amd64/ghostty-vt.dll $(OUT_DIR)/ghostty-vt.dll
	@if [ -f $(NATIVES_DIR)/windows-amd64/busybox.exe ]; then \
		cp $(NATIVES_DIR)/windows-amd64/busybox.exe $(OUT_DIR)/busybox.exe; \
	fi
	@echo "==> 打包完成: $(OUT_DIR)/$(APP_NAME).exe (完全离线免依赖)"

## build-windows-arm64: 跨平台编译 Windows ARM64 程序
build-windows-arm64: ensure-winres
	@mkdir -p $(OUT_DIR)
	@echo "==> [1/3] 生成 Windows ARM64 资源..."
	@$(WINRES) simply \
		--icon $(ICON_ICO) \
		--manifest gui \
		--product-name "$(APP_NAME)" \
		--file-description "$(APP_NAME) Desktop Terminal" \
		--product-version "$(VERSION)" \
		--arch arm64 \
		--out cmd/goshell/rsrc
	@echo "==> [2/3] 编译 Windows ARM64 程序..."
	@trap 'rm -f cmd/goshell/*.syso' EXIT; \
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build \
		-trimpath \
		-ldflags="-H windowsgui $(VERSION_LDFLAGS) -s -w" \
		-o $(OUT_DIR)/$(APP_NAME)-arm64.exe \
		$(MAIN_SRC)
	@cp $(NATIVES_DIR)/windows-arm64/ghostty-vt.dll $(OUT_DIR)/ghostty-vt.dll
	@if [ -f $(NATIVES_DIR)/windows-arm64/busybox.exe ]; then \
		cp $(NATIVES_DIR)/windows-arm64/busybox.exe $(OUT_DIR)/busybox.exe; \
	fi
	@echo "==> 打包完成: $(OUT_DIR)/$(APP_NAME)-arm64.exe"

## package-windows: 编译 Windows 并打包为完全离线的 zip 压缩包与单文件便携版
package-windows: build-windows
	@echo "==> 打包离线 zip 压缩包 (包含 GoShell.exe, ghostty-vt.dll 与 busybox.exe)..."
	@cd $(OUT_DIR) && rm -f $(APP_NAME)-v$(VERSION)-windows-amd64.zip && \
	zip -9 $(APP_NAME)-v$(VERSION)-windows-amd64.zip $(APP_NAME).exe ghostty-vt.dll busybox.exe
	@echo "==> 构建完成！"
	@echo "    [单文件便携版] $(OUT_DIR)/$(APP_NAME).exe (内嵌VT引擎与BusyBox，独立单文件即开即用)"
	@echo "    [离线压缩包]   $(OUT_DIR)/$(APP_NAME)-v$(VERSION)-windows-amd64.zip"

## build-darwin: 编译 macOS 原生版本
build-darwin:
	@mkdir -p $(OUT_DIR)
	@echo "==> 编译 macOS 版本..."
	go build -trimpath -ldflags="$(VERSION_LDFLAGS) -s -w" -o $(OUT_DIR)/$(APP_NAME)-darwin $(MAIN_SRC)
	@ARCH=$$(uname -m | sed 's/x86_64/amd64/'); \
	if [ -f $(NATIVES_DIR)/darwin-$$ARCH/libghostty-vt.dylib ]; then \
		cp $(NATIVES_DIR)/darwin-$$ARCH/libghostty-vt.dylib $(OUT_DIR)/; \
	fi
	@echo "==> 生成成功: $(OUT_DIR)/$(APP_NAME)-darwin"

## build-linux: 编译 Linux 64位版本
build-linux:
	@mkdir -p $(OUT_DIR)
	@echo "==> 编译 Linux amd64 版本..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(VERSION_LDFLAGS) -s -w" -o $(OUT_DIR)/$(APP_NAME)-linux $(MAIN_SRC)
	@cp $(NATIVES_DIR)/linux-amd64/libghostty-vt.so $(OUT_DIR)/
	@echo "==> 生成成功: $(OUT_DIR)/$(APP_NAME)-linux"

## clean: 清理编译生成物与临时文件
clean:
	@echo "==> 清理生成文件..."
	rm -rf $(OUT_DIR)
	rm -f cmd/goshell/*.syso
	@echo "==> 清理完成"
