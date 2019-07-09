// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gtest

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"

	"chromiumos/tast/errors"
)

// Report is a parsed gtest output report.
// See https://github.com/google/googletest/blob/master/googletest/docs/advanced.md#generating-an-xml-report for details.
// Note: when this is implemented, gtest version is 1.8.0, which does not
// support JSON report. Consider switching when it's ready.
// Note: at the moment, only a subset of the report is parsed. More can be
// added upon requirements.
type Report struct {
	Suites []*TestSuite `xml:"testsuite"`
}

// TestSuite represents a testsuite run in Report.
type TestSuite struct {
	Name  string      `xml:"name,attr"`
	Cases []*TestCase `xml:"testcase"`
}

// TestCase represents a testcase run in TestSuite.
type TestCase struct {
	Name     string    `xml:"name,attr"`
	Failures []Failure `xml:"failure"`
}

// Failure represents a test validation failure in TestCase.
type Failure struct {
	Message string `xml:"message,attr"`
}

// Failures returns an array of failed test names, in the "TestSuite.TestCase"
// format. If no error founds, returns nil.
// This walks through whole the report, so if returned value is used, it is
// nice to be cached in a local variable.
func (r *Report) Failures() []string {
	var ret []string
	for _, s := range r.Suites {
		for _, c := range s.Cases {
			if len(c.Failures) > 0 {
				ret = append(ret, fmt.Sprintf("%s.%s", s.Name, c.Name))
			}
		}
	}
	return ret
}

// ParseReport parses the XML gtest output report at path.
func ParseReport(path string) (*Report, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %s", path)
	}
	ret := &Report{}
	if err := xml.Unmarshal(b, ret); err != nil {
		return nil, errors.Wrap(err, "failed to parse gtest xml report")
	}
	return ret, nil
}
