// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"testing"
)

func TestParseSenderOutput(t *testing.T) {
	parsed, err := parseSenderOutput(context.Background(), `
Evaluating crash repo
rt: /var/spool/crash/platform_UserCrash_crasher.20191227.155539.22407.meta
Scheduled to send in 529s
Current send rate: 0 sends and 0 bytes/24hrs
Sending crash:
 Metadata: /var/spool/crash/platform_UserCrash_crasher.20191227.155539.22407.meta (minidump)
 Payload: /var/spool/crash/platform_UserCrash_crasher.20191227.155539.22407.dmp
 Version: 12767.0.0
 Product: ChromeOS
 URL: https://clients2.google.com/cr/report
 Board: caroline
 HWClass: CAROLINE C00-***
 Exec name: platform.UserCrash.crasher
 sig: SIGNATURE_OF_THE_REPORT
 Image type: IMAGE_TYPE
 Boot mode: BOOT_MODE
 Mocking successful send
 `)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	for _, i := range []struct {
		expected  string
		actual    string
		fieldName string
	}{
		{"/var/spool/crash/platform_UserCrash_crasher.20191227.155539.22407.meta", parsed.MetaPath, "MetaPath"},
		{"minidump", parsed.ReportKind, "ReportKind"},
		{"/var/spool/crash/platform_UserCrash_crasher.20191227.155539.22407.dmp", parsed.ReportPayload, "ReportPayload"},
		{"platform.UserCrash.crasher", parsed.ExecName, "ExecName"},
		{"SIGNATURE_OF_THE_REPORT", parsed.Sig, "Sig"},
		{"IMAGE_TYPE", parsed.ImageType, "ImageType"},
		{"BOOT_MODE", parsed.BootMode, "BootMode"},
	} {
		if i.expected != i.actual {
			t.Errorf("%s got %q, want %q", i.fieldName, i.actual, i.expected)
		}
	}
	chkBool := func(t *testing.T, expect, actual bool, fieldName string) {
		if expect != actual {
			t.Errorf("%s: got %t, want %t", fieldName, actual, expect)
		}
	}
	chkBool(t, true, parsed.SendAttempt, "SendAttempt")
	chkBool(t, true, parsed.SendSuccess, "SendSuccess")
	if parsed.SleepTime != 529 {
		t.Errorf("SleepTime got %d, want %d", parsed.SleepTime, 529)
	}

	// Another test case (w/ empty log)
	parsed, err = parseSenderOutput(context.Background(), "")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	// "Scheduled to send" log not exist in the log. Indicates script didn't attempt sending.
	chkBool(t, false, parsed.SendAttempt, "SendAttempt")
	// "Mocking successful send" not exist. Indicates non-success.
	chkBool(t, false, parsed.SendSuccess, "SendSuccess")
}

func TestParseSenderOutputTrimAnomaly(t *testing.T) {
	input := `
 Metadata: MY_METADATA (minudump)
Considering metadata //.../kernel_warning
 Payload: IGNORE_ME
Considering metadata /var/spool/crash/foo
 Exec name: MY_EXEC
`
	expectedOutput := `
 Metadata: MY_METADATA (minudump)
Considering metadata /var/spool/crash/foo
 Exec name: MY_EXEC
`
	parsed, err := parseSenderOutput(context.Background(), input)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if parsed.Output != expectedOutput {
		t.Fatalf("Output got %q, want %q", parsed.Output, expectedOutput)
	}
	if parsed.ReportPayload != "" {
		t.Error("anomaly entry was not ignored")
	}
	if parsed.MetaPath != "MY_METADATA" {
		t.Error("data dropped before anomaly")
	}
	if parsed.ExecName != "MY_EXEC" {
		t.Error("data dropped after anomaly")
	}
}
