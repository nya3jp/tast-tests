// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Location of test data files as a sub-folder of where this file is located.
const deqpParserTestData = "testdata/deqp_parser"

func TestParseDEQPOutput(t *testing.T) {
	// Wild* test cases use data collected from running DEQP on an actual device
	// and possibly breaking Mesa to induce failures. Fake* cases are made-up
	// log files to exercise corner cases.
	for _, tc := range []struct {
		name      string
		stats     map[string]uint
		nonFailed string
		wantErr   bool
	}{
		{"WildSingleSuccess", map[string]uint{"pass": 1}, "dEQP-GLES2.functional.prerequisite.read_pixels", false},
		{"WildSingleFail", map[string]uint{"fail": 1}, "", false},
		{"WildSingleWatchdogTimeout", map[string]uint{"timeout": 1}, "", false},
		{"WildSingleIncomplete", map[string]uint{"parsefailed": 1}, "", false},
		{"WildMultiplePassAndFail", map[string]uint{"pass": 2, "fail": 1}, "dEQP-GLES2.info.vendor dEQP-GLES2.functional.prerequisite.clear_color", false},
		{"WildMultipleWatchdogTimeout", map[string]uint{"timeout": 1, "pass": 1}, "dEQP-GLES2.info.vendor", false},
		{"WildMultipleIncomplete", map[string]uint{"pass": 1, "parsefailed": 1}, "dEQP-GLES2.info.vendor", false},
		{"FakeNonExistent", nil, "", true},
		{"FakeEmpty", nil, "", false},
		{"FakeNoTestCases", nil, "", false},
		{"FakeBeginWithoutEnd1", nil, "", true},
		{"FakeBeginWithoutEnd2", map[string]uint{"pass": 1, "parsefailed": 1}, "dEQP-GLES2.functional.prerequisite.read_pixels", false},
		{"FakeEndWithoutBegin1", nil, "", true},
		{"FakeEndWithoutBegin2", nil, "", true},
		{"FakeTerminateWithoutBegin1", nil, "", true},
		{"FakeTerminateWithoutBegin2", nil, "", true},
		{"FakeBeginWithoutTestName", nil, "", true},
		{"FakeTerminateWithoutCauseNotLastLine1", nil, "", true},
		{"FakeTerminateWithoutCauseNotLastLine2", nil, "", true},
		{"FakeTerminateWithoutCauseLastLine", map[string]uint{"parsefailed": 1}, "", false},
		{"FakeBadXMLIncomplete", nil, "", true},
		{"FakeBadXMLNoCasePath", nil, "", true},
		{"FakeBadXMLCasePathMismatch", nil, "", true},
		{"FakeBadXMLNoResult", nil, "", true},
		{"FakeBadXMLManyResults", nil, "", true},
		{"FakeBadXMLNoStatusCode", nil, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(deqpParserTestData, tc.name+".log")
			astats, anonFailed, aerr := ParseDEQPOutput(p)
			// Treat an empty slice/map and a nil return value as
			// interchangeable.
			// TODO(andrescj): do this also in ParseUIFlags to be consistent.
			if len(astats) == 0 {
				astats = nil
			}
			if len(anonFailed) == 0 {
				anonFailed = []string{}
			}
			if tc.wantErr {
				if aerr == nil {
					t.Errorf("ParseDEQPOutput(%q) unexpectedly succeeded", p)
				}
				return
			}

			if aerr != nil {
				t.Errorf("ParseDEQPOutput(%q) unexpectedly failed: %v", p, aerr)
			} else if !reflect.DeepEqual(tc.stats, astats) || !reflect.DeepEqual(strings.Fields(tc.nonFailed), anonFailed) {
				t.Errorf("ParseDEQPOutput(%q) = [%v, %v]; want [%v, %v]",
					p, astats, anonFailed, tc.stats, tc.nonFailed)
			}
		})
	}
}
