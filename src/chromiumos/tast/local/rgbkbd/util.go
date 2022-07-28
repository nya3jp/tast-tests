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

// AreLastLogLinesEqual extracts the last N lines of a log where N is the
// length of the |expected| log and validates that the rgbkbd log matches
// our expected log.
func AreLastLogLinesEqual(expected []string, actual string) (bool, string) {
	linesToCheck := getLastNLines(actual, len(expected))
	if len(expected) != len(linesToCheck) {
		return false, fmt.Sprintf("Logs are not the same length: (%d, %d)", len(expected), len(linesToCheck))
	}

	for i, v := range linesToCheck {
		if v != expected[i] {
			return false, printLogContents(expected, linesToCheck)
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

// getLastNLines returns any logs written starting at the passed in |num|.
func getLastNLines(log string, num int) (remainingLog []string) {
	split := splitLog(log)
	remainingLog = split[len(split)-num:]
	return remainingLog
}
