// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
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
	sysCrashDir := filepath.Join(tmpDir, "sys_crash")
	sysCrashStash := filepath.Join(tmpDir, "sys_crash.stash")
	userCrashDir := filepath.Join(tmpDir, "user_crash")
	userCrashStash := filepath.Join(tmpDir, "user_crash.stash")
	if err := mkdirAll(runDir, sysCrashDir, sysCrashStash, userCrashDir, userCrashStash); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	if err := createAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log")); err != nil {
		t.Fatalf("createAll: %v", err)
	}

	sp := setUpParams{
		inProgDir:      runDir,
		sysCrashDir:    sysCrashDir,
		sysCrashStash:  sysCrashStash,
		userCrashDir:   userCrashDir,
		userCrashStash: userCrashStash,
		isDevImageTest: false,
		setConsent:     false,
	}
	if err := setUpCrashTest(context.Background(), &sp); err != nil {
		t.Fatalf("setUpCrashTest(%#v): %v", sp, err)
	}

	if err := statAll(sysCrashStash, userCrashStash); err != nil {
		t.Fatal("stash dirs not created: ", err)
	}

	// All pre-existing files should be in the stash.
	if err := statAll(filepath.Join(sysCrashStash, "sysCrash.log"),
		filepath.Join(userCrashStash, "userCrash.log")); err != nil {
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

	tp := tearDownParams{
		inProgDir:      runDir,
		sysCrashDir:    sysCrashDir,
		sysCrashStash:  sysCrashStash,
		userCrashDir:   userCrashDir,
		userCrashStash: userCrashStash,
	}
	if err := tearDownCrashTest(&tp); err != nil {
		t.Errorf("tearDownCrashTest(%#v): %v", tp, err)
	}

	// Ensure that all crash files are in spool directories.
	if err := statAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(sysCrashDir, "newSysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log"),
		filepath.Join(userCrashDir, "newUserCrash.log")); err != nil {
		t.Error("statAll: ", err)
	}

	if err := checkNonExistent(file, sysCrashStash, userCrashStash); err != nil {
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
	sysCrashDir := filepath.Join(tmpDir, "sys_crash")
	sysCrashStash := filepath.Join(tmpDir, "sys_crash.stash")
	userCrashDir := filepath.Join(tmpDir, "user_crash")
	userCrashStash := filepath.Join(tmpDir, "user_crash.stash")
	if err := mkdirAll(runDir, sysCrashDir, sysCrashStash, userCrashDir, userCrashStash); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	if err := createAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(sysCrashStash, "oldSysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log"),
		filepath.Join(userCrashStash, "oldUserCrash.log")); err != nil {
		t.Fatalf("createAll: %v", err)
	}

	sp := setUpParams{
		inProgDir:      runDir,
		sysCrashDir:    sysCrashDir,
		sysCrashStash:  sysCrashStash,
		userCrashDir:   userCrashDir,
		userCrashStash: userCrashStash,
		isDevImageTest: false,
		setConsent:     false,
	}
	if err := setUpCrashTest(context.Background(), &sp); err != nil {
		t.Fatalf("setUpCrashTest(%#v): %v", sp, err)
	}

	// All pre-existing files should be in the stash.
	if err := statAll(filepath.Join(sysCrashStash, "sysCrash.log"),
		filepath.Join(sysCrashStash, "oldSysCrash.log"),
		filepath.Join(userCrashStash, "userCrash.log"),
		filepath.Join(userCrashStash, "oldUserCrash.log")); err != nil {
		t.Errorf("files not all in correct location: %v", err)
	}

	tp := tearDownParams{
		inProgDir:      runDir,
		sysCrashDir:    sysCrashDir,
		sysCrashStash:  sysCrashStash,
		userCrashDir:   userCrashDir,
		userCrashStash: userCrashStash,
	}
	if err := tearDownCrashTest(&tp); err != nil {
		t.Errorf("tearDownCrashTest(%#v): %v", tp, err)
	}

	// Ensure that all crash files are in spool directories.
	if err := statAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(sysCrashDir, "oldSysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log"),
		filepath.Join(userCrashDir, "oldUserCrash.log")); err != nil {
		t.Errorf("statAll: %v", err)
	}

	// Verify that stash dirs were deleted.
	if err := checkNonExistent(sysCrashStash, userCrashStash); err != nil {
		t.Errorf("checkNonExistent: %v", err)
	}
}
