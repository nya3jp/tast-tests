// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/testutil"
)

// crashFile contains information about a crash file used by tests.
// The testutil package uses relative paths while the crash package
// uses absolute paths, so this struct stores both.
type crashFile struct{ rel, abs, data string }

// writeCrashFile writes a file with relative path rel containing data to dir.
func writeCrashFile(t *testing.T, dir, rel, data string) crashFile {
	cf := crashFile{rel, filepath.Join(dir, rel), data}
	if err := testutil.WriteFiles(dir, map[string]string{rel: data}); err != nil {
		t.Fatal(err)
	}
	return cf
}

func TestGetCrashes(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	writeCrashFile(t, td, "foo.txt", "") // skipped because non-core/dmp extension
	fooCore := writeCrashFile(t, td, "foo.core", "")
	fooDmp := writeCrashFile(t, td, "foo.dmp", "")
	fooLog := writeCrashFile(t, td, "foo.log", "")
	fooMeta := writeCrashFile(t, td, "foo.meta", "")
	fooInfo := writeCrashFile(t, td, "foo.info", "")
	fooProclog := writeCrashFile(t, td, "foo.proclog", "")
	fooGPU := writeCrashFile(t, td, "foo.i915_error_state.log.xz", "")
	fooCompressedTxt := writeCrashFile(t, td, "foo.txt.gz", "")
	fooBIOSLog := writeCrashFile(t, td, "foo.bios_log", "")
	fooKCrash := writeCrashFile(t, td, "foo.kcrash", "")
	fooCompressedLog := writeCrashFile(t, td, "foo.log.gz", "")
	barDmp := writeCrashFile(t, td, "bar.dmp", "")
	writeCrashFile(t, td, "bar", "")            // skipped because no extenison
	writeCrashFile(t, td, "subdir/baz.dmp", "") // skipped because in subdir
	writeCrashFile(t, td, "foo.info.gz", "")    // skipped because second extension is wrong
	writeCrashFile(t, td, "other.xz", "")

	dirs := []string{filepath.Join(td, "missing"), td} // nonexistent dir should be skipped
	files, err := GetCrashes(dirs...)
	if err != nil {
		t.Fatalf("GetCrashes(%v) failed: %v", dirs, err)
	}
	sort.Strings(files)
	if exp := []string{barDmp.abs, fooBIOSLog.abs, fooCore.abs, fooDmp.abs, fooGPU.abs, fooInfo.abs, fooKCrash.abs, fooLog.abs, fooCompressedLog.abs, fooMeta.abs, fooProclog.abs, fooCompressedTxt.abs}; !reflect.DeepEqual(files, exp) {
		t.Errorf("GetCrashes(%v) = %v; want %v", dirs, files, exp)
	}
}

func TestProcessRunning(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	// procName must be <=14 characters long so that gopsutil doesn't look at
	// /proc/$$/cmdline.
	procName := fmt.Sprintf("t_%d", rand.Int31())
	if err := ioutil.WriteFile(filepath.Join(td, procName), []byte("#!/bin/sh\nsleep 10\n"), 0777); err != nil {
		t.Fatal("Failed to write a script: ", err)
	}

	cmd := exec.Command(filepath.Join(td, procName))
	if err := cmd.Start(); err != nil {
		t.Fatal("Failed to start a script: ", err)
	}
	func() {
		defer cmd.Wait()
		defer cmd.Process.Kill()

		running, err := processRunning(procName)
		if err != nil {
			t.Fatal("processRunning: ", err)
		}
		if !running {
			t.Fatal("processRunning = false; want true")
		}
	}()

	running, err := processRunning(procName)
	if err != nil {
		t.Fatal("processRunning: ", err)
	}
	if running {
		t.Fatal("processRunning = true; want false")
	}
}

func TestDeleteCoreDumps(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	dir1 := filepath.Join(td, "dir1") // dir1 is missing
	dir2 := filepath.Join(td, "dir2")
	if err := os.Mkdir(dir2, 0777); err != nil {
		t.Fatal("Failed to create dir: ", err)
	}

	initFiles := []string{"a.core", "a.txt", "b.core", "b.dmp", "b.jpg"}
	for _, fn := range initFiles {
		if err := ioutil.WriteFile(filepath.Join(dir2, fn), nil, 0666); err != nil {
			t.Fatal("Failed to touch file: ", err)
		}
	}

	// filesIn returns a list of files under dir.
	filesIn := func(dir string) []string {
		fis, err := ioutil.ReadDir(dir)
		if err != nil {
			t.Fatal("ReadDir failed: ", err)
		}
		var files []string
		for _, fi := range fis {
			files = append(files, fi.Name())
		}
		sort.Strings(files)
		return files
	}

	// First, test the behavior when crash_reporter is running.
	func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		reporterRunning := func() (bool, error) {
			// Cancel the context so that deleteCoreDumps returns after observing exactly once that
			// crash_reporter is running.
			cancel()
			return true, nil
		}

		if err := deleteCoreDumps(ctx, []string{dir1, dir2}, reporterRunning); err == nil {
			t.Error("deleteCoreDumps succeeded unexpectedly while crash_reporter is running")
		}

		got := filesIn(dir2)
		if diff := cmp.Diff(got, initFiles); diff != "" {
			t.Error("Files mismatch after failed deleteCoreDumps (-got +want):\n", diff)
		}
	}()

	// Second, test the behavior when crash_reporter is not running.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reporterRunning := func() (bool, error) {
		// Create a new core file. This should not be deleted.
		if err := ioutil.WriteFile(filepath.Join(dir2, "c.core"), nil, 0666); err != nil {
			t.Fatal("Failed to touch file: ", err)
		}
		return false, nil
	}

	if err := deleteCoreDumps(ctx, []string{dir1, dir2}, reporterRunning); err != nil {
		t.Error("deleteCoreDumps failed when crash_reporter is not running: ", err)
	}

	// a.core: Not deleted because a.dmp does not exist.
	// b.core: Deleted.
	// c.core: Not deleted because it was created after waiting for crash_reporter.
	got := filesIn(dir2)
	want := []string{"a.core", "a.txt", "b.dmp", "b.jpg", "c.core"}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Error("Files mismatch after successful deleteCoreDumps (-got +want):\n", diff)
	}
}
