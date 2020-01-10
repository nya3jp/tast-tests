// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpSensor,
		Desc: "Checks that ectool commands for fingerprint sensor behave as expected",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
	})
}

func FpSensor(ctx context.Context, s *testing.State) {
	exp := regexp.MustCompile("FPMCU encryption status: 0x[a-f0-9]{7}1(.+)FPTPM_seed_set")
	cmd := testexec.CommandContext(ctx, "ectool", "--name=cros_fp", "fpencstatus")
	s.Logf("Running command: %q", shutil.EscapeSlice(cmd.Args))
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Errorf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	} else if !exp.MatchString(string(out)) {
		s.Errorf("FPTPM seed is not set; output %q doesn't match regex %q", string(out), exp)
	}
}
