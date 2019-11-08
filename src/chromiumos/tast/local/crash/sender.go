// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Functions for testing crash_sender. It includes mocking the crash sender, as well as
// verifying the output of the crash sender.

const (
	crashSenderRateDir = "/var/lib/crash_sender"
	crashRunStateDir   = "/run/crash_reporter"
	crashSenderPath    = "/sbin/crash_sender"
	mockCrashSending   = crashRunStateDir + "/mock-crash-sending"
)

var chromeosVersionRegex = regexp.MustCompile("CHROMEOS_RELEASE_VERSION=(.*)")

// SenderOutput represents data extracted from crash sender execution result.
type SenderOutput struct {
	ExecName      string // name of executable which crashed
	ImageType     string // type of image ("dev","test",...), if given
	BootMode      string // current boot mode ("dev",...), if given
	MetaPath      string // path to the report metadata file
	Output        string // the output from the script, copied
	ReportKind    string // kind of report sent (minidump vs kernel)
	ReportPayload string // payload of report sent
	SendAttempt   bool   // did the script attempt to send a crash
	SendSuccess   bool   // if it attempted, was the crash send successful
	Sig           string // signature of the report, or empty string if not given
	SleepTime     int    // if it attempted, how long did it sleep before. -1 otherwise.
	Sending       int    // (if mocked, how long would it have slept)
	ReportExists  bool   // whether the minidump still exist after calling send script
	RateCount     int    // number of crashes that have been uploaded in the past 24 hours
}

// parseSenderOutput parses the log output from the crash_sender script.
// This script can run on the logs from either a mocked or true
// crash send. It looks for one and only one crash from output.
// Non-crash anomalies should be ignored since there're just noise
// during running the test.
func parseSenderOutput(ctx context.Context, output string) (*SenderOutput, error) {
	anomalyTypes := []string{
		"kernel_suspend_warning",
		"kernel_warning",
		"kernel_wifi_warning",
		"selinux_violation",
		"service_failure",
	}

	// Narrow search to lines from crash_sender.
	// returns a string slice with:
	// 0: string before match
	// 1, ... : nth groups in the pattern
	crashSenderSearchIndex := func(pattern string, output string) []int {
		return regexp.MustCompile(pattern).FindStringSubmatchIndex(output)
	}

	crashSenderSearch := func(pattern string, output string) []string {
		return regexp.MustCompile(pattern).FindStringSubmatch(output)
	}
	beforeFirstCrash := "" // None
	isAnormaly := func(s string) bool {
		for _, a := range anomalyTypes {
			if strings.Contains(s, a) {
				return true
			}
		}
		return false
	}

	for {
		crashHeader := crashSenderSearchIndex(`Considering metadata (\S+)`, output)
		if crashHeader == nil {
			break
		}
		if beforeFirstCrash == "" {
			beforeFirstCrash = output[0:crashHeader[0]]
		}
		metaConsidered := output[crashHeader[0]:crashHeader[1]]
		if isAnormaly(metaConsidered) {
			// If it's an anomaly, skip this header, and look for next one.
			output = output[crashHeader[1]:]
		} else {
			// If it's not an anomaly, skip everything before this header.
			// TODO(yamaguchi): This will only trim the first anomaly block.
			// it may contain another reprt added after the first one. Fix it.
			output = output[crashHeader[0]:]
			break
		}
	}

	if beforeFirstCrash != "" {
		output = beforeFirstCrash + output
		// logging.debug('Filtered sender output to parse:\n%s', output)
	}

	sleepMatch := crashSenderSearch(`Scheduled to send in (\d+)s`, output)
	sendAttempt := sleepMatch != nil
	var sleepTime int
	if sendAttempt {
		var err error
		s := sleepMatch[1]
		sleepTime, err = strconv.Atoi(s)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid sleep time in log: %s", s)
		}
	} else {
		sleepTime = -1 // None
	}
	var metaPath, reportKind string
	if m := crashSenderSearch(`Metadata: (\S+) \((\S+)\)`, output); m != nil {
		metaPath = m[1]
		reportKind = m[2]
	}
	var reportPayload string
	if m := crashSenderSearch(`Payload: (\S+)`, output); m != nil {
		reportPayload = m[1]
	}
	var execName string
	if m := crashSenderSearch(`Exec name: (\S+)`, output); m != nil {
		execName = m[1]
	}
	var sig string
	if m := crashSenderSearch(`sig: (\S+)`, output); m != nil {
		sig = m[1]
	}
	var imageType string
	if m := crashSenderSearch(`Image type: (\S+)`, output); m != nil {
		imageType = m[1]
	}
	var bootMode string
	if m := crashSenderSearch(`Boot mode: (\S+)`, output); m != nil {
		bootMode = m[1]
	}
	sendSuccess := strings.Contains(output, "Mocking successful send")

	return &SenderOutput{
		ExecName:      execName,
		ReportKind:    reportKind,
		MetaPath:      metaPath,
		ReportPayload: reportPayload,
		SendAttempt:   sendAttempt,
		SendSuccess:   sendSuccess,
		Sig:           sig,
		ImageType:     imageType,
		BootMode:      bootMode,
		SleepTime:     sleepTime,
		Output:        output,
	}, nil
}

// senderOptions contains options for callSenderOneCrash.
type senderOptions struct {
	SendSuccess bool   // Mock a successful send if true
	Report      string // report to use for crash, if --None-- we create one.
	ShouldFail  bool   // expect the crash_sender program to fail
	IgnorePause bool   // crash_sender should ignore pause file existence
}

// DefaultSenderOptions creates a senderOptions object with default values.
func DefaultSenderOptions() senderOptions {
	return senderOptions{
		SendSuccess: true,
		ShouldFail:  false,
		IgnorePause: true,
	}
}

// SetSendingMock enables / disables mocking of the sending process.
// This uses the _MOCK_CRASH_SENDING file to achieve its aims. See notes
// at the top. sendSuccess decides whether the mock sends success or failure.
func SetSendingMock(enableMock bool, sendSuccess bool) error {
	if enableMock {
		var data string
		if sendSuccess {
			data = ""
		} else {
			data = "1"
		}
		if err := ioutil.WriteFile(mockCrashSending, []byte(data), 0644); err != nil {
			return errors.Wrap(err, "failed to create pause file")
		}
	} else {
		if err := os.Remove(mockCrashSending); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to remove mock crash file %s", mockCrashSending)
		}
	}
	return nil
}

// writeFakeMeta writes a fake meta entry to the system crash directory.
// This is not used unless the call_sender_one_crash is not called with report="".
func writeFakeMeta(name string, execName string, payload string) (string, error) {
	contents := fmt.Sprintf("exec_name=%s\n"+
		"ver=my_ver\n"+
		"payload=%s\n"+
		"done=1\n",
		execName, payload)
	return writeCrashDirEntry(name, []byte(contents))
}

// writeCrashDirEntry Writes a file to the crash directory of the system crash directory with the given name.
// This is used to insert new crash dump files for testing purposes.
func writeCrashDirEntry(name string, contents []byte) (string, error) {
	entry, err := GetCrashDir("root")
	if err != nil {
		return "", errors.Wrapf(err, "failed to get crash dir for user %s", name)
	}
	_, err = os.Stat(SystemCrashDir)
	if err != nil && os.IsNotExist(err) {
		if err := os.Mkdir(SystemCrashDir, os.FileMode(0770)); err != nil {
			return "", errors.Wrapf(err, "failed to create crash directory %s", SystemCrashDir)
		}
	}
	if err := ioutil.WriteFile(entry, contents, 0660); err != nil {
		return "", errors.Wrap(err, "failed to write crash dir entry")
	}
	return entry, nil
}

// waitForSenderCompletion waits for no crash_sender's last message to be placed in the
// system log before continuing and for the process to finish.
// Otherwise we might get only part of the output.
func waitForSenderCompletion(ctx context.Context, reader *syslog.Reader) error {
	c, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	_, err := reader.Wait(c, time.Hour /* unused */, func(e *syslog.Entry) bool {
		return strings.Contains(e.Content, "crash_sender done.")
	})
	if err != nil {
		return errors.Wrap(err, "crash_sender completion log did not appear")
	}
	c, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := WaitForProcessEnd(c, "crash_sender"); err != nil {
		return errors.Wrap(err, "crash_sender process did not end correctly")
	}
	return nil
}

// getDmpContents creates the contents of the dmp file for our made crashes.
// The dmp file contents are deliberately large and hard-to-compress. This
// ensures logging_CrashSender hits its bytes/day cap before its sends/day
// cap.
func getDmpContents() []byte {
	// Matches kDefaultMaxUploadBytes
	const maxCrashSize = 1024 * 1024
	result := make([]byte, maxCrashSize, maxCrashSize)
	rand.Read(result)
	return result
}

// prepareSenderOneCrash creates metadata for a fake crash report.
// This enables mocking of the crash sender, then creates a fake crash report for testing purposes.
//
// The crash_sender will succeed if send_success is set True, or otherwise made fail.
// If reportsEnabled is True, it enables the consent to send reports will be sent.
// report is the report to use for crash. If it's empty string, we create one.
func prepareSenderOneCrash(ctx context.Context, cr *chrome.Chrome, sendSuccess bool, report string) (string, error) {
	// Use the same file format as crash does normally:
	// <basename>.#.#.#.meta
	const fakeTestBasename = "fake.1.2.3"
	testing.ContextLog(ctx, "Setting SendingMock")
	SetSendingMock(true /* enableMock */, sendSuccess)
	if report == "" {
		// Use the same file format as crash does normally:
		// <basename>.#.#.#.meta
		payload, err := writeCrashDirEntry(fmt.Sprintf("%s.dmp", fakeTestBasename), getDmpContents())
		if err != nil {
			return "", errors.Wrap(err, "failed while preparing sender one crash")
		}
		report, err = writeFakeMeta(fmt.Sprintf("%s.meta", fakeTestBasename), "fake", payload)
		if err != nil {
			return "", errors.Wrap(err, "failed while preparing sender one crash")
		}
	}
	return report, nil
}

// callSenderOneCrash calls the crash sender script to mock upload one crash.
func callSenderOneCrash(ctx context.Context, cr *chrome.Chrome, opts senderOptions) (*SenderOutput, error) {
	report, err := prepareSenderOneCrash(ctx, cr, opts.SendSuccess, opts.Report)
	opts.Report = report
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare senderOneCrash")
	}
	SetSendingMock(true /* enableMock */, opts.SendSuccess)
	w, err := syslog.NewReader(syslog.Program("crash_sender"))
	logReader, err := syslog.NewReader(syslog.Program("crash_sender"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create syslog reader")
	}

	var option string
	if opts.IgnorePause {
		option = "--ignore_pause_file"
	}
	cmd := testexec.CommandContext(ctx, crashSenderPath, option)
	scriptOutput, err := cmd.CombinedOutput()
	if code, ok := ExitCode(err); !ok {
		return nil, errors.Wrap(err, "failed to get exit code of crash_sender")
	} else if code != 0 && !opts.ShouldFail {
		return nil, errors.Errorf("crash_sender returned an unexpected non-zero value (%d)", code)
	}

	if err := waitForSenderCompletion(ctx, w); err != nil {
		return nil, err
	}
	output := ""
	for {
		entry, err := logReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, errors.Wrap(err, "failed to get logs")
		}
		output = output + "\n" + entry.Content
	}
	if string(scriptOutput) != "" {
		testing.ContextLogf(ctx, "crash_sender stdout/stderr: %s", scriptOutput)
	}

	var reportExists bool
	fileInfo, err := os.Stat(opts.Report)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "failed to stat report file %s", opts.Report)
	}
	if err == nil {
		if fileInfo.IsDir() {
			return nil, errors.Errorf("report file expected, but %s is a directory", opts.Report)
		}
		reportExists = true
		os.Remove(opts.Report)
	}

	var rateCount int
	fileInfo, err = os.Stat(crashSenderRateDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to stat crash sender rate directory")
	}
	if err == nil {
		files, err := ioutil.ReadDir(crashSenderRateDir)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read crash sender rate directory")
		}
		for _, f := range files {
			if f.Mode().IsRegular() {
				rateCount++
			}
		}
	}

	result, err := parseSenderOutput(ctx, output)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse sender output")
	}

	result.ReportExists = reportExists
	result.RateCount = rateCount

	// Show the result for debugging but remove 'output' field
	// since it's large and earlier in debug output.
	var debugResult = *result
	debugResult.Output = ""
	testing.ContextLog(ctx, "Result of send (besides output): ", debugResult)

	return result, nil
}

// CheckGeneratedReportSending checks that report sendnig works.
// metaPath and payloadPath, execName, reportKind, and expectedSig specifies the test expectation.
func CheckGeneratedReportSending(ctx context.Context, cr *chrome.Chrome, metaPath, payloadPath, execName, reportKind, expectedSig string) error {
	o := DefaultSenderOptions()
	o.Report = filepath.Base(payloadPath)
	result, err := callSenderOneCrash(ctx, cr, o)
	if err != nil {
		return errors.Wrap(err, "failed to call sender one crash")
	}
	if !result.SendAttempt || !result.SendSuccess || result.ReportExists {
		return errors.Errorf("Report not sent properly: sendAttempt=%v, sendSuccess=%v, reportExists=%v",
			result.SendAttempt, result.SendSuccess, result.ReportExists)
	}
	if result.ExecName != execName {
		return errors.Errorf("executable name incorrect: want %q, got %q", execName, result.ExecName)
	}
	if result.ReportKind != reportKind {
		return errors.Errorf("Wrong report type: want %q, got %q", reportKind, result.ReportKind)
	}
	if result.ReportPayload != payloadPath {
		return errors.Errorf("Sent the wrong minidump payload: want %q, got %q", payloadPath, result.ReportPayload)
	}
	if result.MetaPath != metaPath {
		return errors.Errorf("Used the wrong meta file: want %q, got %q", metaPath, result.MetaPath)
	}
	if expectedSig == "" {
		if result.Sig != "" {
			return errors.New("Report should not have signature")
		}
	} else if result.Sig != expectedSig {
		return errors.Errorf("Report signature mismatch: want %q, got %q", expectedSig, result.Sig)
	}

	b, err := ioutil.ReadFile("/etc/lsb-release")
	if err != nil {
		return errors.Wrap(err, "failed to get chromeos version")
	}
	m := chromeosVersionRegex.FindStringSubmatch(string(b))
	if m == nil {
		return errors.Errorf("failed to get chromeos version in lsb-release: %s", string(b))
	}
	version := m[1]
	if m == nil || !strings.Contains(result.Output, version) {
		return errors.Errorf("missing version %s in log output [%s]", version, result.Output)
	}
	return nil
}

// ResetRateLimiting resets the count of crash reports sent today.
// This clears the contents of the rate limiting directory which has
// the effect of reseting our count of crash reports sent.
func ResetRateLimiting() error {
	if err := os.RemoveAll(crashSenderRateDir); err != nil {
		return errors.Wrapf(err, "failed cleaning crash sender rate dir %s", crashSenderRateDir)
	}
	return nil
}
