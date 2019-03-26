// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package logging

import (
	"regexp"
	"strings"

	"chromiumos/tast/errors"
)

// This file contains helper functions for parsing logs from ARC e2e tests.

var regExpFailureSummary = regexp.MustCompile(`(?m)^\[\s*FAILED\s*\]\s+\d+\s+tests?,\s+listed\s+below:$`)
var regExpFailureTest = regexp.MustCompile(`(?m)^\[\s*FAILED\s*\]\s+([\w\./]+)`)

// CheckARCTestResult extracts failed test information from log, and return as an error.
// Return nil if all tests are passed according to log.
func CheckARCTestResult(log string) error {
	// Passed log example:
	// [----------] Global test environment tear-down
	// [==========] 2 tests from 1 test case ran. (954 ms total)
	// [  PASSED  ] 2 tests.
	//
	// Failed log example:
	// [----------] Global test environment tear-down
	// [==========] 2 tests from 2 tests case ran. (589 ms total)
	// [  PASSED  ] 0 tests.
	// [  FAILED  ] 2 tests, listed below:
	// [  FAILED  ] ArcVideoEncoderE2ETest.TestSimpleEncode
	// [  FAILED  ] ArcVideoEncoderE2ETest.TestBitrate
	//
	// First of all we need to find tearDownKeyWord ("Global test environment tear-down") appearance in
	// log, which represents exec was finished successfully. Then we try to find the failure summary line
	// ("2 tests, listed below:"), if tests are all passed this line cannot be found; otherwise, the
	// following lines will list the failed tests per line.
	const tearDownKeyWord = "Global test environment tear-down"
	tearDownPos := strings.Index(log, tearDownKeyWord)
	if tearDownPos < 0 {
		return errors.New("cannot find tear-down key word, exec may be terminated abnormally?")
	}

	summaryPos := regExpFailureSummary.FindStringIndex(log[tearDownPos:])
	if summaryPos == nil {
		return nil // test passed
	}

	matches := regExpFailureTest.FindAllStringSubmatch(log[tearDownPos+summaryPos[1]:], -1)
	if len(matches) == 0 {
		return errors.New("failed to get failed tests after failure summary line")
	}

	// Gather failed test items and report as an error.
	failedTests := []string{}
	for _, match := range matches {
		failedTests = append(failedTests, match[1])
	}
	return errors.Errorf("%d failure(s): %v", len(failedTests), strings.Join(failedTests, ", "))
}
