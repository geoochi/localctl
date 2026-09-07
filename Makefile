.PHONY: build frontend backend clean

# 一键构建：前端产物 → 嵌入 → Go 单二进制
build: frontend
	go build -o localctl ./cmd/localctl

frontend:
	cd frontend && pnpm build

backend:
	go build -o localctl ./cmd/localctl

clean:
	rm -f localctl
	rm -rf frontend/dist
