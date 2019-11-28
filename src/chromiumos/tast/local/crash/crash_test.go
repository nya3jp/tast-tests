// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"chromiumos/tast/errors"
	"chromiumos/tast/testutil"
)

// statAll attempts to stat all given files, returning an error if any call fails.
func statAll(files ...string) error {
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			return err
		}
	}
	return nil
}

// createAll attempts to create all given files with "" as contents.
func createAll(files ...string) error {
	for _, f := range files {
		if err := ioutil.WriteFile(f, nil, 0644); err != nil {
			return err
		}
	}
	return nil
}

// checkNonExistent checks that all specified files do not exist.
func checkNonExistent(files ...string) error {
	for _, f := range files {
		if _, err := os.Stat(f); err == nil {
			return errors.Errorf("%s not deleted", f)
		} else if !os.IsNotExist(err) {
			return errors.Wrapf(err, "unexpected error stating %s", f)
		}
	}
	return nil
}

func TestSetUpAndTearDownCrashTest(t *testing.T) {
	// We can't use the normal file location /run/crash_reporter; we don't have
	// permission to write there. Instead write to a location under /tmp.
	runDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(runDir)

	// Create and populate spool and stash directories.
	sysCrashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(sysCrashDir)

	sysStashDir := sysCrashDir + ".stash"

	userCrashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(userCrashDir)

	userStashDir := userCrashDir + ".stash"

	if err := createAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log")); err != nil {
		t.Fatalf("createAll: %v", err)
	}

	if err := setUpCrashTestWithDirectories(runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir, false); err != nil {
		t.Fatalf("setUpCrashTestWithDirectories(%s, %s, %s, %s, %s, false): %v", runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir, err)
	}

	if err := statAll(sysStashDir, userStashDir); err != nil {
		t.Fatal("stash dirs not created: ", err)
	}

	// All pre-existing files should be in the stash.
	if err := statAll(filepath.Join(sysStashDir, "sysCrash.log"),
		filepath.Join(userStashDir, "userCrash.log")); err != nil {
		t.Fatal("files not all in correct location: ", err)
	}

	file := filepath.Join(runDir, "crash-test-in-progress")
	if _, err := os.Stat(file); err != nil {
		t.Errorf("Cannot stat %s: %v", file, err)
	}

	// Create new files - these should be preserved
	if err := createAll(filepath.Join(userCrashDir, "newUserCrash.log"),
		filepath.Join(sysCrashDir, "newSysCrash.log")); err != nil {
		t.Fatal("createAll: ", err)
	}

	if err := tearDownCrashTestWithDirectories(runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir); err != nil {
		t.Errorf("tearDownCrashTestWithDirectories(%s, %s, %s, %s, %s, false): %v", runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir, err)
	}

	// Ensure that all crash files are in spool directories.
	if err := statAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(sysCrashDir, "newSysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log"),
		filepath.Join(userCrashDir, "newUserCrash.log")); err != nil {
		t.Error("statAll: ", err)
	}

	if err := checkNonExistent(file, sysStashDir, userStashDir); err != nil {
		t.Errorf("checkNonExistent(%s, %s, %s): %v", file, sysStashDir, userStashDir, err)
	}
}

// TestSetUpAndTearDownCrashTestWithOldStash ensures that the crash_reporter
// tests preserve files in an old stash directory that was not cleaned up.
func TestSetUpAndTearDownCrashTestWithOldStash(t *testing.T) {
	// We can't use the normal file location /run/crash_reporter; we don't have
	// permission to write there. Instead write to a location under /tmp.
	runDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTestWithOldStash")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(runDir)

	// Create and populate spool and stash directories.
	sysCrashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTestWithOldStash")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(sysCrashDir)

	sysStashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTestWithOldStash")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(sysStashDir)

	userCrashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTestWithOldStash")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(userCrashDir)

	userStashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTestWithOldStash")
	defer os.RemoveAll(userStashDir)
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}

	if err := createAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(sysStashDir, "oldSysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log"),
		filepath.Join(userStashDir, "oldUserCrash.log")); err != nil {
		t.Fatalf("createAll: %v", err)
	}

	if err := setUpCrashTestWithDirectories(runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir, false); err != nil {
		t.Fatalf("setUpCrashTestWithDirectories(%s, %s, %s, %s, %s): %v", runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir, err)
	}

	// All pre-existing files should be in the stash.
	if err := statAll(filepath.Join(sysStashDir, "sysCrash.log"),
		filepath.Join(sysStashDir, "oldSysCrash.log"),
		filepath.Join(userStashDir, "userCrash.log"),
		filepath.Join(userStashDir, "oldUserCrash.log")); err != nil {
		t.Errorf("files not all in correct location: %v", err)
	}

	if err := tearDownCrashTestWithDirectories(runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir); err != nil {
		t.Errorf("tearDownCrashTestWithDirectories(%s, %s, %s, %s, %s): %v", runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir, err)
	}

	// Ensure that all crash files are in spool directories.
	if err := statAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(sysCrashDir, "oldSysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log"),
		filepath.Join(userCrashDir, "oldUserCrash.log")); err != nil {
		t.Errorf("statAll: %v", err)
	}

	// Verify that stash dirs were deleted.
	if err := checkNonExistent(sysStashDir, userStashDir); err != nil {
		t.Errorf("checkNonExistent(%s, %s): %v", sysStashDir, userStashDir, err)
	}
}

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
