# Testing

This document describes how to run tests for mobilecli.

## Unit Tests

Run Go unit tests:

```bash
make test
```

## Integration Tests

The integration tests use iOS simulators to test real device functionality.

### Prerequisites

1. **Install iOS Simulator Runtimes**
   - Open Xcode
   - **Settings** → **Components**
   - Click the **"+"** button in the bottom-left corner
   - Install the following iOS platforms:
     - **iOS 16.4** (Build 20E247)
     - **iOS 17.5** (Build 21F79) 
     - **iOS 18.6** (Build 22G86)

2. **Install Node.js dependencies**
   ```bash
   cd test
   npm install
   ```

### Running Integration Tests

Run all integration tests:
```bash
cd test
npm run test
```

Run tests for specific iOS version:
```bash
cd test
npm run test -- --grep "iOS 16"
npm run test -- --grep "iOS 17" 
npm run test -- --grep "iOS 18"
```

### Test Behavior

- Tests automatically skip if the required iOS runtime is not installed
- Each test creates a fresh simulator and cleans up after completion
- Tests include screenshot capture and URL opening functionality

