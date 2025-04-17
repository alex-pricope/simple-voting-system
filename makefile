# Makefile for Hackathon Voting System

APP_NAME=simple-voting-system
BUILD_DIR=build
LAMBDA_BINARY=bootstrap
ZIP_FILE=lambda.zip

.PHONY: build test clean package

# Build the Go binary for Lambda (linux target, no CGO)
build:
	go build -o $(BUILD_DIR)/$(LAMBDA_BINARY) main.go

# Run tests (placeholder for now)
test:
	go test ./...

# Run the API locally with Swagger enabled
run-local:
	rm -rf docs
	swag init
	APP_ENV=local go run main.go

# Create a zip file for Lambda deployment
package: build
	cd $(BUILD_DIR) && zip -r ../$(ZIP_FILE) $(LAMBDA_BINARY)

# Clean build and zip files
clean:
	rm -rf $(BUILD_DIR) $(ZIP_FILE)