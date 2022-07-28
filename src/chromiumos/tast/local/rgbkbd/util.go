// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rgbkbd contains utilities for rgbkbd tests.
package rgbkbd

import (
	"fmt"
	"reflect"
	"strings"

	"chromiumos/tast/errors"
)

func printLogContents(expected, actual []string) string {
	content := fmt.Sprintf("Expected: %v | Actual: %v", expected, actual)
	return content
}

// LastLogLinesMatch extracts the last N lines of a log where N is the
// length of the |expected| log and validates that the rgbkbd log matches
// our expected log.
func LastLogLinesMatch(expected []string, actual string) (bool, error) {
	linesToCheck, err := getLastNLines(actual, len(expected))
	if err != nil {
		return false, err
	}

	matches := reflect.DeepEqual(linesToCheck, expected)
	if !matches {
		return matches, errors.New(printLogContents(expected, linesToCheck))
	}
	return matches, nil
}

// splitLog returns the log string as an array of individual entries.
// We use '\n' as the separator because a newline is added between each log
// entry.
func splitLog(log string) []string {
	lines := strings.Split(log, "\n")
	// Remove last entry since '\n' adds an empty element to the end of the arr
	return lines[:len(lines)-1]
}

// getLastNLines returns any logs written starting at |num|.
func getLastNLines(log string, num int) (remainingLog []string, err error) {
	split := splitLog(log)
	if len(split) < num {
		return nil, errors.Errorf("Length of split log array (%d) is < num: %d", len(split), num)
	}
	errors.New("empty name")
	remainingLog = split[len(split)-num:]
	return remainingLog, nil
}
