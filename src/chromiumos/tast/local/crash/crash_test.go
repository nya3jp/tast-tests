// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

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
	if err := ioutil.WriteFile(filepath.Join(sysCrashDir, "sysCrash.log"), []byte(""), 0744); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}
	sysStashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(sysStashDir)
	if err := ioutil.WriteFile(filepath.Join(sysStashDir, "oldSysCrash.log"), []byte(""), 0744); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	userCrashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(userCrashDir)
	if err := ioutil.WriteFile(filepath.Join(userCrashDir, "userCrash.log"), []byte(""), 0744); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}
	userStashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTest")
	defer os.RemoveAll(userStashDir)
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(userStashDir, "oldUserCrash.log"), []byte(""), 0744); err != nil {
		t.Fatalf("ioutil.WriteFile: %v", err)
	}

	if err := setUpCrashTestWithDirectories(runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir); err != nil {
		t.Fatalf("setUpCrashTestWithDirectories(%s, %s, %s, %s, %s): %v", runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir, err)
	}

	if _, err := ioutil.ReadFile(filepath.Join(sysStashDir, "sysCrash.log")); err != nil {
		t.Errorf("Didn't move sysCrash to stash: %v", err)
	}
	if _, err := ioutil.ReadFile(filepath.Join(sysStashDir, "oldSysCrash.log")); err != nil {
		t.Errorf("Didn't preserve oldSysCrash in stash: %v", err)
	}

	if _, err := ioutil.ReadFile(filepath.Join(userStashDir, "userCrash.log")); err != nil {
		t.Errorf("Didn't move userCrash to stash: %v", err)
	}
	if _, err := ioutil.ReadFile(filepath.Join(userStashDir, "oldUserCrash.log")); err != nil {
		t.Errorf("Didn't preserve oldUserCrash in stash: %v", err)
	}

	file := filepath.Join(runDir, "crash-test-in-progress")
	if _, err := os.Stat(file); err != nil {
		t.Errorf("Cannot stat %s: %v", file, err)
	}

	if err := tearDownCrashTestWithDirectories(runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir); err != nil {
		t.Errorf("tearDownCrashTestWithDirectories(%s, %s, %s, %s, %s): %v", runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir, err)
	}

	if _, err := os.Stat(file); err == nil {
		t.Errorf("%s not deleted", file)
	} else if !os.IsNotExist(err) {
		t.Errorf("Unexpected error stating %s: %v", file, err)
	}
}
