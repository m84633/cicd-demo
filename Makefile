.PHONY: all build run gotool clean help wire clear_wire consumer

BINARY="partivo_tickets"
OLD_MODULE="partivo_community"

# Docker image settings
GIT_HASH := $(shell git rev-parse --short HEAD)


all: gotool build

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ./bin/${BINARY} ./cmd/server

# The 'run' command is now an alias for 'frontend' for convenience


gotool:
	go fmt ./...
	go vet ./...

wire:
	go run github.com/google/wire/cmd/wire ./cmd/server

consumer_wire:
	go run github.com/google/wire/cmd/wire ./cmd/consumer

clean:
	@if [ -f ./bin/${BINARY} ]; then rm ./bin/${BINARY} ; fi

help:
	@echo "make - 格式化 Go 程式碼, then go build"
	@echo "make build - go build"
	@echo "make run - go run"
	@echo "make gotool - Go tool 'fmt' and 'vet'"
	@echo "make wire - 執行 wire 生成依賴注入程式碼"
	@echo "make consumer - 啟動 consumer 背景服務"
	@echo "make clean - 移除二進制檔案 和 vim swap files"
	@echo "make clear_wire - 移除 wire 生成的臨時隱藏檔案"

grpc:
	protoc -I api \
	--go_out=api --go_opt=paths=source_relative \
	--go-grpc_out=api --go-grpc_opt=paths=source_relative \
	--grpc-gateway_out=api --grpc-gateway_opt=paths=source_relative \
	--openapiv2_out=. --openapiv2_opt=logtostderr=true,output_format=yaml,json_names_for_fields=true,allow_merge=true \
	api/*/*.proto api/*.proto

docker_run:
	 docker run -p 8081:8081 -d -v ./logs:/app/logs/ partivo_community:1.0

mod:
	go mod edit -module ${BINARY}; \
	find . -type f -name '*.go' -exec sed -i '' "s|${OLD_MODULE}|${BINARY}|g" {} +; \
	go mod tidy

clear_wire:
	rm cmd/server/.!*wire_gen.go

# Allow passing arguments like `make frontend ARGS="-c config.dev.yaml"`
frontend:
	@go run ./cmd/server serve:frontend $(ARGS)

# Corrected typo and allow passing arguments
console:
	@go run ./cmd/server serve:console $(ARGS)

consumer:
	@go run ./cmd/consumer $(ARGS)

###################
# Containerization
###################
.PHONY: docker-build docker-build-frontend docker-build-console docker-build-consumer docker-prune docker-run-frontend docker-stop-frontend

# Build all container images
docker-build: docker-build-frontend docker-build-console docker-build-consumer

# Build the frontend container image
docker-build-frontend:
	@echo "Building docker image partivo_tickets/frontend:$(GIT_HASH)..."
	@docker build -t partivo_tickets/frontend:$(GIT_HASH) -t partivo_tickets/frontend:latest --target server-frontend .

# Build the console container image
docker-build-console:
	@echo "Building docker image partivo_tickets/console:$(GIT_HASH)..."
	@docker build -t partivo_tickets/console:$(GIT_HASH) -t partivo_tickets/console:latest --target server-console .

# Build the consumer container image
docker-build-consumer:
	@echo "Building docker image partivo_tickets/consumer:$(GIT_HASH)..."
	@docker build -t partivo_tickets/consumer:$(GIT_HASH) -t partivo_tickets/consumer:latest --target consumer .

# Prune dangling docker images
docker-prune:
	@echo "Pruning dangling docker images..."
	@docker image prune -f

# Run the frontend container in detached mode
docker-run-frontend:
	@echo "Running frontend container in detached mode on port 8090..."
	@docker run --rm -d --name partivo_frontend -p 8090:8090 -v $(PWD)/logs:/app/logs partivo_tickets/frontend:latest

# Stop the frontend container
docker-stop-frontend:
	@echo "Stopping frontend container..."
	@docker stop partivo_frontend

# Run the console container in detached mode
docker-run-console:
	@echo "Running console container in detached mode on port 8090..."
	@docker run --rm -d --name partivo_console -p 8081:8081 -v $(PWD)/logs:/app/logs partivo_tickets/console:latest
# Stop the console container
docker-stop-console:
	@echo "Stopping console container..."
	@docker stop partivo_console

# Run the consumer container in detached mode
docker-run-consumer:
	@echo "Running consumer container in detached mode..."
	@docker run --rm -d --name partivo_consumer -v $(PWD)/logs:/app/logs partivo_tickets/consumer:latest

# Stop the consumer container
docker-stop-consumer:
	@echo "Stopping consumer container..."
	@docker stop partivo_consumer
