// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bintest

import (
	"encoding/json"
)

type testCase struct {
	Name     string
	Failures []struct {
		Message string
	}
}

type testSuite struct {
	Name      string
	TestSuite []testCase
}

// googleTestResult is a struct to decode a JSON reported by a Google Test binary, which contains
// test results. Its format is defined in:
// https://cs.chromium.org/chromium/src/third_party/googletest/src/googletest/docs/advanced.md?l=2289&rcl=5ec7f0c4a113e2f18ac2c6cc7df51ad6afc24081
type googleTestResult struct {
	Tests      int
	Failures   int
	TestSuites []testSuite
}

// GoogleTestCase represents a test case in Google Test framework.
type GoogleTestCase struct {
	SuiteName string
	CaseName  string
}

// extractFailedCases returns an array of failed test cases extracted from jsonData,
// which is reported by Google Test.
func extractFailedCases(jsonData []byte) ([]GoogleTestCase, error) {
	res := googleTestResult{}
	if err := json.Unmarshal(jsonData, &res); err != nil {
		return nil, err
	}

	var failures []GoogleTestCase
	for _, suite := range res.TestSuites {
		for _, cas := range suite.TestSuite {
			if len(cas.Failures) == 0 {
				continue
			}
			failures = append(failures,
				GoogleTestCase{
					SuiteName: suite.Name,
					CaseName:  cas.Name,
				})
		}
	}
	return failures, nil
}
