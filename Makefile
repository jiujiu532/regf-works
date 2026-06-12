.PHONY: build run dev clean tidy docker

# Go 构建
build:
	go build -o bin/reg-server cmd/server/main.go

run: build
	./bin/reg-server --config configs/config.yaml

dev:
	go run cmd/server/main.go --config configs/config.yaml

clean:
	rm -rf bin/

tidy:
	go mod tidy

# Docker 构建
docker:
	docker build -t regf-works:latest .
