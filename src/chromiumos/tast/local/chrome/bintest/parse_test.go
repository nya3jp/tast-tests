// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bintest

import (
	"testing"
)

func TestParseGtestJSON(t *testing.T) {
	// Test data comes from
	// https://cs.chromium.org/chromium/src/third_party/googletest/src/googletest/docs/advanced.md?l=2427&rcl=5ec7f0c4a113e2f18ac2c6cc7df51ad6afc24081
	data := `{
  "tests": 3,
  "failures": 1,
  "errors": 0,
  "time": "0.035s",
  "timestamp": "2011-10-31T18:52:42Z",
  "name": "AllTests",
  "testsuites": [
    {
      "name": "MathTest",
      "tests": 2,
      "failures": 1,
      "errors": 0,
      "time": "0.015s",
      "testsuite": [
        {
          "name": "Addition",
          "status": "RUN",
          "time": "0.007s",
          "classname": "",
          "failures": [
            {
              "message": "Value of: add(1, 1) Actual: 3\n Expected: 2",
              "type": ""
            },
            {
              "message": "Value of: add(1, -1) Actual: 1\n Expected: 0",
              "type": ""
            }
          ]
        },
        {
          "name": "Subtraction",
          "status": "RUN",
          "time": "0.005s",
          "classname": ""
        }
      ]
    },
    {
      "name": "LogicTest",
      "tests": 1,
      "failures": 0,
      "errors": 0,
      "time": "0.005s",
      "testsuite": [
        {
          "name": "NonContradiction",
          "status": "RUN",
          "time": "0.005s",
          "classname": ""
        }
      ]
    }
  ]
}`

	failures := []GoogleTestCase{
		{"MathTest", "Addition"},
	}

	res, err := extractFailedCases([]byte(data))
	if err != nil {
		t.Fatal("Failed to extract failed cases: ", err)
	}

	if len(res) != len(failures) {
		t.Fatalf("Got a response whose length is incorrect : Actual %d, Expected: %d",
			len(res), len(failures))
	}

	for i, actual := range res {
		expected := failures[i]
		if actual != expected {
			t.Errorf("Got unexpected failed cases : Actual %s/%s, Expected: %s/%s",
				actual.SuiteName, actual.CaseName,
				expected.SuiteName, expected.CaseName)

		}
	}
}
