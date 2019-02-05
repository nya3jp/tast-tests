// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bintest

import (
	"encoding/json"
	"fmt"
	"io"
)

// googleTestCase represents a test case in Google Test framework.
type googleTestCase struct {
	SuiteName string
	CaseName  string
}

func (t *googleTestCase) String() string {
	return fmt.Sprintf("%+v", *t)
}

// extractFailedTests returns an array of failed test cases extracted from jsonData,
// which is reported by Google Test.
func extractFailedTests(r io.Reader) ([]*googleTestCase, error) {
	// Prepare a struct to decode a JSON reported by a Google Test binary, which contains
	// test results. Its format is defined in:
	// https://cs.chromium.org/chromium/src/third_party/googletest/src/googletest/docs/advanced.md?l=2289&rcl=5ec7f0c4a113e2f18ac2c6cc7df51ad6afc24081
	var res struct {
		TestSuites []*struct {
			Name      string `json:"name"`
			TestSuite []*struct {
				Name     string         `json:"name"`
				Failures []*interface{} `json:"failures"`
			} `json:"testsuite"`
		} `json:"testsuites"`
	}

	if err := json.NewDecoder(r).Decode(&res); err != nil {
		return nil, err
	}

	var failures []*googleTestCase
	for _, suite := range res.TestSuites {
		for _, cas := range suite.TestSuite {
			if len(cas.Failures) == 0 {
				continue
			}
			failures = append(failures,
				&googleTestCase{
					SuiteName: suite.Name,
					CaseName:  cas.Name,
				})
		}
	}
	return failures, nil
}
