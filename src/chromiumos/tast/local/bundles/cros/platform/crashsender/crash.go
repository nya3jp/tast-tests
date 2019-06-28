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
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/logs"
	"chromiumos/tast/testing"
)

// TODO(chavey): There are some dup with udev_crash.go, can extract and combine.
// TODO(chavey): It would be best to have a crashtest package that can be
//   used as a lib for all the crash tests. see what makes more sense.
// TODO(chavey): If using a different package, some of those maybe needed by
//   crash tests outside of this package.
const (
	// TODO(chavey): There's an existing function metrics.SetConsent() which will
	//   set up consent properly. Please use that; it's more reliable and standard.
	//   Refactor all the consent code using
	//   tast/local/metrics/metrics.go, func SetConsent()
	consentFile = "/home/chronos/Consent To Send Stats"
	// corePattern is used to pipe core file into crash reporter. It will starts
	// with "|/sbin/crash_reporter". See core(5) man page.
	corePattern          = "/proc/sys/kernel/core_pattern"
	crashSenderPath      = "/sbin/crash_sender"
	fakeTestBasename     = "fake.1.2.3"
	fallbackUserCrashDir = "/home/chronos/crash"

	ownerKeyFile     = whiteListDir + "/owner.key"
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

// consentEnable sets consent to send crash reports.
// This creates the consent file controling whether
// crash_sender will consider that it has consent to send crash reports.
// It also copies a policy blob with the proper policy setting.
// TODO(chavey): Follow the real consent system which is recently implemented
//   in crrev.com/c/1730246
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
	if err := testexec.CommandContext(ctx, crashSenderPath, args...).Run(testexec.DumpLogOnError); err != nil {
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
		payload, err := writeCrashDirEntry(fakeTestBasename+".dmp", "")
		if err != nil {
			return "", errors.Wrapf(err, "failed writing fake test crash dmp file %s", fakeTestBasename)
		}
		if req.report, err = writeFakeMeta(fakeTestBasename+".meta", "fake", payload, true); err != nil {
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
		"--after-cursor="+cursor,
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
// TODO(chavey): Use syslog.NewWatcher() for log watching.
//   It currently watches /var/log/messages instead of journal.
//   crbug.com/991416 covers extending it to watch the journal.
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
// While test are running, set disableCrashDump = true to ensure that crashy
// systems do not make the test flaky.
func initializeCrashReporter(ctx context.Context, disableCrashDump bool) error {
	if disableCrashDump {
		if err := crashTestInProgress(false); err != nil {
			return err
		}
	}
	if err := testexec.CommandContext(ctx, "/sbin/crash_reporter", "--init").Run(testexec.DumpLogOnError); err == nil && disableCrashDump {
		if err := crashTestInProgress(true); err != nil {
			return err
		}
		// set filtering to disable crash reporting
		if err := replaceCrashFilterIn("none"); err != nil {
			return err
		}
	}

	return nil
}

// killCrashSender kills the the crash_sender process if running.
func killCrashSender(ctx context.Context) error {
	// TODO(chavey): Check the exit code of pkill.
	//   pkill returns 0 or 1 on sucess / no match, and 2 or 3 on actual
	//   error (man pgrep). Check the error status and return error only when
	//   it's 2 or 3. Unfortunately getting exit code is a bit tedious in Go 1.11
	//   though. https://golang.org/issue/26539

	testexec.CommandContext(ctx, "pkill", "-9", "-f", "crash_sender").Run()
	if err := testexec.CommandContext(ctx, "pgrep", "-f", "crash_sender").Run(); err == nil {
		return errors.New("failed to kill crash_sender")
	}
	return nil
}

// readCorePattern read the current content of the core_pattern file.
func readCorePattern() (string, error) {
	out, err := ioutil.ReadFile(corePattern)
	if err != nil {
		return "", errors.Wrapf(err, "failed reading core pattern file %s", corePattern)
	}
	return string(out), nil
}

// writeCorePattern writes the pattern to the core_pattern file.
func writeCorePattern(pattern string) error {
	if err := ioutil.WriteFile(corePattern, []byte(pattern), 0644); err != nil {
		return errors.Wrapf(err, "failed writing core pattern file %s", corePattern)
	}
	return nil
}

// replaceCrashFilterIn replaces --filter_in= flag value of the crash reporter.
// When param is an empty string, the flag will be removed.
// The kernel is set up to call the crash reporter with the core dump as stdin
// when a process dies. This function adds a filter to the command line used to
// call the crash reporter. This is used to ignore crashes in which we have no
// interest.
// TODO(chavey): need to reconciliate with yamaguchi-san CL.
func replaceCrashFilterIn(param string) error {
	pattern, err := readCorePattern()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(pattern, "|") {
		return errors.Wrapf(err, "pattern should start with '|', but was: %s", pattern)
	}
	e := strings.Split(strings.TrimSpace(pattern), " ")
	var newargs []string
	replaced := false
	for _, s := range e {
		if !strings.HasPrefix(s, "--filter_in=") {
			newargs = append(newargs, s)
			continue
		}
		if len(param) == 0 {
			// remove from list
			continue
		}
		newargs = append(newargs, "--filter_in="+strconv.Quote(param))
		replaced = true
	}
	if len(param) != 0 && !replaced {
		newargs = append(newargs, "--filter_in="+strconv.Quote(param))
	}
	pattern = strings.Join(newargs, " ")
	return writeCorePattern(pattern)
}

// crashTestInProgress check is test is currently in progress or not.
// TODO(chavey): This is needed by a number of crash tests.
//   it is being added to a more widely usable location
//   src/chromiumos/tast/common/crash/crash.go in
//   https://chromium-review.googlesource.com/c/chromiumos/platform/tast-tests/+/1762454
func crashTestInProgress(enable bool) error {
	// crashTestInProgressFile indicates by its existence a crash test is currently
	// running, and lets crash-reporter run as if it's non test image.
	crashTestInProgressFile := "/run/crash_reporter" + "/crash-test-in-progress"
	if enable {
		if err := ioutil.WriteFile(crashTestInProgressFile, []byte("in-progress"), 0644); err != nil {
			return errors.Wrapf(err, "failed creating crash test in progress file %s", crashTestInProgressFile)
		}
	} else {
		if err := os.Remove(crashTestInProgressFile); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// resetRateLimiting removes existing crash report timestamps so that crash
// report it not disabled by rate limit.
func resetRateLimiting() error {
	if err := os.RemoveAll("/var/lib/crash_sender"); err != nil {
		return err
	}
	return nil
}

// clearSpooledCrashes clears system and user crash directories.
// This removes all crash reports which are waiting to be sent.
func clearSpooledCrashes() error {
	matches, err := filepath.Glob(userCrashDirs)
	if err != nil {
		return errors.Wrapf(err, "failed globing user crash dirs %s", userCrashDirs)
	}
	for _, match := range matches {
		if err := os.RemoveAll(match); err != nil {
			continue
		}
	}
	if err := os.RemoveAll(fallbackUserCrashDir); err != nil {
		return errors.Wrapf(err, "failed cleaning fallback user crash dir %s", fallbackUserCrashDir)
	}
	return nil
}

const (
	// pauseFile (if exists) causes crash sending to be paused. This behavior can
	// be overwritten by --ignore_pause_file.
	pauseFile = "/var/lib/crash_sender_paused"
)

// enableCrashSender enables the system crash_sender.
// This is done by creating pauseFile.
func enableCrashSender() error {
	if err := os.Remove(pauseFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed removing pause file %s", pauseFile)
	}
	return nil
}

// disableCrashSender disables the system crash_sender.
// This is done by removing pauseFile.
func disableCrashSender() error {
	if err := ioutil.WriteFile(pauseFile, []byte(""), 0644); err != nil {
		return errors.Wrapf(err, "failed touching pause file %s", pauseFile)
	}
	return nil
}

const (
	// mockCrashSending is a file controlling the behavior of crash_sender.
	// If the file doesn't exist, then crash_sender runs normally.
	// If the file exists but it is empty, crash_sender will succeed as nop.
	// If the file contains something, then crash_sender will fail.
	mockCrashSending = "/run/crash_reporter" + "/mock-crash-sending"
)

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
	if err := os.RemoveAll(mockCrashSending); err != nil {
		return err
	}
	return nil
}
