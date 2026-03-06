## Makefile — Navi 构建脚本
BINARY  = navi
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
DIST    = ./dist
GO      = go

.PHONY: dev build release clean

## 开发模式（从磁盘加载前端，支持实时修改）
dev:
	FRONTEND_DIR=./frontend $(GO) run .

## 构建单文件（当前平台）
build:
	@mkdir -p $(DIST)
	$(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(DIST)/$(BINARY) .
	@echo "✅  $(DIST)/$(BINARY)"

## 多平台交叉编译（发布到 GitHub Release）
release:
	@mkdir -p $(DIST)
	GOOS=linux   GOARCH=amd64  $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(DIST)/$(BINARY)-linux-amd64
	GOOS=linux   GOARCH=arm64  $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(DIST)/$(BINARY)-linux-arm64
	GOOS=windows GOARCH=amd64  $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(DIST)/$(BINARY)-windows-amd64.exe
	GOOS=darwin  GOARCH=amd64  $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(DIST)/$(BINARY)-darwin-amd64
	GOOS=darwin  GOARCH=arm64  $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(DIST)/$(BINARY)-darwin-arm64
	@echo "✅  All binaries in $(DIST)/"

clean:
	rm -rf $(DIST)
