# pipefail, so a failed xcodebuild is not hidden by the pipe into xcbeautify
SHELL := /bin/bash -o pipefail

# Project configuration
PROJECT = devicekit-ios.xcodeproj
SCHEME = devicekit-ios
BUILD_DIR = build
ARCHIVE_PATH = $(BUILD_DIR)/$(SCHEME).xcarchive
EXPORT_PATH = $(BUILD_DIR)/export

# Build configuration (Debug or Release)
CONFIGURATION ?= Release

# Code signing
DEVELOPMENT_TEAM ?=
CODE_SIGN_IDENTITY ?= Apple Development

# Export method for IPA (development, ad-hoc, app-store, enterprise)
EXPORT_METHOD ?= development

# Minimum iOS version override. Unset uses the project's 15.0, the lowest Xcode 27 accepts;
# the release workflow sets 14.0 so published builds keep running on iOS 14.
IPHONEOS_DEPLOYMENT_TARGET ?=
DEPLOYMENT_TARGET_SETTING = $(if $(IPHONEOS_DEPLOYMENT_TARGET),IPHONEOS_DEPLOYMENT_TARGET=$(IPHONEOS_DEPLOYMENT_TARGET))

.PHONY: help clean build archive ipa-unsigned sim-zip-arm64 sim-zip-x86_64 sim-zip sim-install test-coverage coverage-html lint

.DEFAULT_GOAL := help

help:
	@echo "Available targets:"
	@echo "  sim-zip          Build XCUITest runner zips for both arm64 and x86_64 simulators"
	@echo "  sim-zip-arm64    Build XCUITest runner zip for iOS Simulator (arm64 / Apple Silicon)"
	@echo "  sim-zip-x86_64   Build XCUITest runner zip for iOS Simulator (x86_64 / Intel)"
	@echo "  sim-install      Build and install on the currently booted simulator"
	@echo "  ipa-unsigned     Build unsigned runner IPA + main app IPA for real iOS devices"
	@echo "  debug            Build with Debug configuration"
	@echo "  release          Build with Release configuration"
	@echo "  clean            Remove build artifacts"
	@echo "  test-coverage    Run tests with code coverage"
	@echo "  coverage-html    Generate HTML coverage report (run after test-coverage)"
	@echo "  lint             Run SwiftLint"

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	xcodebuild clean -project $(PROJECT) -scheme $(SCHEME) -configuration $(CONFIGURATION)

debug:
	@$(MAKE) build CONFIGURATION=Debug

release:
	@$(MAKE) build CONFIGURATION=Release

# Create unsigned IPA with XCUITest runner for real iOS devices
ipa-unsigned:
	@echo "Building unsigned test runner for arm64 iOS devices..."
	xcodebuild build-for-testing \
		-project $(PROJECT) \
		-scheme $(SCHEME) \
		-configuration $(CONFIGURATION) \
		-destination 'generic/platform=iOS' \
		-derivedDataPath $(BUILD_DIR) \
		CODE_SIGN_IDENTITY="" \
		CODE_SIGNING_REQUIRED=NO \
		CODE_SIGNING_ALLOWED=NO \
		$(DEPLOYMENT_TARGET_SETTING) | xcbeautify
	@scripts/patch-runner.sh "$(BUILD_DIR)/Build/Products/$(CONFIGURATION)-iphoneos"
	@echo "Packaging runner IPA..."
	@rm -rf $(EXPORT_PATH)/Payload
	@rm -f $(EXPORT_PATH)/$(SCHEME)-runner.ipa
	@mkdir -p $(EXPORT_PATH)/Payload
	@cp -r "$(BUILD_DIR)/Build/Products/$(CONFIGURATION)-iphoneos/$(SCHEME)UITests-Runner.app" $(EXPORT_PATH)/Payload/
	@cd $(EXPORT_PATH) && rm -f $(SCHEME)-runner.ipa && zip -r $(SCHEME)-runner.ipa Payload
	@rm -rf $(EXPORT_PATH)/Payload
	@echo "Runner IPA created at: $(EXPORT_PATH)/$(SCHEME)-runner.ipa"
	@echo "Packaging main app IPA..."
	@rm -rf $(EXPORT_PATH)/Payload
	@rm -f $(EXPORT_PATH)/$(SCHEME).ipa
	@mkdir -p $(EXPORT_PATH)/Payload
	@cp -r "$(BUILD_DIR)/Build/Products/$(CONFIGURATION)-iphoneos/$(SCHEME).app" $(EXPORT_PATH)/Payload/
	@cd $(EXPORT_PATH) && rm -f $(SCHEME).ipa && zip -r $(SCHEME).ipa Payload
	@rm -rf $(EXPORT_PATH)/Payload
	@echo "App IPA created at: $(EXPORT_PATH)/$(SCHEME).ipa"

# Build XCUITest runner for iOS Simulator (arm64 — Apple Silicon)
sim-zip-arm64:
	@echo "Building $(SCHEME) XCUITest runner for iOS Simulator (arm64)..."
	xcodebuild build-for-testing \
		-project $(PROJECT) \
		-scheme $(SCHEME) \
		-configuration $(CONFIGURATION) \
		-destination 'generic/platform=iOS Simulator' \
		-derivedDataPath $(BUILD_DIR) \
		CODE_SIGN_IDENTITY="" \
		CODE_SIGNING_REQUIRED=NO \
		CODE_SIGNING_ALLOWED=NO \
		ARCHS=arm64 \
		$(DEPLOYMENT_TARGET_SETTING) | xcbeautify
	@mkdir -p $(EXPORT_PATH)
	@rm -rf "$(EXPORT_PATH)/$(SCHEME)UITests-Runner.app"
	@cp -r "$(BUILD_DIR)/Build/Products/$(CONFIGURATION)-iphonesimulator/$(SCHEME)UITests-Runner.app" $(EXPORT_PATH)/
	@scripts/patch-runner.sh "$(BUILD_DIR)/Build/Products/$(CONFIGURATION)-iphonesimulator" "$(EXPORT_PATH)"
	@cd $(EXPORT_PATH) && rm -f $(SCHEME)-Sim-arm64.zip && zip -r $(SCHEME)-Sim-arm64.zip $(SCHEME)UITests-Runner.app
	@rm -rf "$(EXPORT_PATH)/$(SCHEME)UITests-Runner.app"
	@echo "Simulator zip created at: $(EXPORT_PATH)/$(SCHEME)-Sim-arm64.zip"

# Build XCUITest runner for iOS Simulator (x86_64 — Intel)
sim-zip-x86_64:
	@echo "Building $(SCHEME) XCUITest runner for iOS Simulator (x86_64)..."
	xcodebuild build-for-testing \
		-project $(PROJECT) \
		-scheme $(SCHEME) \
		-configuration $(CONFIGURATION) \
		-destination 'generic/platform=iOS Simulator' \
		-derivedDataPath $(BUILD_DIR) \
		CODE_SIGN_IDENTITY="" \
		CODE_SIGNING_REQUIRED=NO \
		CODE_SIGNING_ALLOWED=NO \
		ARCHS=x86_64 \
		$(DEPLOYMENT_TARGET_SETTING) | xcbeautify
	@mkdir -p $(EXPORT_PATH)
	@rm -rf "$(EXPORT_PATH)/$(SCHEME)UITests-Runner.app"
	@cp -r "$(BUILD_DIR)/Build/Products/$(CONFIGURATION)-iphonesimulator/$(SCHEME)UITests-Runner.app" $(EXPORT_PATH)/
	@scripts/patch-runner.sh "$(BUILD_DIR)/Build/Products/$(CONFIGURATION)-iphonesimulator" "$(EXPORT_PATH)"
	@cd $(EXPORT_PATH) && rm -f $(SCHEME)-Sim-x86_64.zip && zip -r $(SCHEME)-Sim-x86_64.zip $(SCHEME)UITests-Runner.app
	@rm -rf "$(EXPORT_PATH)/$(SCHEME)UITests-Runner.app"
	@echo "Simulator zip created at: $(EXPORT_PATH)/$(SCHEME)-Sim-x86_64.zip"

# Build both simulator zips
sim-zip: sim-zip-arm64 sim-zip-x86_64

# Build and install on booted simulator
sim-install:
	@BOOTED=$$(xcrun simctl list devices booted -j | jq -r '[.devices[][] | select(.state=="Booted")] | first | .udid'); \
	echo "Building for simulator $$BOOTED..."; \
	xcodebuild build-for-testing \
		-project $(PROJECT) \
		-scheme $(SCHEME) \
		-configuration $(CONFIGURATION) \
		-destination "id=$$BOOTED" \
		-derivedDataPath $(BUILD_DIR)/local | xcbeautify; \
	PRODUCTS="$(BUILD_DIR)/local/Build/Products/$(CONFIGURATION)-iphonesimulator"; \
	scripts/patch-runner.sh "$$PRODUCTS"; \
	xcrun simctl install "$$BOOTED" "$$PRODUCTS/$(SCHEME).app"; \
	xcrun simctl install "$$BOOTED" "$$PRODUCTS/$(SCHEME)UITests-Runner.app"; \
	echo "Installed on simulator $$BOOTED"

# Build, run Playwright tests with code coverage
test-coverage:
	@rm -rf $(BUILD_DIR)/coverage.xcresult
	@BOOTED=$$(xcrun simctl list devices booted -j | jq -r '[.devices[][] | select(.state=="Booted")] | first | .udid'); \
	scripts/test-coverage.sh $(PROJECT) $(SCHEME) "$$BOOTED" $(BUILD_DIR)

# Generate HTML coverage report (run after test-coverage)
coverage-html:
	@PROFDATA="$(BUILD_DIR)/local/coverage/Coverage.profdata"; \
	BINARY="$(BUILD_DIR)/local/Build/Products/Debug-iphonesimulator/$(SCHEME)UITests-Runner.app/PlugIns/$(SCHEME)UITests.xctest/$(SCHEME)UITests"; \
	if [ ! -f "$$PROFDATA" ] || [ ! -f "$$BINARY" ]; then echo "error: Run 'make test-coverage' first"; exit 1; fi; \
	rm -rf coverage-html; \
	xcrun llvm-cov show "$$BINARY" -instr-profile "$$PROFDATA" -format=html -output-dir=coverage-html \
		-ignore-filename-regex='build/local/SourcePackages|DerivedSources'; \
	echo "Coverage report: coverage-html/index.html"; \
	open coverage-html/index.html

# Run SwiftLint
lint:
	@echo "Running SwiftLint..."
	swiftlint lint --strict
