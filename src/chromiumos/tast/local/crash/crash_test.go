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
	sysCrashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	sysStashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	userCrashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	userStashDir, err := ioutil.TempDir("", "TestSetUpAndTearDownCrashTest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}

	if err := setUpCrashTestWithDirectories(runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir); err != nil {
		t.Fatalf("setUpCrashTestWithDirectories(%s, %s, %s, %s, %s): %v", runDir, sysCrashDir, sysStashDir, userCrashDir, userStashDir, err)
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
