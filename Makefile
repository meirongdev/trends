# trends — build & test
# 前端构建产物嵌入 Go 二进制由 API Server 一并托管(单部署单元)。
# 依赖装好后,Go 构建/测试全程离线(GOPROXY=off);仅 `npm install` 需网络(见 README)。

.PHONY: web build test test-web test-go run clean

# 构建前端并拷进 Go 的 embed 目录
web:
	cd web && npm run build
	rm -rf internal/api/dist
	mkdir -p internal/api/dist
	cp -r web/dist/. internal/api/dist/

# 完整构建:先前端,再把它嵌进 Go 二进制
build: web
	GOPROXY=off go build -o bin/trends ./cmd/trends

test: test-web test-go

test-web:
	cd web && npm run test

# 注意:用 ./cmd/... ./internal/... 而非 ./...,避免 go 扫描 web/node_modules 里的 .go 文件。
test-go:
	GOPROXY=off go test ./cmd/... ./internal/...

run: build
	./bin/trends

clean:
	rm -rf bin web/dist internal/api/dist/assets
