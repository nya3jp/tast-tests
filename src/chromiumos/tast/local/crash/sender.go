// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// Functions for testing crash_sender. It includes mocking the crash sender, as well as
// verifying the output of the crash sender.

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
