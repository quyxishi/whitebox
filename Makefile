all: build test

# Build the application
build:
	@echo -- Building
	@go build -o main.exe cmd/api/main.go

# Run the application
run:
	@go run cmd/api/main.go

# Test the application
test:
	@echo -- Testing
	@go test ./... -v

# Clean the binary
clean:
	@echo -- Cleaning
	@rm -f main

# Live-Reload
watch:
	@powershell -ExecutionPolicy Bypass -Command "if (Get-Command air -ErrorAction SilentlyContinue) { \
		air; \
		Write-Output '-- Watching'; \
	} else { \
		Write-Output '-- Installing air'; \
		go install github.com/air-verse/air@latest; \
		air; \
		Write-Output '-- Watching'; \
	}"

.PHONY: all build run test clean watch
