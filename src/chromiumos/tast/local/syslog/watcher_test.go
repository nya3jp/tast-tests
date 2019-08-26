// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package syslog

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tastTest "chromiumos/tast/testing"
)

const (
	target = "Target"
)

type testStruct struct {
	contents string
	expected []bool
}

var tests = []testStruct{
	{"hello\nStart Target End\ngoodbye\n", []bool{true, false}},
	{"hello\nStart Nope End\ngoodbye\n", []bool{false}},
	{"", []bool{false}},
	{"Target at beginning\n", []bool{true, false}},
	{"Target with no new line", []bool{true, false}},
	{"hello\nAt the end of file is Target\n", []bool{true, false}},
	{"hello\nAt the end of file without new line is Target", []bool{true, false}},
	{"Target", []bool{true, false}},
	{"hello\nhey target is case sensitive\n", []bool{false}},
	{"There are 2 Targets!\nrepeat 2 Targets!", []bool{true, true, false}},
	{"TargetTargetTarget", []bool{true, true, true, false}},
	{"TargetTargetTarget\n", []bool{true, true, true, false}},
	{strings.Repeat("long line ", 1000) + " Target", []bool{true, false}},
	{"last", []bool{false}},
}

func TestHasMessageBasic(t *testing.T) {
	// Create a file, look for the target message it in.
	const filename = "TestHasMessageBasic"
	for _, tc := range tests {
		// Go into a subfunction so that defers happen at the right time.
		func() {
			t.Logf("TestHasMessageBasic{%q, %v}\n", tc.contents, tc.expected)
			file, err := ioutil.TempFile("", filename)
			if err != nil {
				t.Fatalf("ioutil.TempFile(\"\", %s) failed: %v", filename, err)
			}
			defer os.Remove(file.Name())
			defer file.Close()

			w, err := NewWatcher(file.Name())
			if err != nil {
				t.Fatalf("Could not create Watcher on %s: %v", file.Name(), err)
			}
			defer w.Close()

			if _, err = file.WriteString(tc.contents); err != nil {
				t.Fatalf("WriteString() failed: %v", err)
			}
			if err = file.Sync(); err != nil {
				t.Fatalf("Sync failed: %v", err)
			}

			for i, expected := range tc.expected {
				if hasMessage, err := w.HasMessage(target); err != nil {
					t.Errorf("Error searching for message: %v", err)
				} else if !hasMessage && expected {
					t.Errorf("Did not find %s %d times in %q", target, i+1, tc.contents)
				} else if hasMessage && !expected {
					t.Errorf("Found %s %d times in %q", target, i+1, tc.contents)
				}
			}
		}()
	}
}

func TestHasMessageAddToFile(t *testing.T) {
	// Keep adding to a file, expect HasMessage to notice when the target appears.
	const filename = "TestHasMessageAddToFile"
	file, err := ioutil.TempFile("", filename)
	if err != nil {
		t.Fatalf("ioutil.TempFile(\"\", %s) failed: %v", filename, err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	w, err := NewWatcher(file.Name())
	if err != nil {
		t.Fatalf("Could not create Watcher on %s: %v", file.Name(), err)
	}
	defer w.Close()

	if hasMessage, err := w.HasMessage(target); err != nil {
		t.Errorf("Error searching for message: %v", err)
	} else if hasMessage {
		t.Errorf("Found %s before writing anything to file", target)
	}
	for _, tc := range tests {
		t.Logf("TestHasMessageAddToFile{%q, %v}\n", tc.contents, tc.expected)
		if _, err = file.WriteString(tc.contents); err != nil {
			t.Fatalf("WriteString() failed: %v", err)
		}
		if err = file.Sync(); err != nil {
			t.Fatalf("Sync failed: %v", err)
		}

		for i, expected := range tc.expected {
			if hasMessage, err := w.HasMessage(target); err != nil {
				t.Errorf("Error searching for message: %v", err)
			} else if !hasMessage && expected {
				t.Errorf("Did not find %s %d times in %q", target, i+1, tc.contents)
			} else if hasMessage && !expected {
				t.Errorf("Found %s %d times in %q", target, i+1, tc.contents)
			}
		}
	}

	if hasMessage, err := w.HasMessage(target); err != nil {
		t.Errorf("Error searching for message: %v", err)
	} else if hasMessage {
		t.Errorf("Found %s after searching entire file", target)
	}
}

func TestHasMessageAddToFileNoUnsuccesfulReads(t *testing.T) {
	// Basically the same as TestHasMessageAddToFile except that we always go
	// straight from one "Target" to the next. Every HasMessage should return true;
	// there are no unsuccessful reads to 'clear' the reader.
	const filename = "TestHasMessageAddToFileNoUnsuccesfulReads"
	file, err := ioutil.TempFile("", filename)
	if err != nil {
		t.Fatalf("ioutil.TempFile(\"\", %s) failed: %v", filename, err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	w, err := NewWatcher(file.Name())
	if err != nil {
		t.Fatalf("Could not create Watcher on %s: %v", file.Name(), err)
	}
	defer w.Close()
	for _, tc := range tests {
		t.Logf("TestHasMessageAddToFileNoUnsuccesfulReads{%q, %v}\n", tc.contents, tc.expected)
		if _, err = file.WriteString(tc.contents); err != nil {
			t.Fatalf("WriteString() failed: %v", err)
		}
		if err = file.Sync(); err != nil {
			t.Fatalf("Sync failed: %v", err)
		}

		for i, expected := range tc.expected {
			if expected {
				if hasMessage, err := w.HasMessage(target); err != nil {
					t.Errorf("Error searching for message: %v", err)
				} else if !hasMessage {
					t.Errorf("Did not find %s %d times in %q", target, i+1, tc.contents)
				}
			}
		}
	}

	if hasMessage, err := w.HasMessage(target); err != nil {
		t.Errorf("Error searching for message: %v", err)
	} else if hasMessage {
		t.Errorf("Found %s after searching entire file", target)
	}
}

func TestSplitAcrossWrites(t *testing.T) {
	const filename = "TestSplitAcrossWrites"
	file, err := ioutil.TempFile("", filename)
	if err != nil {
		t.Fatalf("ioutil.TempFile(\"\", %s) failed: %v", filename, err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	w, err := NewWatcher(file.Name())
	if err != nil {
		t.Fatalf("Could not create Watcher on %s: %v", file.Name(), err)
	}
	defer w.Close()

	if _, err = file.WriteString("Tar"); err != nil {
		t.Fatalf("WriteString(\"Tar\") failed: %v", err)
	}
	if err = file.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if hasMessage, err := w.HasMessage(target); err != nil {
		t.Errorf("Error searching for message: %v", err)
	} else if hasMessage {
		t.Errorf("Found %s before completely written", target)
	}

	if _, err = file.WriteString("get"); err != nil {
		t.Fatalf("WriteString(\"get\") failed: %v", err)
	}
	if err = file.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if hasMessage, err := w.HasMessage(target); err != nil {
		t.Errorf("Error searching for message: %v", err)
	} else if !hasMessage {
		t.Errorf("Did not find %s after completely written", target)
	}
}

func TestFileRotation(t *testing.T) {
	const (
		dirName             = "TestFileRotation"
		baseFilename        = "test_log"
		rotatedBaseFilename = baseFilename + ".1"
	)

	for _, tc := range []struct {
		before, after string
		expected      bool
	}{
		{"", "Target", true},
		{"Target", "", true},
		{"split file: Tar", "get", true},
		{"stuff and Target", "", true},
		{"", "stuff and Target", true},
		{"Target", "other stuff", true},
		{"other stuff", "Target", true},
		{"other stuff\n", "Target", true},
		{"", "", false},
		{"stuff", "", false},
		{"", "stuff", false},
		{"stuff", "more stuff", false},
		{"Target\nother line", "another line\nand another\n", true},
		{"line\n", "another line\nand Target\nand another\n", true},
	} {
		func() {
			t.Logf("TestFileRotation{%q, %q, %v}\n", tc.before, tc.after, tc.expected)
			tmpDir, err := ioutil.TempDir("", dirName)
			if err != nil {
				t.Fatalf("ioutil.TempDir(\"\", %s) failed: %v", dirName, err)
			}
			defer os.RemoveAll(tmpDir)

			filename := filepath.Join(tmpDir, baseFilename)
			file, err := os.Create(filename)
			if err != nil {
				t.Fatalf("os.Create(%s) failed: %v", filename, err)
			}
			fileOpen := true
			defer func() {
				if fileOpen {
					file.Close()
				}
			}()

			w, err := NewWatcher(filename)
			if err != nil {
				t.Fatalf("Could not create Watcher on %s: %v", filename, err)
			}
			defer w.Close()

			if _, err = file.WriteString(tc.before); err != nil {
				t.Fatalf("WriteString(%s) failed: %v", tc.before, err)
			}
			err = file.Close()
			fileOpen = false
			if err != nil {
				t.Fatalf("First close failed: %v", err)
			}
			rotatedFilename := filepath.Join(tmpDir, rotatedBaseFilename)
			err = os.Rename(filename, rotatedFilename)
			if err != nil {
				t.Fatalf("Rename %s -> %s failed: %v", filename, rotatedFilename, err)
			}
			file, err = os.Create(filename)
			if err != nil {
				t.Fatalf("os.Create(%s) failed: %v", filename, err)
			}
			fileOpen = true
			if _, err = file.WriteString(tc.after); err != nil {
				t.Fatalf("WriteString(%s) failed: %v", tc.after, err)
			}
			if err = file.Sync(); err != nil {
				t.Fatalf("Sync failed: %v", err)
			}
			if hasMessage, err := w.HasMessage(target); err != nil {
				t.Errorf("Error searching for message: %v", err)
			} else if !hasMessage && tc.expected {
				t.Errorf("Did not find %s in %q and then %q", target, tc.before, tc.after)
			} else if hasMessage && !tc.expected {
				t.Errorf("Found %s in %q and then %q", target, tc.before, tc.after)
			}
		}()
	}
}

func TestFileRaceCondition(t *testing.T) {
	// Test the race condition where a log is rotated but the new file isn't
	// created yet.
	const (
		dirName             = "TestFileRaceCondition"
		baseFilename        = "test_log"
		rotatedBaseFilename = baseFilename + ".1"
	)

	// Set up
	tmpDir, err := ioutil.TempDir("", dirName)
	if err != nil {
		t.Fatalf("ioutil.TempDir(\"\", %s) failed: %v", dirName, err)
	}
	defer os.RemoveAll(tmpDir)

	filename := filepath.Join(tmpDir, baseFilename)
	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("os.Create(%s) failed: %v", filename, err)
	}
	fileOpen := true
	defer func() {
		if fileOpen {
			file.Close()
		}
	}()

	w, err := NewWatcher(filename)
	if err != nil {
		t.Fatalf("Could not create Watcher on %s: %v", filename, err)
	}
	defer w.Close()

	if _, err = file.WriteString("Target 1\n"); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	err = file.Close()
	fileOpen = false
	if err != nil {
		t.Fatalf("First close failed: %v", err)
	}

	if hasMessage, err := w.HasMessage(target); err != nil {
		t.Errorf("Error searching for message: %v", err)
	} else if !hasMessage {
		t.Error("Did not find message at beginning of test")
	}

	if hasMessage, err := w.HasMessage(target); err != nil {
		t.Errorf("Error searching for message (2nd time): %v", err)
	} else if hasMessage {
		t.Error("Found message twice at beginning of test")
	}

	// Rotate but don't create new file.
	rotatedFilename := filepath.Join(tmpDir, rotatedBaseFilename)
	err = os.Rename(filename, rotatedFilename)
	if err != nil {
		t.Fatalf("Rename %s -> %s failed: %v", filename, rotatedFilename, err)
	}

	if hasMessage, err := w.HasMessage(target); err != nil {
		t.Errorf("Error searching for message after rotation begun: %v", err)
	} else if hasMessage {
		t.Error("Found message after rotation begun")
	}

	// Create new file; Watcher should pick it up.
	file, err = os.Create(filename)
	if err != nil {
		t.Fatalf("os.Create(%s) failed: %v", filename, err)
	}
	fileOpen = true
	if _, err = file.WriteString("Target 2"); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	if err = file.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if hasMessage, err := w.HasMessage(target); err != nil {
		t.Errorf("Error searching for message after rotation finished: %v", err)
	} else if !hasMessage {
		t.Error("Did not find message after rotation finished")
	}
}

func TestWaitForMessageMessageExistsBefore(t *testing.T) {
	const filename = "TestWaitForMessageMessageExistsBefore"
	file, err := ioutil.TempFile("", filename)
	if err != nil {
		t.Fatalf("ioutil.TempFile(\"\", %s) failed: %v", filename, err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	w, err := NewWatcher(file.Name())
	if err != nil {
		t.Fatalf("Could not create Watcher on %s: %v", file.Name(), err)
	}
	defer w.Close()
	if _, err = file.WriteString(target); err != nil {
		t.Fatalf("WriteString(%s) failed: %v", target, err)
	}
	if err = file.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if err := w.WaitForMessage(context.Background(), target); err != nil {
		t.Errorf("WaitForMessage failed: %v", err)
	}
}

func TestWaitForMessageMessageAddedDuring(t *testing.T) {
	const filename = "TestWaitForMessageMessageAddedDuring"
	file, err := ioutil.TempFile("", filename)
	if err != nil {
		t.Fatalf("ioutil.TempFile(\"\", %s) failed: %v", filename, err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	w, err := NewWatcher(file.Name())
	if err != nil {
		t.Fatalf("Could not create Watcher on %s: %v", file.Name(), err)
	}
	defer w.Close()

	go func() {
		// As best we can, try to ensure main thread is already inside
		// WaitForMessage() when we write to the file. We have no way to know when
		// WaitForMessage() actually starts polling, so we do the best we can.
		tastTest.Sleep(context.Background(), time.Second)

		if _, err := file.WriteString(target); err != nil {
			t.Fatalf("WriteString(%s) failed: %v", target, err)
		}
		if err := file.Sync(); err != nil {
			t.Fatalf("Sync failed: %v", err)
		}
	}()

	if err := w.WaitForMessage(context.Background(), target); err != nil {
		t.Errorf("WaitForMessage failed: %v", err)
	}
}

func TestWaitForMessageFails(t *testing.T) {
	const filename = "TestWaitForMessageFails"
	file, err := ioutil.TempFile("", filename)
	if err != nil {
		t.Fatalf("ioutil.TempFile(\"\", %s) failed: %v", filename, err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	w, err := NewWatcher(file.Name())
	if err != nil {
		t.Fatalf("Could not create Watcher on %s: %v", file.Name(), err)
	}
	defer w.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := w.WaitForMessage(ctx, target); err == nil {
		t.Error("WaitForMessage returned without error, but target was not present")
	} else if strings.Index(err.Error(), "message \"Target\" not found") == -1 {
		t.Errorf("WaitForMessage returned wrong error: %v", err)
	}
}
