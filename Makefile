MODULE_NAME := wallet-service
PROTO_REPO := https://github.com/zoroplay/sbe-service-proto.git
PROTO_SRC_DIR := proto_src
PROTO_OUT_DIR := proto

.PHONY: update-proto gen-proto

update-proto:
	@echo "Fetching latest proto files..."
	@rm -rf $(PROTO_SRC_DIR)
	@mkdir -p $(PROTO_SRC_DIR)
	@git clone --depth 1 $(PROTO_REPO) /tmp/sbe-service-proto
	@cp /tmp/sbe-service-proto/proto/*.proto $(PROTO_SRC_DIR)/
	@rm -rf /tmp/sbe-service-proto
	@echo "Proto files updated in $(PROTO_SRC_DIR)"

gen-proto:
	@echo "Generating Go code from protos..."
	@mkdir -p $(PROTO_OUT_DIR)
	@protoc -I=$(PROTO_SRC_DIR) \
		--go_out=. --go_opt=module=$(MODULE_NAME) \
		--go-grpc_out=. --go-grpc_opt=module=$(MODULE_NAME) \
		$(PROTO_SRC_DIR)/*.proto
	@echo "Code generation complete."

migrate:
	@echo "Running database migrations..."
	@go run cmd/migrate/main.go
	@echo "Migrations complete."

build:
	@echo "Building application..."
	@go build -o bin/server main.go

start:
	@echo "Starting application..."
	@./bin/server

watch:
	@if command -v air > /dev/null; then \
		air; \
	else \
		read -p "Go's 'air' is not installed on your machine. Do you want to install it? [Y/n] " choice; \
		if [ "$$choice" != "n" ] && [ "$$choice" != "N" ]; then \
			go install github.com/air-verse/air@latest; \
			air; \
		else \
			echo "You chose not to install air. Exiting..."; \
			exit 1; \
		fi; \
	fi

