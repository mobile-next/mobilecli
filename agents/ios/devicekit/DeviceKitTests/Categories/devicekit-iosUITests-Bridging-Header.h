/**
 * @file devicekit-iosUITests-Bridging-Header.h
 * @brief Bridging header exposing Objective-C APIs to Swift.
 *
 * This file imports Objective-C headers that need to be accessible from
 * Swift code in the devicekit-iosUITests target. These include:
 *
 * - XCUIApplication helper methods
 * - Accessibility client proxy for app enumeration
 * - Snapshot parameter swizzling (XCAXClient_iOS)
 *
 * @note Headers imported here are automatically available in Swift without
 *       explicit import statements.
 */

// MARK: - XCUIApplication Categories

/** Private XCUIApplication API (bundleID, applicationImpl). */
#import "XCUIApplication.h"

/** Helper methods for querying active applications. */
#import "XCUIApplication+Helper.h"

// MARK: - Accessibility Client

/** Proxy for accessing XCTest's accessibility interface. */
#import "AXClientProxy.h"

// MARK: - Snapshot Configuration

/** Accessibility client whose defaultParameters AXClientSwizzler overrides. */
#import "XCAXClient_iOS.h"

// MARK: - Fast Screenshot

/** Fast screenshot capture using XCTest daemon proxy. */
#import "DKScreenshot.h"
