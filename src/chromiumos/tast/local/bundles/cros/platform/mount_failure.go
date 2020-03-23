// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MountFailure,
		Desc:     "Verify mount and umount failures are logged as expected",
		Contacts: []string{"sarthakkukreti@google.com", "cros-monitoring-forensics@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

type mountFailureCrashLog struct {
	LogPath     string
	LogContents string
	LogHeader   string
}

type mountFailureCrash struct {
	CrashName   string
	CmdlineArgs []string
	LogCommands []string
}

type logCommandMap map[string]mountFailureCrashLog

// Description of logs in /run that are collected by the mount failure collector.
var mountFailureLogMap = logCommandMap{
	"shutdown_umount_failure_state": {"/run/shutdown_umount_failure.log", "log_shutdown_umount_failure", "===shutdown umount() failure logs==="},
	"dumpe2fs_stateful":             {"/run/dumpe2fs_stateful.log", "log_dumpe2fs_stateful", "===dumpe2fs (stateful partition)==="},
	"dumpe2fs_encstateful":          {"/run/mount_encrypted/dumpe2fs.log", "log_dumpe2fs_encstateful", "===dumpe2fs (/dev/mapper/encstateful)==="},
	"mount-encrypted":               {"/run/mount_encrypted/mount-encrypted.log", "log_mount-encrypted", "===mount-encrypted==="},
	"umount-encrypted":              {"/run/mount_encrypted/umount-encrypted.log", "log_umount-encrypted", "===umount-encrypted==="},
	// Ramoops and dmesg are collected directly: check if the header is present in the file.
	"console-ramoops": {"", "", "===ramoops==="},
	"kernel-warning":  {"", "", "===dmesg==="},
}

var mountFailures = []mountFailureCrash{
	{"mount_failure_stateful", []string{"--mount_failure", "--mount_device=stateful"}, []string{"console-ramoops", "kernel-warning", "dumpe2fs_stateful"}},
	{"mount_failure_encstateful", []string{"--mount_failure", "--mount_device=encstateful"}, []string{"console-ramoops", "kernel-warning", "dumpe2fs_encstateful", "mount-encrypted"}},
	{"umount_failure_stateful", []string{"--umount_failure", "--mount_device=stateful"}, []string{"shutdown_umount_failure_state", "umount-encrypted"}},
}

func saveMountFailureLogs() error {
	for _, mountFailureLog := range mountFailureLogMap {
		if mountFailureLog.LogPath == "" {
			continue
		}
		if err := ioutil.WriteFile(mountFailureLog.LogPath, []byte(mountFailureLog.LogContents), 0644); err != nil {
			return errors.Wrapf(err, "failed to write %s", mountFailureLog.LogPath)
		}
	}
	return nil
}

func cleanupMountFailureLogs() error {
	for _, mountFailureLog := range mountFailureLogMap {
		if mountFailureLog.LogPath == "" {
			continue
		}
		if err := os.Remove(mountFailureLog.LogPath); err != nil {
			return errors.Wrapf(err, "failed to remove %s", mountFailureLog.LogPath)
		}
	}
	return nil
}

func runMountFailureCollectors(ctx context.Context) error {
	for _, mountFailure := range mountFailures {
		cmd := testexec.CommandContext(ctx, "/sbin/crash_reporter", mountFailure.CmdlineArgs...)
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "crash_reporter command failed: %s", mountFailure.CrashName)
		}
	}
	return nil
}

func generateExpectedFilesRegexes() []string {
	var expectedFilesRegex []string
	for _, mountFailure := range mountFailures {
		expectedFilesRegex = append(expectedFilesRegex, mountFailure.CrashName+`\.\d{8}\.\d{6}\.0\.log`, mountFailure.CrashName+`\.\d{8}\.\d{6}\.0\.meta`)
	}
	return expectedFilesRegex
}

func validateCrashContents(files map[string][]string) error {
	for _, mountFailure := range mountFailures {
		logFileRegex := mountFailure.CrashName + `\.\d{8}\.\d{6}\.0\.log`

		if len(files[logFileRegex]) != 1 {
			return errors.Errorf("multiple crash files within the same regex bucket: %s: %d", mountFailure.CrashName, len(files[logFileRegex]))
		}

		f := files[logFileRegex][0]
		contents, err := ioutil.ReadFile(f)
		if err != nil {
			return errors.Wrapf(err, "couldn't read log file %s: %v", f, err)
		}

		for _, cmd := range mountFailure.LogCommands {
			if !strings.Contains(string(contents), mountFailureLogMap[cmd].LogHeader) {
				return errors.Errorf("header not found: %s", mountFailureLogMap[cmd].LogHeader)
			}
			if !strings.Contains(string(contents), mountFailureLogMap[cmd].LogContents) {
				return errors.Errorf("contents not found: %s", mountFailureLogMap[cmd].LogContents)
			}
		}
	}

	return nil
}

func MountFailure(ctx context.Context, s *testing.State) {
	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent()); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}

	// Teardown on exiting the test.
	defer crash.TearDownCrashTest()

	if err := saveMountFailureLogs(); err != nil {
		s.Fatal("Failed to set up debug logs for mount failure collector: ", err)
	}

	// Cleanup logs on exit.
	defer cleanupMountFailureLogs()

	// Run crash_reporter to generate the crashes.
	if err := runMountFailureCollectors(ctx); err != nil {
		s.Fatal("Failed to run mount failure collectors: ", err)
	}

	s.Log("Waiting for crash files")

	expectedRegexes := generateExpectedFilesRegexes()

	if len(expectedRegexes) == 0 {
		s.Fatal("No regexes to test against")
	}

	files, err := crash.WaitForCrashFiles(ctx, []string{crash.SystemCrashDir}, []string{}, expectedRegexes)
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}

	if err := validateCrashContents(files); err != nil {
		s.Fatal("Failed to validate contents of the crash reporters: ", err)
	}
}
