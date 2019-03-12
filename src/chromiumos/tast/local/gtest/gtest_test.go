// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gtest

import (
	"reflect"
	"testing"
)

func TestParseTestList(t *testing.T) {
	const content = `TestSuite1.
  TestCase1
  TestCase2
TestSuite2.
  TestCase3
  TestCase4
  TestCase5/0
  TestCase5/1
`
	expected := []string{
		"TestSuite1.TestCase1",
		"TestSuite1.TestCase2",
		"TestSuite2.TestCase3",
		"TestSuite2.TestCase4",
		"TestSuite2.TestCase5/0",
		"TestSuite2.TestCase5/1",
	}
	result := parseTestList(content)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("parseTestList returns %s; want %s", result, expected)
	}
}
