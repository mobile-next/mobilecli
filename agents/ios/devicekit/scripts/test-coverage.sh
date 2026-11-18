#!/bin/sh
# Build an instrumented XCUITest runner, install it on the booted simulator, and
# run the Playwright tests against it. Playwright's globalSetup launches the runner
# (on port 12004); this script collects Swift code coverage from the run.
# Usage: scripts/test-coverage.sh <project> <scheme> <simulator-udid> <build-dir>

set -e

PROJECT="$1"
SCHEME="$2"
BOOTED="$3"
BUILD_DIR="$4"
RUNNER_BUNDLE_ID="com.mobilenext.devicekit-iosUITests.xctrunner"
PRODUCTS="${BUILD_DIR}/local/Build/Products/Debug-iphonesimulator"
COVERAGE_DIR="${BUILD_DIR}/local/coverage"

# Build the runner with coverage instrumentation
echo "Building instrumented XCUITest runner..."
xcodebuild build-for-testing \
    -project "$PROJECT" \
    -scheme "$SCHEME" \
    -configuration Debug \
    -destination "id=$BOOTED" \
    -derivedDataPath "$BUILD_DIR/local" \
    -enableCodeCoverage YES \
    > /dev/null

# Install host app + runner on the booted simulator
echo "Installing apps on simulator $BOOTED..."
xcrun simctl install "$BOOTED" "$PRODUCTS/${SCHEME}.app"
xcrun simctl install "$BOOTED" "$PRODUCTS/${SCHEME}UITests-Runner.app"

# Prepare coverage output dir. iOS simulators run on the host filesystem, so the
# instrumented runner writes .profraw straight to this host path via LLVM_PROFILE_FILE
# (forwarded into the simulator process by simctl as SIMCTL_CHILD_LLVM_PROFILE_FILE).
rm -rf "$COVERAGE_DIR"
mkdir -p "$COVERAGE_DIR"
COVERAGE_DIR_ABS="$(cd "$COVERAGE_DIR" && pwd)"

# Run Playwright tests. globalSetup terminates any stale runner and launches a fresh
# one on port 12004; DEVICEKIT_SIMULATOR_UDID pins it to this simulator.
echo "Running Playwright tests..."
TEST_EXIT=0
( cd tests && \
    SIMCTL_CHILD_LLVM_PROFILE_FILE="${COVERAGE_DIR_ABS}/%p.profraw" \
    DEVICEKIT_SIMULATOR_UDID="$BOOTED" \
    npm test ) || TEST_EXIT=$?

# Terminate the runner so the instrumented binary flushes its .profraw on exit
echo "Stopping XCUITest server..."
xcrun simctl terminate "$BOOTED" "$RUNNER_BUNDLE_ID" > /dev/null 2>&1 || true
sleep 2  # give the profile writer a moment to flush

# Merge profraw -> profdata and report
PROFDATA="${COVERAGE_DIR_ABS}/Coverage.profdata"
BINARY="$PRODUCTS/${SCHEME}UITests-Runner.app/PlugIns/${SCHEME}UITests.xctest/${SCHEME}UITests"

if ls "${COVERAGE_DIR_ABS}"/*.profraw > /dev/null 2>&1 && [ -f "$BINARY" ]; then
    xcrun llvm-profdata merge -sparse "${COVERAGE_DIR_ABS}"/*.profraw -o "$PROFDATA"
    echo ""
    echo "=== Coverage Report ==="
    xcrun llvm-cov report "$BINARY" -instr-profile "$PROFDATA" -ignore-filename-regex='build/local/SourcePackages|DerivedSources'
    echo ""
    echo "For detailed line coverage:"
    echo "  xcrun llvm-cov show $BINARY -instr-profile $PROFDATA -format=html -output-dir=coverage-html"
else
    echo "warning: Coverage data not found"
fi

exit $TEST_EXIT
