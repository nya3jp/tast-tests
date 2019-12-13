// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"io/ioutil"
	"os"
	"path/filepath"
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

// mkdirAll attempts to mkdir all given directories.
func mkdirAll(files ...string) error {
	for _, f := range files {
		if err := os.MkdirAll(f, 0755); err != nil {
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
	tmpDir := testutil.TempDir(t)
	defer os.RemoveAll(tmpDir)

	// We can't use the normal file location /run/crash_reporter; we don't have
	// permission to write there. Instead write to a location under /tmp.
	runDir := filepath.Join(tmpDir, "run")
	pausePath := filepath.Join(tmpDir, "paused")
	sysCrashDir := filepath.Join(tmpDir, "sys_crash")
	sysStashDir := filepath.Join(tmpDir, "sys_crash.stash")
	userCrashDir := filepath.Join(tmpDir, "user_crash")
	userStashDir := filepath.Join(tmpDir, "user_crash.stash")
	if err := mkdirAll(runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	if err := createAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log")); err != nil {
		t.Fatalf("createAll: %v", err)
	}

	if err := setUpCrashTestWithDirectories(runDir, pausePath, sysCrashDir, sysStashDir, userCrashDir, userStashDir, false); err != nil {
		t.Fatalf("setUpCrashTestWithDirectories: %v", err)
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

	if _, err := os.Stat(pausePath); err != nil {
		t.Errorf("Pause file not created: %v", err)
	}

	// Create new files - these should be preserved
	if err := createAll(filepath.Join(userCrashDir, "newUserCrash.log"),
		filepath.Join(sysCrashDir, "newSysCrash.log")); err != nil {
		t.Fatal("createAll: ", err)
	}

	if err := tearDownCrashTestWithDirectories(runDir, pausePath, sysCrashDir, sysStashDir, userCrashDir, userStashDir); err != nil {
		t.Errorf("tearDownCrashTestWithDirectories: %v", err)
	}

	// Ensure that all crash files are in spool directories.
	if err := statAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(sysCrashDir, "newSysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log"),
		filepath.Join(userCrashDir, "newUserCrash.log")); err != nil {
		t.Error("statAll: ", err)
	}

	if err := checkNonExistent(file, pausePath, sysStashDir, userStashDir); err != nil {
		t.Errorf("checkNonExistent: %v", err)
	}
}

// TestSetUpAndTearDownCrashTestWithOldStash ensures that the crash_reporter
// tests preserve files in an old stash directory that was not cleaned up.
func TestSetUpAndTearDownCrashTestWithOldStash(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	defer os.RemoveAll(tmpDir)

	// We can't use the normal file location /run/crash_reporter; we don't have
	// permission to write there. Instead write to a location under /tmp.
	runDir := filepath.Join(tmpDir, "run")
	pausePath := filepath.Join(tmpDir, "paused")
	sysCrashDir := filepath.Join(tmpDir, "sys_crash")
	sysStashDir := filepath.Join(tmpDir, "sys_crash.stash")
	userCrashDir := filepath.Join(tmpDir, "user_crash")
	userStashDir := filepath.Join(tmpDir, "user_crash.stash")
	if err := mkdirAll(runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	if err := createAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(sysStashDir, "oldSysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log"),
		filepath.Join(userStashDir, "oldUserCrash.log")); err != nil {
		t.Fatalf("createAll: %v", err)
	}

	if err := setUpCrashTestWithDirectories(runDir, pausePath, sysCrashDir, sysStashDir, userCrashDir, userStashDir, false); err != nil {
		t.Fatalf("setUpCrashTestWithDirectories: %v", err)
	}

	// All pre-existing files should be in the stash.
	if err := statAll(filepath.Join(sysStashDir, "sysCrash.log"),
		filepath.Join(sysStashDir, "oldSysCrash.log"),
		filepath.Join(userStashDir, "userCrash.log"),
		filepath.Join(userStashDir, "oldUserCrash.log")); err != nil {
		t.Errorf("files not all in correct location: %v", err)
	}

	if err := tearDownCrashTestWithDirectories(runDir, pausePath, sysCrashDir, sysStashDir, userCrashDir, userStashDir); err != nil {
		t.Errorf("tearDownCrashTestWithDirectories: %v", err)
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
		t.Errorf("checkNonExistent: %v", err)
	}
}
