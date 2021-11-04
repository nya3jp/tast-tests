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
	pausePath := filepath.Join(tmpDir, "pause")
	mockSendingPath := filepath.Join(tmpDir, "mock-sending")
	mockConsentPath := filepath.Join(runDir, "mock-consent")
	remoteMockConsentPath := filepath.Join(sysCrashDir, "mock-consent")
	remoteInProgPath := filepath.Join(sysCrashDir, "crash-test-in-progress")
	sendDir := filepath.Join(tmpDir, "send")
	if err := mkdirAll(runDir, sysCrashDir, sysCrashStash, userCrashDir, userCrashStash, sendDir); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	if err := createAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log"),
		filepath.Join(sendDir, ".send.1")); err != nil {
		t.Fatalf("createAll: %v", err)
	}

	sp := setUpParams{
		inProgDir: runDir,
		crashDirs: []crashAndStash{
			{
				crashDir: sysCrashDir,
				stashDir: sysCrashStash,
			},
			{
				crashDir: userCrashDir,
				stashDir: userCrashStash,
			},
		},
		rebootPersistDir: sysCrashDir,
		senderPausePath:  pausePath,
		senderProcName:   "crash_sender.fake",
		mockSendingPath:  mockSendingPath,
		sendRecordDir:    sendDir,
		isDevImageTest:   false,
		setConsent:       false,
		setMockConsent:   true,
		rebootTest:       false,
	}
	if err := setUpCrashTest(context.Background(), &sp); err != nil {
		t.Fatalf("setUpCrashTest(%#v): %v", sp, err)
	}

	if err := statAll(sysCrashStash, userCrashStash); err != nil {
		t.Fatal("stash dirs not created: ", err)
	}

	if err := statAll(mockSendingPath); err != nil {
		t.Error("Mock sending was not enabled: ", err)
	}
	if err := statAll(mockConsentPath); err != nil {
		t.Error("Mock consent was not enabled: ", err)
	}

	if err := checkNonExistent(remoteMockConsentPath, remoteInProgPath); err != nil {
		t.Error("Reboot-persistance files unexpectedly created: ", err)
	}
	if err := checkNonExistent(filepath.Join(sendDir, ".send.1")); err != nil {
		t.Error("Send reports were not cleared: ", err)
	}

	// All pre-existing files should be in the stash.
	if err := statAll(filepath.Join(sysCrashStash, "sysCrash.log"),
		filepath.Join(userCrashStash, "userCrash.log")); err != nil {
		t.Fatal("files not all in correct location: ", err)
	}

	inProgFile := filepath.Join(runDir, "crash-test-in-progress")
	if err := statAll(inProgFile, pausePath); err != nil {
		t.Errorf("statAll: %v", err)
	}

	// Create new files - these should be preserved
	if err := createAll(filepath.Join(userCrashDir, "newUserCrash.log"),
		filepath.Join(sysCrashDir, "newSysCrash.log")); err != nil {
		t.Fatal("createAll: ", err)
	}

	tp := tearDownParams{
		inProgDir: runDir,
		crashDirs: []crashAndStash{
			{
				crashDir: sysCrashDir,
				stashDir: sysCrashStash,
			},
			{
				crashDir: userCrashDir,
				stashDir: userCrashStash,
			},
		},
		rebootPersistDir: sysCrashDir,
		senderPausePath:  pausePath,
		mockSendingPath:  mockSendingPath,
	}
	if err := tearDownCrashTest(context.Background(), &tp); err != nil {
		t.Errorf("tearDownCrashTest(%#v): %v", tp, err)
	}

	// Ensure that all crash files are in spool directories.
	if err := statAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(sysCrashDir, "newSysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log"),
		filepath.Join(userCrashDir, "newUserCrash.log")); err != nil {
		t.Error("statAll: ", err)
	}

	if err := checkNonExistent(inProgFile, pausePath, sysCrashStash, userCrashStash, mockSendingPath, mockConsentPath); err != nil {
		t.Errorf("checkNonExistent: %v", err)
	}
}

func TestSetUpAndTearDownReboot(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	defer os.RemoveAll(tmpDir)

	// We can't use the normal file location /run/crash_reporter; we don't have
	// permission to write there. Instead write to a location under /tmp.
	runDir := filepath.Join(tmpDir, "run")
	sysCrashDir := filepath.Join(tmpDir, "sys_crash")
	sysCrashStash := filepath.Join(tmpDir, "sys_crash.stash")
	userCrashDir := filepath.Join(tmpDir, "user_crash")
	userCrashStash := filepath.Join(tmpDir, "user_crash.stash")
	pausePath := filepath.Join(tmpDir, "pause")
	mockSendingPath := filepath.Join(tmpDir, "mock-sending")
	remoteMockConsentPath := filepath.Join(sysCrashDir, "mock-consent")
	remoteInProgPath := filepath.Join(sysCrashDir, "crash-test-in-progress")
	sendDir := filepath.Join(tmpDir, "send")

	// Explicitly _don't_ make runDir -- SetUpCrashTest should make it.
	if err := mkdirAll(sysCrashDir, sysCrashStash, userCrashDir, userCrashStash, sendDir); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	sp := setUpParams{
		inProgDir: runDir,
		crashDirs: []crashAndStash{
			{
				crashDir: sysCrashDir,
				stashDir: sysCrashStash,
			},
			{
				crashDir: userCrashDir,
				stashDir: userCrashStash,
			},
		},
		rebootPersistDir: sysCrashDir,
		senderPausePath:  pausePath,
		mockSendingPath:  mockSendingPath,
		sendRecordDir:    sendDir,
		isDevImageTest:   false,
		setConsent:       false,
		setMockConsent:   true,
		rebootTest:       true,
	}
	if err := setUpCrashTest(context.Background(), &sp); err != nil {
		t.Fatalf("setUpCrashTest(%#v): %v", sp, err)
	}
	if err := statAll(remoteMockConsentPath, remoteInProgPath); err != nil {
		t.Error("Reboot-persistence files were not created: ", err)
	}

	tp := tearDownParams{
		inProgDir: runDir,
		crashDirs: []crashAndStash{
			{
				crashDir: sysCrashDir,
				stashDir: sysCrashStash,
			},
			{
				crashDir: userCrashDir,
				stashDir: userCrashStash,
			},
		},
		rebootPersistDir: sysCrashDir,
		senderPausePath:  pausePath,
		mockSendingPath:  mockSendingPath,
	}

	if err := tearDownCrashTest(context.Background(), &tp); err != nil {
		t.Errorf("tearDownCrashTest(%#v): %v", tp, err)
	}
	if err := checkNonExistent(remoteMockConsentPath, remoteInProgPath); err != nil {
		t.Error("Reboot-persistance files not removed: ", err)
	}

	sp.setMockConsent = false
	if err := setUpCrashTest(context.Background(), &sp); err != nil {
		t.Fatalf("setUpCrashTest(%#v): %v", sp, err)
	}
	if err := statAll(remoteInProgPath); err != nil {
		t.Error("Reboot-persistence in-prog file was not created: ", err)
	}
	if err := checkNonExistent(remoteMockConsentPath); err != nil {
		t.Error("Reboot-persistance mock-consent file unexpectedly created: ", err)
	}
	if err := tearDownCrashTest(context.Background(), &tp); err != nil {
		t.Errorf("tearDownCrashTest(%#v): %v", tp, err)
	}
	if err := checkNonExistent(remoteMockConsentPath, remoteInProgPath); err != nil {
		t.Error("Reboot-persistance files not removed: ", err)
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
	pausePath := filepath.Join(tmpDir, "pause")
	mockSendingPath := filepath.Join(tmpDir, "mock-sending")
	mockConsentPath := filepath.Join(runDir, "mock-consent")
	sendDir := filepath.Join(tmpDir, "send")
	if err := mkdirAll(runDir, sysCrashDir, sysCrashStash, userCrashDir, userCrashStash, sendDir); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	if err := createAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(sysCrashStash, "oldSysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log"),
		filepath.Join(userCrashStash, "oldUserCrash.log")); err != nil {
		t.Fatalf("createAll: %v", err)
	}

	sp := setUpParams{
		inProgDir: runDir,
		crashDirs: []crashAndStash{
			{
				crashDir: sysCrashDir,
				stashDir: sysCrashStash,
			},
			{
				crashDir: userCrashDir,
				stashDir: userCrashStash,
			},
		},
		rebootPersistDir: sysCrashDir,
		senderPausePath:  pausePath,
		senderProcName:   "crash_sender.fake",
		mockSendingPath:  mockSendingPath,
		isDevImageTest:   false,
		setConsent:       false,
		setMockConsent:   false,
	}
	if err := setUpCrashTest(context.Background(), &sp); err != nil {
		t.Fatalf("setUpCrashTest(%#v): %v", sp, err)
	}
	// setMockConsent is false, so mockConsentPath should not be created.
	if err := checkNonExistent(mockConsentPath); err != nil {
		t.Errorf("checkNonExistent: %v", err)
	}

	// All pre-existing files should be in the stash.
	if err := statAll(filepath.Join(sysCrashStash, "sysCrash.log"),
		filepath.Join(sysCrashStash, "oldSysCrash.log"),
		filepath.Join(userCrashStash, "userCrash.log"),
		filepath.Join(userCrashStash, "oldUserCrash.log")); err != nil {
		t.Errorf("files not all in correct location: %v", err)
	}

	tp := tearDownParams{
		inProgDir: runDir,
		crashDirs: []crashAndStash{
			{
				crashDir: sysCrashDir,
				stashDir: sysCrashStash,
			},
			{
				crashDir: userCrashDir,
				stashDir: userCrashStash,
			},
		},
		rebootPersistDir: sysCrashDir,
		senderPausePath:  pausePath,
		mockSendingPath:  mockSendingPath,
	}
	if err := tearDownCrashTest(context.Background(), &tp); err != nil {
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

func TestMoveAllCrashesTo(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	defer os.RemoveAll(tmpDir)
	nonExistent := filepath.Join(tmpDir, "non_existent")
	dst1 := filepath.Join(tmpDir, "dstdir1")
	if err := moveAllCrashesTo(nonExistent, dst1); !os.IsNotExist(err) {
		t.Fatalf("moveAllCrashesTo: %v", err)
	}
	if err := checkNonExistent(dst1); err != nil {
		t.Fatalf("%s should not be created when source doesn't exist", dst1)
	}

	src := filepath.Join(tmpDir, "srcdir")
	srcSub := filepath.Join(src, "subdir")
	dst2 := filepath.Join(tmpDir, "dstdir2")
	if err := mkdirAll(src, srcSub); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	if err := createAll(filepath.Join(src, "foo.log"),
		filepath.Join(srcSub, "subFoo.log")); err != nil {
		t.Fatalf("createAll: %v", err)
	}

	if err := moveAllCrashesTo(src, dst2); err != nil {
		t.Fatalf("moveAllCrashesTo: %v", err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("%s should still exist", src)
	}
	// Verify that foo.log was moved and that srcSub (and its contents)
	// were not.
	if err := statAll(dst2, filepath.Join(dst2, "foo.log"),
		srcSub, filepath.Join(srcSub, "subFoo.log")); err != nil {
		t.Fatalf("statAll: %s", err)
	}
	if err := checkNonExistent(filepath.Join(dst2, "subdir"),
		filepath.Join(src, "foo.log")); err != nil {
		t.Fatalf("checkNonExistent: %s", err)
	}
}

func TestFilterIn(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	defer os.RemoveAll(tmpDir)

	// We can't use the normal file location /run/crash_reporter; we don't have
	// permission to write there. Instead write to a location under /tmp.
	runDir := filepath.Join(tmpDir, "run")
	pausePath := filepath.Join(tmpDir, "pause")
	filterInPath := filepath.Join(runDir, "filter-in")
	mockSendingPath := filepath.Join(tmpDir, "mock-sending")
	sendDir := filepath.Join(tmpDir, "send")
	if err := mkdirAll(runDir, sendDir); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}
	sp := setUpParams{
		inProgDir:       runDir,
		crashDirs:       []crashAndStash{},
		senderPausePath: pausePath,
		mockSendingPath: mockSendingPath,
		sendRecordDir:   sendDir,
		filterInPath:    filterInPath,
		filterIn:        "a command filter",
	}
	if err := setUpCrashTest(context.Background(), &sp); err != nil {
		t.Fatalf("setUpCrashTest(%#v): %v", sp, err)
	}

	// Verify filter in file was created with right contents.
	contents, err := ioutil.ReadFile(filterInPath)
	if err != nil {
		t.Fatalf("Could not read filter in file %q: %v", filterInPath, err)
	}
	if string(contents) != sp.filterIn {
		t.Fatalf("Incorrect filter in contents: got %q but want %q", string(contents), sp.filterIn)
	}

	tp := tearDownParams{
		inProgDir:       runDir,
		crashDirs:       []crashAndStash{},
		senderPausePath: pausePath,
		filterInPath:    filterInPath,
		mockSendingPath: mockSendingPath,
	}
	if err := tearDownCrashTest(context.Background(), &tp); err != nil {
		t.Errorf("tearDownCrashTest(%#v): %v", tp, err)
	}

	if err := checkNonExistent(filterInPath); err != nil {
		t.Errorf("filter in file not deleted: %v", err)
	}

	// Verify that the file is deleted if the test requests no filtering.
	// (the default)
	if err := ioutil.WriteFile(filterInPath, []byte("asdf"), 0664); err != nil {
		t.Fatalf("Error creating fake filterIn file: %v", err)
	}
	sp.filterIn = ""

	if err := setUpCrashTest(context.Background(), &sp); err != nil {
		t.Fatalf("setUpCrashTest(%#v): %v", sp, err)
	}
	if err := checkNonExistent(filterInPath); err != nil {
		t.Errorf("filter in file not deleted: %v", err)
	}
}
