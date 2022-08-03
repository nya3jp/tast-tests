// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rgbkbd contains utilities for rgbkbd tests.
package rgbkbd

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	"chromiumos/tast/errors"
)

const logFilename = "capslock_rgb_state_updates.log"

func writeErrOutputToLog(outDir, log string) error {
	if err := ioutil.WriteFile(filepath.Join(outDir, logFilename),
		[]byte(log), 0644); err != nil {
		return err
	}
	return nil
}

func createLog(expected, actual []string) string {
	content := fmt.Sprintf("Expected:\n%v\n\nActual:\n%v\n", expected, actual)
	return content
}

// LastLogLinesMatch extracts the last N lines of a log where N is the
// length of the |expected| log and validates that the rgbkbd log matches
// our expected log.
func LastLogLinesMatch(expected []string, actual, outDir string) (bool, error) {
	linesToCheck, err := lastNLines(actual, len(expected))
	if err != nil {
		return false, err
	}

	matches := reflect.DeepEqual(linesToCheck, expected)
	if !matches {
		err := writeErrOutputToLog(outDir, createLog(expected, linesToCheck))
		if err != nil {
			return false, errors.Wrap(err, "unable to write to log file")
		}
		return matches, errors.Errorf("unexpected logs, find more details in %s", filepath.Join(outDir, logFilename))
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

// lastNLines returns any logs written starting at |num|.
func lastNLines(log string, num int) (remainingLog []string, err error) {
	split := splitLog(log)
	if len(split) < num {
		return nil, errors.Errorf("Length of split log array (%d) is < num: %d", len(split), num)
	}
	remainingLog = split[len(split)-num:]
	return remainingLog, nil
}

// RainbowModeCount returns the number of 'SetKeyColor' logs written when rainbow mode is called.
func RainbowModeCount(log string) (int, error) {
	splitLog := splitLog(log)
	// Every rainbow mode struct starts with this entry.
	const rainbowModeStart = "RGB::SetKeyColor - 1"
	for i, line := range splitLog {
		if strings.Contains(line, rainbowModeStart) {
			return len(splitLog) - i, nil
		}
	}
	return -1, errors.New("failed to get rainbow mode count")
}
