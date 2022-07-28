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

// AreLogsEqual checks if two arrays contain the same elements in the same order.
func AreLogsEqual(expected, actual []string) (bool, string) {
	if len(expected) != len(actual) {
		return false, fmt.Sprintf("Arrays are not the same length: (%d, %d)", len(expected), len(actual))
	}

	for i, v := range actual {
		if v != expected[i] {
			return false, printLogContents(expected, actual)
		}
	}
	return true, ""
}

func splitLog(log string) []string {
	return strings.Split(log, "\n")
}

// VerifyInitialLogState checks that the rgbkbd log contains the expected logs for the calls made at startup.
func VerifyInitialLogState(log string) (bool, string) {
	split := splitLog(log)
	expected := []string{"RGB::SetKeyColor - 44,255,255,210",
		"RGB::SetKeyColor - 57,255,255,210",
		"RGB::SetAllKeyColors - 255,255,210",
		"RGB::SetAllKeyColors - 0,81,231"}
	var actual []string = split[0:4]
	return AreLogsEqual(expected, actual)

}

// GetRemainingLog returns any logs written after the initial calls made at startup.
func GetRemainingLog(log string) (remainingLog []string) {
	split := splitLog(log)
	remainingLog = split[4 : len(split)-1]
	return remainingLog
}
