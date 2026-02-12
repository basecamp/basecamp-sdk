.PHONY: build test check clean help

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
	@echo "  clean      Remove build artifacts"
	@echo "  help       Show this help"
