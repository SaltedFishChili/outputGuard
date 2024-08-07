# 定义变量
SERVER_DIR=./cmd/server
GATEWAY_DIR=./cmd/gateway
ROUTER_DIR=./cmd/router
TARGET_DIR=./cmd/target
DOCKER_DIR=./docker
SERVER_TARGET=$(TARGET_DIR)/server
GATEWAY_TARGET=$(TARGET_DIR)/gateway
ROUTER_TARGET=$(TARGET_DIR)/router

# 默认平台
GOOS ?= linux
GOARCH ?= amd64

# 默认目标
all: server gateway router

# 构建 server
server:
	@echo "Building server for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(SERVER_TARGET)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(SERVER_TARGET)/server $(SERVER_DIR)
	@if [ $$? -eq 0 ]; then echo "Server build completed successfully. Output directory: $(SERVER_TARGET)"; fi
	@cp -r static config $(SERVER_TARGET)


# 构建 gateway
gateway:
	@echo "Building gateway for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(GATEWAY_TARGET)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(GATEWAY_TARGET)/gateway $(GATEWAY_DIR)
	@if [ $$? -eq 0 ]; then echo "Gateway build completed successfully. Output directory: $(GATEWAY_TARGET)"; fi

# 构建 router
router:
	@echo "Building router for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(ROUTER_TARGET)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(ROUTER_TARGET)/router $(ROUTER_DIR)
	@if [ $$? -eq 0 ]; then echo "Router build completed successfully. Output directory: $(ROUTER_TARGET)"; fi

# 清理构建结果
clean:
	@echo "Cleaning up..."
	@rm -rf $(TARGET_DIR)
	@echo "Clean up completed."

# 帮助信息
help:
	@echo "Usage: make [target] [GOOS=target_os GOARCH=target_arch]"
	@echo "Targets:"
	@echo "  all           - Build all components (default)"
	@echo "  server        - Build the server component"
	@echo "  gateway       - Build the gateway component"
	@echo "  router        - Build the router component"
	@echo "  clean         - Clean up build artifacts"
	@echo "  help          - Display this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  GOOS          - Target operating system (default: linux)"
	@echo "  GOARCH        - Target architecture (default: amd64)"