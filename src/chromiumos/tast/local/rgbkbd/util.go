// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rgbkbd contains utilities for rgbkbd tests.
package rgbkbd

import (
	"fmt"
	"strings"
)

func printLogContents(expected, actual []string) string {
	content := fmt.Sprintf("Expected: %v | Actual: %v", expected, actual)
	return content
}

// AreLogsEqual checks if two logs contain the same entries in the same order.
func AreLogsEqual(expected, actual []string) (bool, string) {
	if len(expected) != len(actual) {
		return false, fmt.Sprintf("Logs are not the same length: (%d, %d)", len(expected), len(actual))
	}

	for i, v := range actual {
		if v != expected[i] {
			return false, printLogContents(expected, actual)
		}
	}
	return true, ""
}

// splitLog returns the log string as an array of individual entries.
// We use '\n' as the separator because a newline is added between each log
// entry.
func splitLog(log string) []string {
	lines := strings.Split(log, "\n")
	// Remove last entry since '\n' adds an empty element to the end of the arr
	return lines[:len(lines)-1]
}

// TODO(michaelcheco): Add tast test that verifies calls made at startup
// (inital caps lock state, default keyboard backlight color) when RGB keyboard
// is supported.

// GetRemainingLog returns any logs written after the initial calls made at startup.
func GetRemainingLog(log string) (remainingLog []string) {
	split := splitLog(log)
	remainingLog = split[4:]
	return remainingLog
}
