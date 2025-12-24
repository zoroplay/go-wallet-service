MODULE_NAME := wallet-service
PROTO_LOCAL_DIR := ../sbe-service-proto/proto
PROTO_SRC_DIR := proto_src
PROTO_OUT_DIR := proto

.PHONY: update-proto gen-proto proto-all gen-wallet-proto

# Fetch latest proto files from local sbe-service-proto directory
update-proto-local:
	@echo "Copying latest proto files from local sbe-service-proto..."
	@rm -rf $(PROTO_SRC_DIR)
	@mkdir -p $(PROTO_SRC_DIR)
	@cp $(PROTO_LOCAL_DIR)/*.proto $(PROTO_SRC_DIR)/
	@echo "Proto files copied to $(PROTO_SRC_DIR)"

# Fetch latest proto files from remote repo
update-proto:
	@echo "Fetching latest proto files from remote..."
	@rm -rf $(PROTO_SRC_DIR)
	@mkdir -p $(PROTO_SRC_DIR)
	@git clone --depth 1 https://github.com/zoroplay/sbe-service-proto.git /tmp/sbe-service-proto
	@cp /tmp/sbe-service-proto/proto/*.proto $(PROTO_SRC_DIR)/
	@rm -rf /tmp/sbe-service-proto
	@echo "Proto files updated in $(PROTO_SRC_DIR)"

# Generate Go code from all proto files
gen-proto:
	@echo "Generating Go code from protos..."
	@mkdir -p $(PROTO_OUT_DIR)/wallet
	@mkdir -p $(PROTO_OUT_DIR)/identity
	@mkdir -p $(PROTO_OUT_DIR)/bonus
	@protoc -I=$(PROTO_SRC_DIR) \
		--go_out=$(PROTO_OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT_DIR) --go-grpc_opt=paths=source_relative \
		--go_opt=Mwallet.proto=wallet-service/proto/wallet \
		--go_opt=Midentity.proto=wallet-service/proto/identity \
		--go_opt=Mbonus.proto=wallet-service/proto/bonus \
		--go-grpc_opt=Mwallet.proto=wallet-service/proto/wallet \
		--go-grpc_opt=Midentity.proto=wallet-service/proto/identity \
		--go-grpc_opt=Mbonus.proto=wallet-service/proto/bonus \
		$(PROTO_SRC_DIR)/*.proto
	@echo "Code generation complete."
	@echo "Moving generated files to proper directories..."
	@mv $(PROTO_OUT_DIR)/wallet.pb.go $(PROTO_OUT_DIR)/wallet/ 2>/dev/null || true
	@mv $(PROTO_OUT_DIR)/wallet_grpc.pb.go $(PROTO_OUT_DIR)/wallet/ 2>/dev/null || true
	@mv $(PROTO_OUT_DIR)/identity.pb.go $(PROTO_OUT_DIR)/identity/ 2>/dev/null || true
	@mv $(PROTO_OUT_DIR)/identity_grpc.pb.go $(PROTO_OUT_DIR)/identity/ 2>/dev/null || true
	@mv $(PROTO_OUT_DIR)/bonus.pb.go $(PROTO_OUT_DIR)/bonus/ 2>/dev/null || true
	@mv $(PROTO_OUT_DIR)/bonus_grpc.pb.go $(PROTO_OUT_DIR)/bonus/ 2>/dev/null || true
	@echo "Done!"

# Generate only wallet.proto (useful for quick iterations)
gen-wallet-proto:
	@echo "Generating Go code from wallet.proto..."
	@mkdir -p $(PROTO_OUT_DIR)/wallet
	@protoc -I=$(PROTO_SRC_DIR) \
		--go_out=$(PROTO_OUT_DIR)/wallet --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT_DIR)/wallet --go-grpc_opt=paths=source_relative \
		$(PROTO_SRC_DIR)/wallet.proto
	@echo "wallet.pb.go and wallet_grpc.pb.go generated."

# Full proto update: copy local + generate
proto-all: update-proto-local gen-proto
	@echo "Proto update complete!"

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

