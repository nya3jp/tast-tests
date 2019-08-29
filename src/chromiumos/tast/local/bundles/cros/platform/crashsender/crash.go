// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crashsender

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// TODO(chavey) there are some dup with udev_crash.go, can extract and combine.
// TODO(chavey) it would be best to have a crashtest package that can be
//  be used as a lib for all the crash tests. see what makes more sense.
// TODO(chavey) If using a different package, some of those maybe needed by
// crash tests outside of this package.
const (
	consentFile             = "/home/chronos/Consent To Send Stats"
	corePattern             = "/proc/sys/kernel/core_pattern"
	crashReporterPath       = "/sbin/crash_reporter"
	crashRunStateDir        = "/run/crash_reporter"
	crashSenderPath         = "/sbin/crash_sender"
	crashSenderRateDir      = "/var/lib/crash_sender"
	crashTestInProgressFile = crashRunStateDir + "/crash-test-in-progress"
	fakeTestBasename        = "fake.1.2.3"
	fallbackUserCrashDir    = "/home/chronos/crash"
	mockCrashSending        = crashRunStateDir + "/mock-crash-sending"
	ownerKeyFile            = whiteListDir + "/owner.key"
	pauseFile               = "/var/lib/crash_sender_paused"
	signedPolicyFile        = whiteListDir + "/policy"
	systemCrashDir          = "/var/spool/crash"
	userCrashDirs           = "/home/chronos/u-*/crash"
	whiteListDir            = "/var/lib/whitelist"
)

// TODO(chavey) Those are similar to the ones used by udev_crash, need
// to merge. Right now we have to use this long name to isolate the collision
// with udev_crash in the same platform package.
const (
	MockMetricsOffPolicyFile = "crash_sender_mock_metrics_off_policy.bin"
	MockMetricsOnPolicyFile  = "crash_sender_mock_metrics_on_policy.bin"
	MockMetricsOwnerKeyFile  = "crash_sender_mock_metrics_owner.key"
)

const (
	// TODO(chavey) This whole business of push / pop is really not readable.
	// Need to refactor using a different naming.
	pushedConsentKey = 1
	pushedPolicyKey  = 2
	pushedOwnerKey   = 3
)

type crashRequest struct {
	// Mock a successful send if true.
	sendSuccess bool
	// Has the user consented to sending create reports.
	reportsEnabled bool
	// Report to use for crash, if nil create one.
	report string
	// expect the crash_sender program to fail.
	shouldFail bool
	// crash_sender should ignore the pause file existence.
	ignorePause bool
}

func fileIsDir(name string) bool {
	stat, err := os.Lstat(name)
	if err != nil {
		return false
	}
	return stat.Mode().IsDir()
}

func fileExist(name string) (bool, error) {
	_, err := os.Lstat(name)

	// On error, the file could be non existant or it could be a critical error.
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func removeIfExist(name string, all bool) error {
	if exist, err := fileExist(name); err != nil || !exist {
		return err
	}
	if all {
		return os.RemoveAll(name)
	}
	return os.Remove(name)
}

func pushed(key int) (string, string, error) {
	f := map[int]string{
		pushedConsentKey: consentFile,
		pushedPolicyKey:  signedPolicyFile,
		pushedOwnerKey:   ownerKeyFile,
	}
	if _, ok := f[key]; !ok {
		return "", "", errors.Errorf("no file found for %d", key)
	}

	p := map[int]string{
		pushedConsentKey: "pushed_consent",
		pushedPolicyKey:  "pushed_policy",
		pushedOwnerKey:   "pushed_owner_key",
	}

	if _, ok := p[key]; !ok {
		return "", "", errors.Errorf("no path found for %d", key)
	}
	return f[key], p[key], nil
}

func pushFilePath(key int) error {
	file, path, err := pushed(key)
	if err != nil {
		return errors.Wrap(err, "failed getting pushed file path")
	}
	if ok, _ := fileExist(file); ok {
		return fsutil.MoveFile(file, path)
	}
	return errors.Errorf("%q not found", file)
}

func popFilePath(key int) error {
	file, path, err := pushed(key)
	if err != nil {
		return errors.Wrap(err, "failed getting pushed file path")
	}
	if ok, _ := fileExist(path); ok {
		return fsutil.MoveFile(path, file)
	} else if ok, _ := fileExist(file); ok {
		return os.Remove(file)
	}
	return nil
}

// pushConsent pushes the consent file, thus disabling consent.
// The consent files can be created in the new test if required. Call
// popConsent() to restore the original state.
func pushConsent() error {
	if err := pushFilePath(pushedConsentKey); err != nil {
		return errors.Wrap(err, "failed pushing consent")
	}
	if err := pushFilePath(pushedPolicyKey); err != nil {
		return errors.Wrap(err, "failed pushing policy")
	}
	if err := pushFilePath(pushedOwnerKey); err != nil {
		return errors.Wrap(err, "failed pushing owner key")
	}
	return nil
}

// popConsent pops the consent files, enabling/disabling consent as it was before
// we pushed the consent.
func popConsent() error {
	if err := popFilePath(pushedConsentKey); err != nil {
		return errors.Wrap(err, "failed poping consent")
	}
	if err := popFilePath(pushedPolicyKey); err != nil {
		return errors.Wrap(err, "failed poping policy")
	}
	if err := popFilePath(pushedOwnerKey); err != nil {
		return errors.Wrap(err, "failed poping owner key")
	}
	return nil
}

// consent sets whether or not we have consent to send crash reports.
// This creates or deletes the _CONSENT_FILE to control whether
// crash_sender will consider that it has consent to send crash reports.
// It also copies a policy blob with the proper policy setting.
//
// autotest source contains the original mock_metrics files to use
// mock_metrics_owner.key @ //autotest/client/cros/mock_metrics_owner.key
// mock_metrics_on.policy @ //autotest/client/cros/mock_metrics_on.policy
// mock_metrics_off.policy @ //autotest/client/cros/mock_metrics_off.policy
//
// pushConsent() and popConsent the ability to manage consent file.
// pushConsent() disables consent. Consent can bee popped back later.
// Using push/pop makes it easier to manage nested tests.
// If _automatic_consent_saving is set, the consent ispushed at the start
// and popped at the end.
func consent(ctx context.Context, s *testing.State, reportsEnabled bool) error {
	if reportsEnabled {
		// create policy and own key files to enable metric/consent
		if info, err := os.Stat(whiteListDir); err == nil && info.IsDir() {
			src := s.DataPath(MockMetricsOnPolicyFile)
			if err := fsutil.CopyFile(src, signedPolicyFile); err != nil {
				return errors.Wrapf(err, "failed copying %s to %s", src, signedPolicyFile)
			}
			src = s.DataPath(MockMetricsOwnerKeyFile)
			if err := fsutil.CopyFile(src, ownerKeyFile); err != nil {
				return errors.Wrapf(err, "failed to copying %s to %s", src, ownerKeyFile)
			}
		}
		// Create deprecated consent file.  This is created *after* the
		// policy file in order to avoid a race condition where chrome
		// might remove the consent file if the policy's not set yet.
		// We create it as a temp file first in order to make the creation
		// of the consent file, owned by chronos, atomic.
		// See crosbug.com/18413.
		tempFile := fmt.Sprintf("%s.tmp", consentFile)
		if err := ioutil.WriteFile(tempFile, []byte("test-consent"), 0644); err != nil {
			return errors.Wrap(err, "failed setting test-content")
		}

		if err := os.Chown(tempFile, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
			return errors.Wrapf(err, "failed changing %s ownership", tempFile)
		}
		if err := fsutil.MoveFile(tempFile, consentFile); err != nil {
			return errors.Wrapf(err, "failed moving %s", tempFile)
		}
	} else {
		if info, err := os.Stat(whiteListDir); err == nil && info.IsDir() {
			src := s.DataPath(MockMetricsOffPolicyFile)
			if err := fsutil.CopyFile(src, signedPolicyFile); err != nil {
				return errors.Wrapf(err, "failed to create %s", signedPolicyFile)
			}
			src = s.DataPath(MockMetricsOwnerKeyFile)
			if err := fsutil.CopyFile(src, ownerKeyFile); err != nil {
				return errors.Wrapf(err, "failed to create %s", ownerKeyFile)
			}
		}
		if err := os.Remove(consentFile); err != nil {
			return errors.Wrapf(err, "failed removing %s", consentFile)
		}

	}
	return nil
}

// callSenderOneCrash calls the crash sender script to mock upload one crash.
func callSenderOneCrash(ctx context.Context, s *testing.State, req *crashRequest) error {
	var err error
	if req.report, err = prepareSenderOneCrash(ctx, s, req); err != nil {
		return errors.Wrap(err, "failed to prepare sender one crash")
	}

	var parm string
	if req.ignorePause {
		parm = "--ignore_pause_file"
	}
	if _, err = testexec.CommandContext(ctx, crashSenderPath, parm).Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed calling crash_sender")
	}

	_ = waitForSenderCompletion(ctx)

	req.report = ""

	return nil

}

// prepareSenderOneCrash creates metadata for a fake crash report.
// Enables mocking of the crash sender, then creates a fake crash report for testing purposes.
func prepareSenderOneCrash(ctx context.Context, s *testing.State, req *crashRequest) (string, error) {
	if err := sendingMock(true, req.sendSuccess); err != nil {
		return "", errors.Wrap(err, "failed sendingMock")
	}
	if err := consent(ctx, s, req.reportsEnabled); err != nil {
		return "", errors.Wrap(err, "failed consent")
	}

	if len(req.report) == 0 {
		// Use the same file format as crash does normally:
		// <basename>.#.#.#.meta
		var payload string
		var err error
		if payload, err = writeCrashDirEntry(fmt.Sprintf("%s.dmp", fakeTestBasename), ""); err != nil {
			return "", errors.Wrapf(err, "failed writing fake test crash dmp file %s", fakeTestBasename)
		}
		if req.report, err = writeFakeMeta(fmt.Sprintf("%s.meta", fakeTestBasename), "fake", payload, true); err != nil {
			return "", errors.Wrapf(err, "failed writing fake test crash meta file %", fakeTestBasename)
		}
	}
	return req.report, nil
}

// logReaderFind look at the system logs to determenine if crash_sender has ran.
func logReaderFind(ctx context.Context) error {
	return nil
}

// waitForSenderCompletion waits for crash_sender to complete.
// Wait for no crash_sender's last message to be placed in the
// system log before continuing and for the process to finish.
// Otherwise we might get only part of the output.
func waitForSenderCompletion(ctx context.Context) error {
	_ = testing.Poll(ctx, logReaderFind, &testing.PollOptions{Timeout: 60 * time.Second})
	return nil
}

func writeCrashDirEntry(filename string, content string) (string, error) {
	entry := path.Join(systemCrashDir, filename)
	if _, err := os.Stat(entry); os.IsNotExist(err) {
		if err = os.MkdirAll(systemCrashDir, 0644); err != nil {
			return "", errors.Wrapf(err, "failed to create %s", systemCrashDir)
		}
	}
	if err := ioutil.WriteFile(entry, []byte(content), 0644); err != nil {
		return "", errors.Wrap(err, "failed writing crash directory entry")
	}
	return entry, nil
}

// writeFakeMeta creates a fake crash report.
func writeFakeMeta(filename string, execName string, payload string, complete bool) (string, error) {
	lastLine := ""
	if complete {
		lastLine = "done=1\n"
	}
	content := fmt.Sprintf("exec_name=%s\nver=my_ver\npayload=%s\n%s", execName, payload, lastLine)
	return writeCrashDirEntry(filename, content)
}

// checkSendResult validates that the sent crash is correct.
func checkSendResult(ctx context.Context) error {
	return nil
}

// replaceCrashFilterIn replaces --filter_in= parameter of the crash reporter.
// The kernel is set up to call the crash reporter with the core dump
// as stdin when a process dies. This function adds a filter to the
// command line used to call the crash reporter. This is used to ignore
// crashes in which we have no interest.
//
// This removes any --filter_in= parameter and optionally replaces it
// with a new one.
func replaceCrashFilterIn(parm string) error {
	var (
		pattern  string
		stream   []byte
		err      error
		fileInfo os.FileInfo
	)

	if fileInfo, err = os.Stat(corePattern); err != nil {
		return errors.Wrapf(err, "failed getting core patern file info %s", corePattern)
	}
	if stream, err = ioutil.ReadFile(corePattern); err != nil {
		return errors.Wrapf(err, "failed reading core pattern file %s", corePattern)
	}
	pattern = string(stream)
	re := regexp.MustCompile(`--filter_in=\S*\s*`)
	idx := re.FindStringIndex(pattern)
	if idx != nil {
		pattern = fmt.Sprintf("%s%s", pattern[:idx[0]], pattern[idx[1]:])
	}
	if len(parm) != 0 {
		pattern = fmt.Sprintf("%s\n%s", pattern, parm)
	}

	if err = ioutil.WriteFile(corePattern, []byte(pattern), fileInfo.Mode().Perm()); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s", corePattern)
	}
	return nil
}

// initializeCrashReporter starts up the crash reporter.
func initializeCrashReporter(ctx context.Context, lock bool) error {
	var err error
	if !lock {
		if err = crashTestInProgress(false); err != nil {
			return err
		}
	}
	if _, err = testexec.CommandContext(ctx, crashReporterPath, "--init").Output(testexec.DumpLogOnError); err == nil && !lock {
		if err = crashTestInProgress(true); err != nil {
			return err
		}
		if err = crashFiltering("none", true); err != nil {
			return err
		}
	}

	return err
}

// killCrashSender kills the the crash_sender process if running.
func killCrashSender(ctx context.Context) error {
	// TODO(chavey) this is brute force, we should check if the process exist then kill it.
	_, err := testexec.CommandContext(ctx, "pkill", "-9", "-e", "crash_sender").Output(testexec.DumpLogOnError)
	return err
}

// crashFiltering adds a --filter_in argument to the kernel core dump cmdline.
func crashFiltering(name string, enable bool) error {
	var filter string
	if enable {
		filter = fmt.Sprintf("--filter_in=%s", name)
	}
	if err := replaceCrashFilterIn(filter); err != nil {
		return errors.Wrapf(err, "failed replacing crash filter %s", filter)
	}
	return nil
}

func crashTestInProgress(enable bool) error {
	if enable {
		if err := ioutil.WriteFile(crashTestInProgressFile, []byte("in-progress"), 0644); err != nil {
			return errors.Wrapf(err, "failed creating crash test in progress file %s", crashTestInProgressFile)
		}
	} else {
		return removeIfExist(crashTestInProgressFile, false)
	}
	return nil
}

func resetRateLimiting() error {
	return removeIfExist(crashSenderRateDir, true)
}

// clearSpooledCrashes clears system and user crash directories.
// This removes all crash reports which are waiting to be sent.
func clearSpooledCrashes() error {
	matches, err := filepath.Glob(userCrashDirs)
	if err != nil {
		return errors.Wrapf(err, "failed globing user crash dirs %s", userCrashDirs)
	}
	for _, match := range matches {
		if err = removeIfExist(match, true); err != nil {
			continue
		}
	}
	if err := removeIfExist(fallbackUserCrashDir, true); err != nil {
		return errors.Wrapf(err, "failed cleaning fallback user crash dir %s", fallbackUserCrashDir)
	}
	return nil
}

// systemSending sets whether or not the system crash_sender is allowed to run.
// This is done by creating or removing _PAUSE_FILE.
// crash_sender may still be allowed to run if _call_sender_one_crash is
// called with 'ignore_pause=True'.
func systemSending(enabled bool) error {
	if enabled {
		if err := removeIfExist(pauseFile, false); err != nil {
			return errors.Wrapf(err, "failed removing pause file %s", pauseFile)
		}
	} else {
		if err := ioutil.WriteFile(pauseFile, []byte(""), 0644); err != nil {
			return errors.Wrapf(err, "failed touching pause file %s", pauseFile)
		}
	}
	return nil
}

// sendingMock enables / disables mocking of the sending process.
// This uses the _MOCK_CRASH_SENDING file to achieve its aims. See notes
// at the top.
// Enables / disabled mocking of crash_send.
// This function uses the _MOCK_CRASH_SENDING file to enable / disable mocking.
func sendingMock(enabled bool, success bool) error {
	var data = []byte("")
	if enabled {
		if !success {
			data = []byte("1")
		}
		return ioutil.WriteFile(mockCrashSending, data, 0644)
	}
	return removeIfExist(mockCrashSending, false)
}
