.PHONY: build run tidy test smoke smoke-admin seed-demo check-health

CONF ?= app/app/configs/config.yaml
BASE ?= http://127.0.0.1:8000

build:
	go build -o bin/app.exe ./app/app/cmd/app

run: build
	./bin/app.exe -conf $(CONF)

tidy:
	go mod tidy

test:
	go test ./app/app/internal/biz/... -count=1

# 冒烟前确认服务已启动（db 字段由 smoke_mainpath 严格校验）
check-health:
	@curl -sf "$(BASE)/health" >/dev/null || (echo "服务未就绪: $(BASE)/health — 请先 make run" && false)

smoke: check-health
	go run scripts/smoke_mainpath.go

smoke-admin: check-health
	go run scripts/smoke_admin_write.go

# 演示种子（直连 MySQL，无需 HTTP 服务）
seed-demo:
	go run scripts/seed_demo_users.go
