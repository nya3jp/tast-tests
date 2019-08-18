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
	"time"

	"chromiumos/tast/errors"
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
  TestCase5/2  # GetParam() = foo
`
	expected := []string{
		"TestSuite1.TestCase1",
		"TestSuite1.TestCase2",
		"TestSuite2.TestCase3",
		"TestSuite2.TestCase4",
		"TestSuite2.TestCase5/0",
		"TestSuite2.TestCase5/1",
		"TestSuite2.TestCase5/2",
	}
	result := parseTestList(content)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("parseTestList returns %s; want %s", result, expected)
	}
}

func TestGTestArgs(t *testing.T) {
	for _, tc := range []struct {
		expected []string
		opts     []option
	}{{
		expected: []string{"testexec", "--gtest_filter=pattern"},
		opts:     []option{Filter("pattern")},
	}, {
		expected: []string{"testexec"},
		opts:     []option{Filter("")},
	}, {
		expected: []string{"testexec", "--gtest_repeat=-1"},
		opts:     []option{Repeat(-1)},
	}, {
		expected: []string{"testexec", "a", "b", "c"},
		opts:     []option{ExtraArgs("a", "b", "c")},
	}, {
		expected: []string{"sudo", "--user=#10", "testexec"},
		opts:     []option{UID(10)},
	}} {
		args, err := New("testexec", tc.opts...).Args()
		if err != nil {
			t.Error("Unexpected fail: ", err)
		}
		if !reflect.DeepEqual(args, tc.expected) {
			t.Errorf("Unexpected args: got %#v; want %#v", args, tc.expected)
		}
	}

	// Test error case of ExtraArgs.
	if _, err := New("testexec", ExtraArgs("--gtest_something")).Args(); err == nil {
		t.Error("Unexpected success for ExtraArgs with --gtest prefix")
	}
}

const fakeGTest = `#!/bin/sh

if [[ "$1" == --gtest_output=xml:* ]]; then
  output="${1#--gtest_output=xml:}"
  echo "<testsuites></testsuites>" > "${output}"
fi
echo "test log"
exit 0
`

func setUpTest() (td, gtest string, retErr error) {
	td, err := ioutil.TempDir("", "gtest")
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create temp dir")
	}
	defer func() {
		if retErr != nil {
			os.RemoveAll(td)
		}
	}()

	gtest = filepath.Join(td, "gtest")
	if err := ioutil.WriteFile(gtest, []byte(fakeGTest), 0755); err != nil {
		return "", "", errors.Wrap(err, "failed to create an executable script")
	}

	return td, gtest, nil
}

func TestRun(t *testing.T) {
	td, gtest, err := setUpTest()
	if err != nil {
		t.Fatal("Failed to set up test: ", err)
	}
	defer os.RemoveAll(td)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if report, err := New(gtest).Run(ctx); err != nil {
		t.Fatal("Unexpected execution error: ", err)
	} else if report == nil {
		t.Fatal("Report is unexpectedly nil")
	}
}

func TestStart(t *testing.T) {
	td, gtest, err := setUpTest()
	if err != nil {
		t.Fatal("Failed to set up test: ", err)
	}
	defer os.RemoveAll(td)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd, err := New(gtest).Start(ctx)
	if err != nil {
		t.Fatal("Unexpected execution error: ", err)
	}
	// The command is expected to be successfully terminated quickly.
	if err := cmd.Wait(); err != nil {
		t.Fatal("Unexpected wait error: ", err)
	}
}

func TestLogfile(t *testing.T) {
	td, gtest, err := setUpTest()
	if err != nil {
		t.Fatal("Failed to set up test: ", err)
	}
	defer os.RemoveAll(td)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
		t.Fatalf("Unexpected log file: got %q", out)
	}
}

func TestTempLogfile(t *testing.T) {
	td, gtest, err := setUpTest()
	if err != nil {
		t.Fatal("Failed to set up test: ", err)
	}
	defer os.RemoveAll(td)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logpath := filepath.Join(td, "log_*.txt")

	if _, err := New(gtest, TempLogfile(logpath)).Run(ctx); err != nil {
		t.Fatal("Unexpected execution error: ", err)
	}

	matches, err := filepath.Glob(logpath)
	if err != nil {
		t.Fatal("Unexpected glob pattern: ", err)
	}
	if len(matches) != 1 {
		t.Fatal("Unexpected number of log files: got ", len(matches))
	}

	out, err := ioutil.ReadFile(matches[0])
	if err != nil {
		t.Fatal("Failed to read log file: ", err)
	}
	expect := regexp.MustCompile(`^Running .*/gtest --gtest_output=xml:.*\n\ntest log\n$`)
	if !expect.Match(out) {
		t.Fatalf("Unexpected log file: got %q", out)
	}

	// Run the test again, which should create a new logfile.
	if _, err := New(gtest, TempLogfile(logpath)).Run(ctx); err != nil {
		t.Fatal("Unexpected execution error: ", err)
	}

	if matches, err := filepath.Glob(logpath); err != nil {
		t.Fatal("Unexpected glob pattern: ", err)
	} else if len(matches) != 2 {
		t.Fatal("Unexpected number of log files: got ", len(matches))
	}
}
