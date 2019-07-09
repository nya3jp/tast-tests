// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gtest

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/testutil"
)

func TestReport(t *testing.T) {
	// The test data comes from
	// https://github.com/google/googletest/blob/437e1008c97b6bf595fec85da42c6925babd96b2/googletest/docs/advanced.md#generating-an-xml-report
	const data = `<?xml version="1.0" encoding="UTF-8"?>
	<testsuites tests="3" failures="1" errors="0" time="0.035" timestamp="2011-10-31T18:52:42" name="AllTests">
		<testsuite name="MathTest" tests="2" failures="1" errors="0" time="0.015">
			<testcase name="Addition" status="run" time="0.007" classname="">
				<failure message="Value of: add(1, 1)&#x0A;  Actual: 3&#x0A;Expected: 2" type="">...</failure>
				<failure message="Value of: add(1, -1)&#x0A;  Actual: 1&#x0A;Expected: 0" type="">...</failure>
			</testcase>
			<testcase name="Subtraction" status="run" time="0.005" classname="">
			</testcase>
		</testsuite>
		<testsuite name="LogicTest" tests="1" failures="0" errors="0" time="0.005">
			<testcase name="NonContradiction" status="run" time="0.005" classname="">
			</testcase>
		</testsuite>
	</testsuites>`

	dir := testutil.TempDir(t)
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "output.xml")
	if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal("Failed to create output.xml: ", err)
	}

	report, err := ParseReport(path)
	if err != nil {
		t.Fatal("Failed to parse output.xml: ", err)
	}

	expected := &Report{
		Suites: []*TestSuite{{
			Name: "MathTest",
			Cases: []*TestCase{{
				Name: "Addition",
				Failures: []Failure{{
					Message: "Value of: add(1, 1)\n  Actual: 3\nExpected: 2",
				}, {
					Message: "Value of: add(1, -1)\n  Actual: 1\nExpected: 0",
				}},
			}, {
				Name: "Subtraction",
			}},
		}, {
			Name: "LogicTest",
			Cases: []*TestCase{{
				Name: "NonContradiction",
			}},
		}},
	}
	if diff := cmp.Diff(report, expected); diff != "" {
		t.Fatalf("Unexpected parsed result: (-want, +got):\n%s", diff)
	}

	expectedNames := []string{"MathTest.Addition"}
	names := report.FailedTestNames()
	if !reflect.DeepEqual(names, expectedNames) {
		t.Errorf("Unexpected failed tests: got %v; want %v", names, expectedNames)
	}
}
