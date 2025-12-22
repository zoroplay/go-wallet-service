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
	@mkdir -p $(PROTO_OUT_DIR)/betting
	@mkdir -p $(PROTO_OUT_DIR)/bonus
	@mkdir -p $(PROTO_OUT_DIR)/gaming
	@mkdir -p $(PROTO_OUT_DIR)/fixture
	@mkdir -p $(PROTO_OUT_DIR)/retail
	@mkdir -p $(PROTO_OUT_DIR)/cashbook
	@mkdir -p $(PROTO_OUT_DIR)/affiliate
	@mkdir -p $(PROTO_OUT_DIR)/commission
	@mkdir -p $(PROTO_OUT_DIR)/noti
	@mkdir -p $(PROTO_OUT_DIR)/common
	@protoc -I=$(PROTO_SRC_DIR) \
		--go_out=$(PROTO_OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT_DIR) --go-grpc_opt=paths=source_relative \
		--go_opt=Mwallet.proto=wallet-service/proto/wallet \
		--go_opt=Midentity.proto=wallet-service/proto/identity \
		--go_opt=Mbetting.proto=wallet-service/proto/betting \
		--go_opt=Mbonus.proto=wallet-service/proto/bonus \
		--go_opt=Mgaming.proto=wallet-service/proto/gaming \
		--go_opt=Mfixture.proto=wallet-service/proto/fixture \
		--go_opt=Mretail.proto=wallet-service/proto/retail \
		--go_opt=Mcashbook.proto=wallet-service/proto/cashbook \
		--go_opt=Maffiliate.proto=wallet-service/proto/affiliate \
		--go_opt=Mcommission.proto=wallet-service/proto/commission \
		--go_opt=Mnoti.proto=wallet-service/proto/noti \
		--go_opt=Mcommon.proto=wallet-service/proto/common \
		--go_opt=Moutrights.proto=wallet-service/proto/outrights \
		--go-grpc_opt=Mwallet.proto=wallet-service/proto/wallet \
		--go-grpc_opt=Midentity.proto=wallet-service/proto/identity \
		--go-grpc_opt=Mbetting.proto=wallet-service/proto/betting \
		--go-grpc_opt=Mbonus.proto=wallet-service/proto/bonus \
		--go-grpc_opt=Mgaming.proto=wallet-service/proto/gaming \
		--go-grpc_opt=Mfixture.proto=wallet-service/proto/fixture \
		--go-grpc_opt=Mretail.proto=wallet-service/proto/retail \
		--go-grpc_opt=Mcashbook.proto=wallet-service/proto/cashbook \
		--go-grpc_opt=Maffiliate.proto=wallet-service/proto/affiliate \
		--go-grpc_opt=Mcommission.proto=wallet-service/proto/commission \
		--go-grpc_opt=Mnoti.proto=wallet-service/proto/noti \
		--go-grpc_opt=Mcommon.proto=wallet-service/proto/common \
		--go-grpc_opt=Moutrights.proto=wallet-service/proto/outrights \
		$(PROTO_SRC_DIR)/*.proto
	@echo "Code generation complete."
	@echo "Moving generated files to proper directories..."
	@mv $(PROTO_OUT_DIR)/*.pb.go $(PROTO_OUT_DIR)/wallet/ 2>/dev/null || true
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

