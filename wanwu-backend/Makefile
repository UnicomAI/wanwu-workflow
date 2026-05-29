include .env

LDFLAGS := -X main.buildTime=$(shell date +%Y-%m-%d,%H:%M:%S) \
			-X main.buildVersion=${WANWU_WORKFLOW_VERSION} \
			-X main.gitCommitID=$(shell git --git-dir=../.git rev-parse HEAD) \
			-X main.gitBranch=$(shell git --git-dir=../.git for-each-ref --format='%(refname:short)->%(upstream:short)' $(shell git --git-dir=../.git symbolic-ref -q HEAD)) \
			-X main.builder=$(shell git config user.name)

build-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod vendor -ldflags "$(LDFLAGS)" -o ./bin/amd64/ ./cmd/workflow-wanwu

build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -mod vendor -ldflags "$(LDFLAGS)" -o ./bin/arm64/ ./cmd/workflow-wanwu

check:
	go vet ./...
	go fmt ./...
	goimports -w -local github.com/UnicomAI/wanwu-workflow .
	docker run --rm -t -v $(PWD):/app -w /app golangci/golangci-lint:v2.10.1 bash -c 'golangci-lint run -v --timeout 10m'

init:
	cp ../docker/volumes/mysql/schema.sql configs
	cp ../docker/atlas/migrations/20250[8-9][1-9]*.sql configs/migrations
	cp ../docker/atlas/migrations/20251[0-2]*.sql configs/migrations
	go mod tidy
	go mod vendor