// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package biod

import (
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BioCryptoInitSuccess,
		Desc: "Checks that bio crypto init finishes gracefully without violations and that FPMCU seed is set",
		Contacts: []string{
			"josienordrum@google.org", // Test Author
			"hesling@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
	})
}

const (
	seedSuccessString = "FPTPM_seed_set"
	violation         = "bio_crypto_init"
)

func BioCryptoInitSuccess(ctx context.Context, s *testing.State) {
	auditLogs, err := syslog.GetAuditLogsSinceBoot(ctx)
	if err != nil {
		s.Fatal("Failed to get recent audit logs: ", err)
	}
	for _, l := range auditLogs {
		if strings.Contains(l, violation) {
			s.Fatal("bio_crypto_init warning found in audit logs, check syscall https://chromium.googlesource.com/chromiumos/docs/+/HEAD/constants/syscalls.md: ", l)
		}
	}

	upstartlogs, err := syslog.GetUpstartLogsSinceBoot(ctx)
	if err != nil {
		s.Fatal("Failed to get recent upstart logs: ", err)
	}
	rx := regexp.MustCompile(`WARNING.*bio_crypto_init`)
	for _, l := range upstartlogs {
		if rx.MatchString(l) != false {
			s.Fatal("bio_crypto_init string found in upstart.log: ", l)
		}
	}

	cmd := testexec.CommandContext(ctx, "ectool", "--name=cros_fp", "fpencstatus")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get FP Encryption Status: ", err)
	}
	fpEncStatus := strings.Split(string(out), "\n")
	if !strings.Contains(fpEncStatus[0], seedSuccessString) {
		s.Fatal("FPTPM seet not set")
	}

}
