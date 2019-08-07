// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package syslog

import (
	"context"
	"os"
	"strings"
	"testing"
)

const (
	target = "Target"
)

type testStruct struct {
	contents string
	expected bool
}

var tests = []testStruct{
	{"hello\nStart Target End\ngoodbye\n", true},
	{"hello\nStart Nope End\ngoodbye\n", false},
	{"", false},
	{"Target at beginning\n", true},
	{"Target with no new line", true},
	{"hello\nAt the end of file is Target\n", true},
	{"hello\nAt the end of file without new line is Target", true},
	{"Target", true},
	{"hello\nhey target is case sensitive\n", false},
	{"There are 2 Targets!\nrepeat 2 Targets!", true},
	{strings.Repeat("long line ", 1000) + " Target", true},
	{"last", false},
}

func TestHasMessageBasic(t *testing.T) {
	// Create a file, look for the target message it in.
	const filename = "/tmp/TestHasMessageBasic"
	ctx := context.Background()
	for _, tc := range tests {
		// Go into a subfunction so that defers happen at the right time.
		func() {
			t.Logf("TestHasMessageBasic{'%s', %v}\n", tc.contents, tc.expected)
			file, err := os.Create(filename)
			if err != nil {
				t.Fatalf("os.Create(%s) failed: %v", filename, err)
			}
			defer file.Close()

			w, err := NewWatcher(ctx, filename)
			if err != nil {
				t.Fatalf("Could not create Watcher: %v", err)
			}
			defer w.Close()

			if _, err = file.WriteString(tc.contents); err != nil {
				t.Fatalf("WriteString() failed: %v", err)
			}
			if err = file.Sync(); err != nil {
				t.Fatalf("Sync failed: %v", err)
			}

			if hasMessage, err := w.HasMessage(ctx, target); err != nil {
				t.Errorf("Error searching for message: %v", err)
			} else if !hasMessage && tc.expected {
				t.Errorf("Did not find %s in '%s'", target, tc.contents)
			} else if hasMessage && !tc.expected {
				t.Errorf("Found %s in '%s'", target, tc.contents)
			}
		}()
	}
}

func TestHasMessageAddToFile(t *testing.T) {
	// Keep adding to a file, expect HasMessage to notice when the target appears.
	const filename = "/tmp/TestHasMessageAddToFile"
	ctx := context.Background()
	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("os.Create(%s) failed: %v", filename, err)
	}
	defer file.Close()

	w, err := NewWatcher(ctx, filename)
	if err != nil {
		t.Fatalf("Could not create Watcher: %v", err)
	}
	defer w.Close()

	if hasMessage, err := w.HasMessage(ctx, target); err != nil {
		t.Errorf("Error searching for message: %v", err)
	} else if hasMessage {
		t.Errorf("Found %s before writing anything to file", target)
	}
	for _, tc := range tests {
		t.Logf("TestHasMessageAddToFile{'%s', %v}\n", tc.contents, tc.expected)
		if _, err = file.WriteString(tc.contents); err != nil {
			t.Fatalf("WriteString() failed: %v", err)
		}
		if err = file.Sync(); err != nil {
			t.Fatalf("Sync failed: %v", err)
		}

		if hasMessage, err := w.HasMessage(ctx, target); err != nil {
			t.Errorf("Error searching for message: %v", err)
		} else if !hasMessage && tc.expected {
			t.Errorf("Did not find %s in '%s'", target, tc.contents)
		} else if hasMessage && !tc.expected {
			t.Errorf("Found %s in '%s'", target, tc.contents)
		}
	}

	if hasMessage, err := w.HasMessage(ctx, target); err != nil {
		t.Errorf("Error searching for message: %v", err)
	} else if hasMessage {
		t.Errorf("Found %s after searching entire file", target)
	}
}

func TestFileRotation(t *testing.T) {
	const (
		filename        = "/tmp/TestFileRotation"
		rotatedFilename = filename + ".1"
	)
	ctx := context.Background()

	for _, tc := range []struct {
		before, after string
		expected      bool
	}{
		{"", "Target", true},
		{"Target", "", true},
		{"stuff and Target", "", true},
		{"", "stuff and Target", true},
		{"Target", "other stuff", true},
		{"other stuff", "Target", true},
		{"", "", false},
		{"stuff", "", false},
		{"", "stuff", false},
		{"stuff", "more stuff", false},
		{"Target\nother line", "another line\nand another\n", true},
		{"line\n", "another line\nand Target\nand another\n", true},
	} {
		func() {
			t.Logf("TestFileRotation{'%s', '%s', %v}\n", tc.before, tc.after, tc.expected)
			os.Remove(rotatedFilename)

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

			w, err := NewWatcher(ctx, filename)
			if err != nil {
				t.Fatalf("Could not create Watcher: %v", err)
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
			err = os.Rename(filename, rotatedFilename)
			if err != nil {
				t.Fatalf("Rename failed: %v", err)
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
			if hasMessage, err := w.HasMessage(ctx, target); err != nil {
				t.Errorf("Error searching for message: %v", err)
			} else if !hasMessage && tc.expected {
				t.Errorf("Did not find %s in '%s' and then '%s'", target, tc.before, tc.after)
			} else if hasMessage && !tc.expected {
				t.Errorf("Found %s in '%s' and then '%s'", target, tc.before, tc.after)
			}
		}()
	}
}

func TestFileRaceCondition(t *testing.T) {
	// Test the race condition where a log is rotated but the new file isn't
	// created yet.
	const (
		filename        = "/tmp/TestFileRaceCondition"
		rotatedFilename = filename + ".1"
	)
	ctx := context.Background()

	// Set up
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

	w, err := NewWatcher(ctx, filename)
	if err != nil {
		t.Fatalf("Could not create Watcher: %v", err)
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

	if hasMessage, err := w.HasMessage(ctx, target); err != nil {
		t.Errorf("Error searching for message: %v", err)
	} else if !hasMessage {
		t.Error("Did not find message at beginning of test")
	}

	if hasMessage, err := w.HasMessage(ctx, target); err != nil {
		t.Errorf("Error searching for message (2nd time): %v", err)
	} else if hasMessage {
		t.Error("Found message twice at beginning of test")
	}

	// Rotate but don't create new file.
	err = os.Rename(filename, rotatedFilename)
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	if hasMessage, err := w.HasMessage(ctx, target); err != nil {
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
	if hasMessage, err := w.HasMessage(ctx, target); err != nil {
		t.Errorf("Error searching for message after rotation finished: %v", err)
	} else if !hasMessage {
		t.Error("Did not find message after rotation finished")
	}
}
