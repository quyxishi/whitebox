all: build test

# Build the application
build:
	@echo -- Building
	@go build -o whitebox.exe ./cmd/api

# Run the application
run:
	@echo -- Running
	@go run ./cmd/api $(filter-out run,$(MAKECMDGOALS))

# Test the application
test:
	@echo -- Testing
	@go test ./... -v

# Clean the binary
clean:
	@echo -- Cleaning
	@rm -f whitebox

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

%:
	@: