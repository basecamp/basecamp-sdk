.PHONY: build test check clean generate help

# Build the Swift SDK
build:
	@echo "==> Building Swift SDK..."
	swift build

# Run Swift tests
test:
	@echo "==> Running Swift tests..."
	swift test

# Run all Swift checks (build + test)
check: build test
	@echo "==> Swift SDK checks passed"

# Regenerate service files from OpenAPI spec and behavior model
generate:
	@echo "==> Generating Swift SDK services..."
	swift run BasecampGenerator --openapi ../openapi.json --behavior ../behavior-model.json --output Sources/Basecamp/Generated

# Clean build artifacts
clean:
	@echo "==> Cleaning Swift SDK..."
	swift package clean

# Help
help:
	@echo "Swift SDK Makefile"
	@echo ""
	@echo "  build      Build the Swift SDK"
	@echo "  test       Run Swift tests"
	@echo "  check      Run all checks (build + test)"
	@echo "  generate   Regenerate services from OpenAPI spec"
	@echo "  clean      Remove build artifacts"
	@echo "  help       Show this help"
