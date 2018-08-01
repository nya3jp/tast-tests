// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"path/filepath"
	"reflect"
	"testing"
)

// Location of test data files as a sub-folder of where this file is located.
const deqpParserTestData = "testdata/deqp_parser"

func TestParseDEQPOutput(t *testing.T) {
	// Wild* test cases use data collected from running DEQP on an actual device
	// and possibly breaking Mesa to induce failures. Fake* are made-up log
	// files to exercise corner cases.
	for _, tc := range []struct {
		name    string
		stats   map[string]uint
		failed  []string
		wantErr bool
	}{
		{"WildSingleSuccess", map[string]uint{"pass": 1}, nil, false},
		{"WildSingleFail", map[string]uint{"fail": 1}, []string{"dEQP-GLES2.functional.prerequisite.read_pixels"}, false},
		{"WildSingleWatchdogTimeout", map[string]uint{"timeout": 1}, []string{"dEQP-GLES2.functional.prerequisite.read_pixels"}, false},
		{"WildSingleIncomplete", map[string]uint{"parsefailed": 1}, []string{"dEQP-GLES2.functional.prerequisite.read_pixels"}, false},
		{"WildMultiplePassAndFail", map[string]uint{"pass": 2, "fail": 1}, []string{"dEQP-GLES2.functional.prerequisite.read_pixels"}, false},
		{"WildMultipleWatchdogTimeout", map[string]uint{"timeout": 1, "pass": 1}, []string{"dEQP-GLES2.functional.prerequisite.clear_color"}, false},
		{"WildMultipleIncomplete", map[string]uint{"pass": 1, "parsefailed": 1}, []string{"dEQP-GLES2.functional.prerequisite.clear_color"}, false},
		{"FakeNotExistent", nil, nil, true},
		{"FakeEmpty", nil, nil, false},
		{"FakeNoTestCases", nil, nil, false},
		{"FakeBeginWithoutEnd1", nil, nil, true},
		{"FakeBeginWithoutEnd2", map[string]uint{"pass": 1, "parsefailed": 1}, []string{"dEQP-GLES2.functional.prerequisite.read_pixels2"}, false},
		{"FakeEndWithoutBegin1", nil, nil, true},
		{"FakeEndWithoutBegin2", nil, nil, true},
		{"FakeTerminateWithoutBegin1", nil, nil, true},
		{"FakeTerminateWithoutBegin2", nil, nil, true},
		{"FakeBeginWithoutTestName", nil, nil, true},
		{"FakeTerminateWithoutCauseNotLastLine1", nil, nil, true},
		{"FakeTerminateWithoutCauseNotLastLine2", nil, nil, true},
		{"FakeTerminateWithoutCauseLastLine", map[string]uint{"parsefailed": 1}, []string{"dEQP-GLES2.functional.prerequisite.read_pixels"}, false},
		{"FakeBadXMLIncomplete", nil, nil, true},
		{"FakeBadXMLNoCasePath", nil, nil, true},
		{"FakeBadXMLCasePathMismatch", nil, nil, true},
		{"FakeBadXMLNoResult", nil, nil, true},
		{"FakeBadXMLManyResults", nil, nil, true},
		{"FakeBadXMLNoStatusCode", nil, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(deqpParserTestData, tc.name+".log")
			astats, afailed, aerr := ParseDEQPOutput(p)
			// Treat an empty slice/map and a nil return value as
			// interchangeable.
			// TODO(andrescj): do this also in ParseUIFlags to be consistent.
			if len(astats) == 0 {
				astats = nil
			}
			if len(afailed) == 0 {
				afailed = nil
			}
			if tc.wantErr {
				if aerr == nil {
					t.Errorf("ParseDEQPOutput(%q) unexpectedly succeeded", p)
				} else {
					t.Log(aerr)
				}
				return
			}

			if aerr != nil {
				t.Errorf("ParseDEQPOutput(%q) unexpectedly failed: %v", p, aerr)
			} else if !reflect.DeepEqual(tc.stats, astats) || !reflect.DeepEqual(tc.failed, afailed) {
				t.Errorf("ParseDEQPOutput(%q) = [%v, %v]; want [%v, %v]",
					p, astats, afailed, tc.stats, tc.failed)
			}
		})
	}
}
