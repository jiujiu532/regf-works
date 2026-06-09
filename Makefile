.PHONY: build run dev clean tidy fireworks-service solver-service docker-lite docker-full

# Go 构建
build:
	go build -o bin/reg-server cmd/server/main.go
	go build -o bin/reg-cli cmd/cli/main.go

run: build
	./bin/reg-server --config configs/config.yaml

dev:
	go run cmd/server/main.go --config configs/config.yaml

clean:
	rm -rf bin/

tidy:
	go mod tidy

# Python 服务
fireworks-service:
	cd scripts && pip install -r requirements.txt && python fireworks_reg.py --port 5000

solver-service:
	cd scripts && pip install -r requirements-full.txt && python turnstile_solver.py --port 8888

# Docker 构建
docker-lite:
	docker build -f Dockerfile.lite -t regf-works:lite .

docker-full:
	docker build -f Dockerfile.full -t regf-works:full .
