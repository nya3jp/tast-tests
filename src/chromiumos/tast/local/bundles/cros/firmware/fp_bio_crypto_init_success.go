// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/firmware"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpBioCryptoInitSuccess,
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
	upstartLogPath    = "/var/log/upstart.log"
)

func FpBioCryptoInitSuccess(ctx context.Context, s *testing.State) {
	latest, err := firmware.GetAuditLogsSinceBoot(ctx)
	if err != nil {
		s.Fatal("Failed to get recent audit logs: ", err)
	}
	if strings.Contains(latest, violation) {
		s.Fatal("bio_crypto_init string found in audit logs")
	}

	upstartlogs, err := firmware.GetUpstartLogsSinceBoot(ctx)
	if err != nil {
		s.Fatal("Failed to get recent upstart logs: ", err)
	}
	if strings.Contains(string(upstartlogs), violation) {
		s.Fatal("bio_crypto_init string found in upstart.log")
	}

	cmd := testexec.CommandContext(ctx, "ectool", "--name=cros_fp", "fpencstatus")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get FP Encryption Status: ", err)
	}
	if !strings.Contains(string(out), seedSuccessString) {
		s.Fatal("FPTPM seet not set")
	}

}
