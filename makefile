APP_NAME=simple-voting-system
BUILD_DIR=build
LAMBDA_BINARY=bootstrap
ZIP_FILE=$(BUILD_DIR)/lambda.zip

.PHONY: build-lambda build-lambda-arm test clean pack up wait-localstack run-local

# Build the Go binary for Lambda (linux target, no CGO)
build-lambda:
	mkdir -p $(BUILD_DIR)
	cp config.yaml $(BUILD_DIR)/
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(LAMBDA_BINARY) main.go

# Build the Go binary for Lambda (linux arm64 target, no CGO)
build-lambda-arm:
	mkdir -p $(BUILD_DIR)
	cp config.yaml $(BUILD_DIR)/
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(LAMBDA_BINARY) main.go

test: up wait-localstack
	go test -v ./...

up:
	docker compose up -d

# Run the API locally with Swagger enabled
run-local: test
	rm -rf docs
	swag init
	APP_ENV=local go run main.go

# Package Lambda binary after tests pass
pack: test build-lambda-arm
	cd $(BUILD_DIR) && zip -r lambda.zip $(LAMBDA_BINARY) config.yaml

# Clean build and zip files
clean:
	rm -rf $(BUILD_DIR) $(ZIP_FILE)

wait-localstack:
	@echo "Waiting for LocalStack (dynamodb) to be ready..."
	@retries=15; \
	while ! curl -s http://localhost:4566/_localstack/health | grep '"dynamodb": *"running"' > /dev/null; do \
		sleep 2; \
		retries=$$((retries - 1)); \
		if [ $$retries -le 0 ]; then \
			echo "Timeout waiting for LocalStack"; \
			exit 1; \
		fi; \
	done