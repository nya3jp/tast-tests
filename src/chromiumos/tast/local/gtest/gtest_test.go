// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gtest

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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

func TestGTestToArgs(t *testing.T) {
	for _, tc := range []struct {
		expected []string
		opts     []option
	}{{
		expected: []string{"testexec", "--gtest_filter=pattern"},
		opts:     []option{Filter("pattern")},
	}, {
		expected: []string{"testexec", "a", "b", "c"},
		opts:     []option{ExtraArgs("a", "b", "c")},
	}, {
		expected: []string{"sudo", "--user=#10", "testexec"},
		opts:     []option{UID(10)},
	}} {
		args, err := New("testexec", tc.opts...).ToArgs()
		if err != nil {
			t.Error("Unexpected fail: ", err)
		}
		if !reflect.DeepEqual(args, tc.expected) {
			t.Errorf("Unexpected args: got %+v; want %+v", args, tc.expected)
		}
	}

	// Test error case of ExtraArgs.
	if _, err := New("testexec", ExtraArgs("--gtest_something")).ToArgs(); err == nil {
		t.Error("Unexpected success for ExtraArgs with --gtest prefix")
	}
}

const fakeGTest = `#!/bin/sh
output="${1#--gtest_output=xml:}"
echo "<testsuites></testsuites>" > "${output}"
echo "test log"
`

func TestLogfile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	td, err := ioutil.TempDir("", "gtest")
	if err != nil {
		t.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(td)

	gtest := filepath.Join(td, "gtest")
	if err := ioutil.WriteFile(gtest, []byte(fakeGTest), 0755); err != nil {
		t.Fatal("Failed to create executable script: ", err)
	}

	logpath := filepath.Join(td, "log.txt")

	if _, err := New(gtest, Logfile(logpath)).Run(ctx); err != nil {
		t.Fatal("Unexpected execution error: ", err)
	}

	out, err := ioutil.ReadFile(logpath)
	if err != nil {
		t.Fatal("Failed to read log file: ", err)
	}
	expect := regexp.MustCompile(`^Running .*/gtest --gtest_output=xml:.*\n\ntest log\n$`)
	if !expect.Match(out) {
		t.Fatalf("Unexpected log file: got %q", string(out))
	}
}

func TestTempLogfile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	td, err := ioutil.TempDir("", "gtest")
	if err != nil {
		t.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(td)

	gtest := filepath.Join(td, "gtest")
	if err := ioutil.WriteFile(gtest, []byte(fakeGTest), 0755); err != nil {
		t.Fatal("Failed to create executable script: ", err)
	}

	logpath := filepath.Join(td, "log_*.txt")

	if _, err := New(gtest, TempLogfile(logpath)).Run(ctx); err != nil {
		t.Fatal("Unexpected execution error: ", err)
	}

	if matches, err := filepath.Glob(logpath); err != nil {
		t.Fatal("Unexpected glob pattern: ", err)
	} else if len(matches) != 1 {
		t.Fatal("Unexpected number of log files: got ", len(matches))
	}
}
