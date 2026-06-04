# trends — build & test
# 前端构建产物嵌入 Go 二进制,由 API Server 一并托管(单部署单元)。
# 依赖装好后 Go 构建/测试全程离线(GOPROXY=off);仅 `npm install` 需网络(见 README)。
#
# 约定:Go 包路径用 ./cmd/... ./internal/...,不要用 ./...
#       (否则 go 会扫描 web/node_modules 里的杂散 .go 文件)。

GO        ?= go
GOPKGS    := ./cmd/... ./internal/...
GOFMT_DIRS := cmd internal
OFFLINE   := GOPROXY=off
BIN       := bin/trends

.DEFAULT_GOAL := help
.PHONY: help setup deps hooks web build run dev dev-api dev-web \
        test test-web test-go test-race typecheck \
        fmt fmt-check vet lint lint-fix tidy check clean

help: ## 列出可用目标
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-11s\033[0m %s\n", $$1, $$2}'

setup: deps hooks ## 新克隆一键初始化:装前端依赖 + 启用 git hooks

deps: ## 安装前端依赖(需联网)
	cd web && npm install

hooks: ## 启用 git hooks(core.hooksPath -> .githooks);跳过单次提交用 git commit --no-verify
	git config core.hooksPath .githooks
	@chmod +x .githooks/* 2>/dev/null || true
	@echo "✓ git hooks 已启用(.githooks/pre-commit)"

# ---- 构建 ----
web: ## 构建前端并拷进 Go 的 embed 目录
	cd web && npm run build
	rm -rf internal/api/dist
	mkdir -p internal/api/dist
	cp -r web/dist/. internal/api/dist/

build: web ## 完整构建:前端 + 嵌入式 Go 二进制 → bin/trends
	$(OFFLINE) $(GO) build -o $(BIN) ./cmd/trends

# ---- 运行 ----
run: build ## 构建并运行(生产形态:单二进制托管内嵌 SPA)
	./$(BIN)

dev: ## 开发说明:两个终端分别跑 dev-api 与 dev-web
	@echo '开发模式:终端1 -> make dev-api (:8080);终端2 -> make dev-web (Vite HMR, /api 代理到 :8080)'

dev-api: ## 开发:从源码运行 API/worker(:8080)
	$(OFFLINE) $(GO) run ./cmd/trends

dev-web: ## 开发:前端 Vite 开发服务器(HMR,/api 代理到 :8080)
	cd web && npm run dev

# ---- 测试 ----
test: test-web test-go ## 跑全部测试(前端 + Go)

test-web: ## 前端测试(Vitest)
	cd web && npm run test

test-go: ## Go 测试
	$(OFFLINE) $(GO) test $(GOPKGS)

test-race: ## Go 测试(竞态检测)
	$(OFFLINE) $(GO) test -race $(GOPKGS)

typecheck: ## 前端类型检查(tsc --noEmit)
	cd web && npx tsc --noEmit

# ---- 格式化 / 静态检查 ----
fmt: ## Go 格式化(gofmt,会改写文件)
	$(GO) fmt $(GOPKGS)

fmt-check: ## 检查 Go 格式(gofmt -l;有未格式化文件则失败,不改写)
	@out=$$(gofmt -l $(GOFMT_DIRS)); \
	if [ -n "$$out" ]; then echo "以下文件需要 gofmt:"; echo "$$out"; exit 1; fi

vet: ## go vet
	$(OFFLINE) $(GO) vet $(GOPKGS)

lint: ## 前端 lint(ESLint,只检查)
	cd web && npm run lint

lint-fix: ## 前端 lint 自动修复(eslint --fix)
	cd web && npm run lint -- --fix

tidy: ## go mod tidy(离线;新增未缓存依赖需联网)
	$(OFFLINE) $(GO) mod tidy

check: fmt-check vet test-go typecheck lint ## 总检查(不改写):fmt-check + vet + Go 测试 + 前端类型检查 + lint

clean: ## 清理构建产物
	rm -rf bin web/dist internal/api/dist/assets
