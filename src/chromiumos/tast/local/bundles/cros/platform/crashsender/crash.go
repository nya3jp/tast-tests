// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crashsender

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/logs"
	"chromiumos/tast/testing"
)

// TODO(chavey): there are some dup with udev_crash.go, can extract and combine.
// TODO(chavey): it would be best to have a crashtest package that can be
// used as a lib for all the crash tests. see what makes more sense.
// TODO(chavey): If using a different package, some of those maybe needed by
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

	// mockCrashSending is a file controlling the behavior of crash_sender.
	// If the file doesn't exist, then crash_sender runs normally.
	// If the file exists but it is empty, crash_sender will succeed as nop.
	// If the file contains something, then crash_sender will fail.
	mockCrashSending = crashRunStateDir + "/mock-crash-sending"

	ownerKeyFile     = whiteListDir + "/owner.key"
	pauseFile        = "/var/lib/crash_sender_paused"
	signedPolicyFile = whiteListDir + "/policy"
	systemCrashDir   = "/var/spool/crash"
	userCrashDirs    = "/home/chronos/u-*/crash"
	whiteListDir     = "/var/lib/whitelist"
)

type crashRequest struct {
	// sendSuccess mocks a successful send if true.
	sendSuccess bool
	// reportsEnabled indicates that the user consented to sending create reports.
	reportsEnabled bool
	// report use for crash, if empty create one.
	report string
	// shouldFail indicates if the crash_sender program is expected to fail.
	shouldFail bool
	// ignorePause indicates that crash_sender should ignore the pause file.
	ignorePause bool
	// mock files
	mockOffPolicyFile string
	mockOnPolicyFile  string
	mockKeyFile       string
}

func removeIfExist(name string, all bool) error {
	if all {
		if err := os.RemoveAll(name); err != nil {
			return err
		}
		return nil
	}
	if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// consentEnable sets consent to send crash reports.
// This creates the consent file controling whether
// crash_sender will consider that it has consent to send crash reports.
// It also copies a policy blob with the proper policy setting.
func consentEnable(ctx context.Context, req *crashRequest) error {
	// create policy and own key files to enable metric/consent
	if info, err := os.Stat(whiteListDir); err == nil && info.IsDir() {
		if err := fsutil.CopyFile(req.mockOnPolicyFile, signedPolicyFile); err != nil {
			return errors.Wrapf(err, "failed copying %s to %s",
				req.mockOnPolicyFile, signedPolicyFile)
		}
		if err := fsutil.CopyFile(req.mockKeyFile, ownerKeyFile); err != nil {
			return errors.Wrapf(err, "failed to copying %s to %s",
				req.mockKeyFile, ownerKeyFile)
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
	return nil
}

// consentDisable unsets consent to send crash reports.
// Deletes the consent file to control whether crash_sender has consent to send
// crash reports. It also copies a policy blob with the proper policy setting.
func consentDisable(ctx context.Context, req *crashRequest) error {
	if info, err := os.Stat(whiteListDir); err == nil && info.IsDir() {
		if err := fsutil.CopyFile(req.mockOffPolicyFile, signedPolicyFile); err != nil {
			return errors.Wrapf(err, "failed to create %s", signedPolicyFile)
		}
		if err := fsutil.CopyFile(req.mockKeyFile, ownerKeyFile); err != nil {
			return errors.Wrapf(err, "failed to create %s", ownerKeyFile)
		}
	}
	if err := os.Remove(consentFile); err != nil {
		return errors.Wrapf(err, "failed removing %s", consentFile)
	}

	return nil
}

// callCrashSender calls the crash sender script to upload one crash.
func callCrashSender(ctx context.Context, req *crashRequest) (map[string]string, error) {
	cursor, err := logsCursor(ctx)
	if err != nil {
		return make(map[string]string), errors.Wrap(err, "failed getting the log current tip")
	}

	if req.report, err = prepareSenderCrash(ctx, req); err != nil {
		return make(map[string]string), errors.Wrap(err, "failed to prepare sender crash")
	}

	var args []string
	if req.ignorePause {
		args = append(args, "--ignore_pause_file")
	}
	if err = testexec.CommandContext(ctx, crashSenderPath, args...).Run(testexec.DumpLogOnError); err != nil {
		return make(map[string]string), errors.Wrap(err, "failed calling crash_sender")
	}

	var m map[string]string
	if m, err = waitForSenderCompletion(ctx, cursor, "crash_sender done. (mock)", 10); err != nil {
		return m, errors.Wrap(err, "failed to get sender crash completion")
	}

	return m, nil
}

// prepareSenderCrash creates metadata for a fake crash report.
// Enables mocking of the crash sender, then creates a fake crash report for testing purposes.
func prepareSenderCrash(ctx context.Context, req *crashRequest) (string, error) {
	if err := enableCrashSenderMock(req.sendSuccess); err != nil {
		return "", errors.Wrap(err, "failed enabling crash_sender mock")
	}

	if req.reportsEnabled {
		if err := consentEnable(ctx, req); err != nil {
			return "", errors.Wrap(err, "failed enabling consent")
		}
	} else {
		if err := consentDisable(ctx, req); err != nil {
			return "", errors.Wrap(err, "failed disabling consent")
		}
	}

	if len(req.report) == 0 {
		// Use the same file format as crash does normally:
		// <basename>.#.#.#.meta
		payload, err := writeCrashDirEntry(fmt.Sprintf("%s.dmp", fakeTestBasename), "")
		if err != nil {
			return "", errors.Wrapf(err, "failed writing fake test crash dmp file %s", fakeTestBasename)
		}
		if req.report, err = writeFakeMeta(fmt.Sprintf("%s.meta", fakeTestBasename), "fake", payload, true); err != nil {
			return "", errors.Wrapf(err, "failed writing fake test crash meta file %", fakeTestBasename)
		}
	}
	return req.report, nil
}

// waitForSenderCompletion waits for crash_sender to complete.
// Wait for crash_sender done message to be placed in the
// system log before continuing and for the process to finish.
// Otherwise we might get only part of the output.
//   cursor is set to the log location to start reading from.
//   key is the log message that signals completion.
//   timeout in seconds before timing out.
func waitForSenderCompletion(ctx context.Context, cursor string, key string, timeout time.Duration) (map[string]string, error) {
	var logMap map[string]string
	err := testing.Poll(ctx,
		func(ctx context.Context) error {
			var err error
			for true {
				if logMap, _, err = logsRead(ctx, cursor); err != nil {
					break
				}
				if _, ok := logMap[key]; ok {
					return nil
				}
				if cursor, err = logsCursor(ctx); err != nil {
					break
				}
			}
			return errors.Wrap(err, "failed to get map from the log")
		},
		&testing.PollOptions{Timeout: timeout * time.Second})

	if err != nil {
		return make(map[string]string), errors.Wrap(err, "failed polling for crash sender completion")
	}

	return logMap, nil
}

// logsRead Read all log entries generated after cursor.
//   cursor should be set by an earlier call to logsCursor.
func logsRead(ctx context.Context, cursor string) (map[string]string, []byte, error) {
	b, err := testexec.CommandContext(ctx,
		"journalctl",
		fmt.Sprintf("--after-cursor=%s", cursor),
		"--no-pager",
		"_COMM=crash_sender").Output(testexec.DumpLogOnError)
	if err != nil {
		return make(map[string]string), []byte(""), err
	}

	r, err := regexp.Compile(`^.*\[\d+\]:\s+`)
	if err != nil {
		return make(map[string]string), []byte(""), errors.Wrap(err, "failed compiling regex")
	}
	m := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		l := scanner.Text()
		i := r.FindStringIndex(l)
		if i != nil {
			v := string(l[i[1]:])
			q := strings.Split(v, ":")
			// looking for exact match of ^\S.+:\s*.+$
			if len(q) == 2 {
				// q[0] does not contain any white leading spaces
				m[q[0]] = strings.TrimSpace(q[1])
			} else {
				m[q[0]] = "unspec"
			}
		}
	}

	if err := scanner.Err(); err != nil {
		testing.ContextLogf(ctx, "%s", err)
		// Return an empty map since we could not parse the raw bytes.
		// Return the read bytes since those were successfuly read.
		return make(map[string]string), b, err
	}
	return m, b, nil
}

// logsCursor get the log current cursor.
func logsCursor(ctx context.Context) (string, error) {
	cursor, err := logs.GetJournaldCursor(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed getting journald cursor")
	}
	return cursor, nil
}

func writeCrashDirEntry(filename string, content string) (string, error) {
	if err := os.MkdirAll(systemCrashDir, 0644); err != nil {
		return "", errors.Wrapf(err, "failed to create %s", systemCrashDir)
	}
	entry := filepath.Join(systemCrashDir, filename)
	if err := ioutil.WriteFile(entry, []byte(content), 0644); err != nil {
		return "", errors.Wrap(err, "failed writing crash directory entry")
	}
	return entry, nil
}

// writeFakeMeta creates a fake crash report.
func writeFakeMeta(filename, execName, payload string, complete bool) (string, error) {
	lines := []string{
		"exec_name=" + execName,
		"ver=my_ver",
		"payload=" + payload,
	}
	if complete {
		lines = append(lines, "done=1")
	}
	content := strings.Join(lines, "\n") + "\n"
	return writeCrashDirEntry(filename, content)
}

// initializeCrashReporter starts up the crash reporter.
// If call while test are in progress, set updateProgress to true.
// While test are running, set disableCrashDump = true to ensure that crashy
// systems do not make the test flaky.
func initializeCrashReporter(ctx context.Context, disableCrashDump bool) error {
	if disableCrashDump {
		if err := crashTestInProgress(false); err != nil {
			return err
		}
	}
	if _, err := testexec.CommandContext(ctx, crashReporterPath, "--init").Output(testexec.DumpLogOnError); err == nil && disableCrashDump {
		if err := crashTestInProgress(true); err != nil {
			return err
		}
		// set filtering to disable crash reporting
		if err := crashFiltering("none"); err != nil {
			return err
		}
	}

	return nil
}

// killCrashSender kills the the crash_sender process if running.
func killCrashSender(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "pgrep", "-f", "crash_sender").Run(); err != nil {
		return nil
	}
	return testexec.CommandContext(ctx, "pkill", "-9", "-f", "crash_sender").Run()
}

// readCrashFilter read the current content of the core_pattern file.
func readCrashFilter() (string, error) {
	out, err := ioutil.ReadFile(corePattern)
	if err != nil {
		return "", errors.Wrapf(err, "failed reading core pattern file %s", corePattern)
	}
	return string(out), nil
}

// writeCrashFilter writes the pattern to the core_pattern file.
func writeCrashFilter(pattern string) error {
	if err := ioutil.WriteFile(corePattern, []byte(pattern), 0644); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s", corePattern)
	}
	return nil
}

// crashFiltering adds a --filter_in argument to the kernel core dump cmdline.
func crashFiltering(name string) error {
	if len(name) == 0 {
		return nil
	}

	filterIn := fmt.Sprintf("--filter_in=%s", name)

	out, err := readCrashFilter()
	if err != nil {
		return err
	}
	pattern := string(out)
	if !strings.HasPrefix(pattern, "|/sbin/crash_reporter") {
		return errors.Errorf("%s has invalid content %s", corePattern, pattern)
	}
	re := regexp.MustCompile(`--filter_in=\S*\s*`)
	pattern = fmt.Sprintf(string(re.ReplaceAll([]byte(pattern), []byte("$1"))), filterIn)

	return writeCrashFilter(pattern)
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

// enableCrashSender enables the system crash_sender.
// This is done by creating pauseFile.
func enableCrashSender() error {
	if err := removeIfExist(pauseFile, false); err != nil {
		return errors.Wrapf(err, "failed removing pause file %s", pauseFile)
	}
	return nil
}

// disableCrashSender disables the system crash_sender.
// This is done by removing _PAUSE_FILE.
func disableCrashSender() error {
	if err := ioutil.WriteFile(pauseFile, []byte(""), 0644); err != nil {
		return errors.Wrapf(err, "failed touching pause file %s", pauseFile)
	}
	return nil
}

// enableCrashSenderMock enables mocking of crash_sender.
// Add the mockCrashSending file to enable mocking.
func enableCrashSenderMock(success bool) error {
	var data = []byte("")
	if !success {
		data = []byte("1")
	}
	return ioutil.WriteFile(mockCrashSending, data, 0644)
}

// disableCrashSenderMock disables mocking of crash_sender.
// Remove the mockCrashSending file to disable mocking.
func disableCrashSenderMock() error {
	return removeIfExist(mockCrashSending, false)
}
