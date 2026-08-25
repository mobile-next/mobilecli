# Testing

This document describes how to run tests for mobilecli.

## Unit Tests

Run Go unit tests:

```bash
make test
```

## Integration Tests

The integration tests use iOS simulator, Android real device and Android emulator, to test functionality.

### Prerequisites

1. Have one (or more) of these devices connected. Only the first device of each platform and type will be used:
  - Android real device
  - Android emulator, booted
  - iOS simulator, booted

2. **Install Node.js dependencies**
   ```bash
   cd test
   npm install
   ```

### Running Integration Tests

Run all integration tests:
```bash
make test-e2e
```

Running the tests will print out statement coverage per function. An output `coverage.html` will also be written, which is the same information but
more human-friendly.
