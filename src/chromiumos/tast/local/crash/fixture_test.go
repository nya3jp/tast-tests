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
	"strings"
	"syscall"
	"testing"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
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
	if err := mkdirAll(runDir, sysCrashDir, sysCrashStash, userCrashDir, userCrashStash); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	if err := createAll(filepath.Join(sysCrashDir, "sysCrash.log"),
		filepath.Join(userCrashDir, "userCrash.log")); err != nil {
		t.Fatalf("createAll: %v", err)
	}

	// Start a fake crash_sender process.
	// Process name length must be up to TASK_COMM_LEN (16).
	procName := fmt.Sprintf("test_%d", rand.Int31())
	shell := fmt.Sprintf("export; echo -n %s > /proc/self/comm; sleep 5", procName)
	cmd := exec.Command("sh", "-c", shell)
	cmd.Env = append(os.Environ(), "LD_PRELOAD=") // workaround sandbox failures on running in Portage
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start fake crash_sender process: %v", err)
	}

	// Start a goroutine to reap the subprocess. This must be done concurrently
	// with pkill because pkill waits for signaled processes to exit.
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := cmd.Wait()
		if ws, ok := testexec.GetWaitStatus(err); !ok || !ws.Signaled() || ws.Signal() != syscall.SIGKILL {
			t.Errorf("Fake crash_sender process was not killed: %v", err)
		}
	}()
	defer func() { <-done }()

	// Wait for the process name change to take effect.
	for {
		// We don't use gopsutil to get the process name here because it somehow
		// caches the process name until it exits.
		b, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/comm", cmd.Process.Pid))
		if err != nil {
			t.Fatalf("Failed to get name of fake crash_sender process: %v", err)
		}
		name := strings.TrimRight(string(b), "\n")
		if name == procName {
			break
		}
	}

	sp := setUpParams{
		inProgDir:       runDir,
		sysCrashDir:     sysCrashDir,
		sysCrashStash:   sysCrashStash,
		userCrashDir:    userCrashDir,
		userCrashStash:  userCrashStash,
		senderPausePath: pausePath,
		senderProcName:  procName,
		isDevImageTest:  false,
		setConsent:      false,
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
		inProgDir:       runDir,
		sysCrashDir:     sysCrashDir,
		sysCrashStash:   sysCrashStash,
		userCrashDir:    userCrashDir,
		userCrashStash:  userCrashStash,
		senderPausePath: pausePath,
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

	if err := checkNonExistent(inProgFile, pausePath, sysCrashStash, userCrashStash); err != nil {
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
	pausePath := filepath.Join(tmpDir, "pause")
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
		inProgDir:       runDir,
		sysCrashDir:     sysCrashDir,
		sysCrashStash:   sysCrashStash,
		userCrashDir:    userCrashDir,
		userCrashStash:  userCrashStash,
		senderPausePath: pausePath,
		senderProcName:  "crash_sender.fake",
		isDevImageTest:  false,
		setConsent:      false,
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
		inProgDir:       runDir,
		sysCrashDir:     sysCrashDir,
		sysCrashStash:   sysCrashStash,
		userCrashDir:    userCrashDir,
		userCrashStash:  userCrashStash,
		senderPausePath: pausePath,
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
