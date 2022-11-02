// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

var ()

func init() {
	testing.AddTest(&testing.Test{
		Func:     SuspendLeavesS0,
		Desc:     "Check to see that the EC sees that we've left S0 when suspending",
		Contacts: []string{"eizan@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func SuspendLeavesS0(ctx context.Context, s *testing.State) {
	if err := testexec.CommandContext(ctx, "cp", "/var/log/cros_ec.log", "/tmp/cros_ec.log.previous").Run(); err != nil {
		s.Fatal("Failed to save cros_ec.log: ", err)
	}
	defer func() {
		if err := testexec.CommandContext(ctx, "rm", "/tmp/cros_ec.log.previous").Run(); err != nil {
			s.Fatal("Failed to clean up cros_ec.log.previous: ", err)
		}
	}()

	if err := testexec.CommandContext(ctx, "suspend_stress_test", "-c", "1").Run(); err != nil {
		s.Fatal("Failed to execute suspend_stress_test command: ", err)
	}

	out, _ := testexec.CommandContext(ctx, "diff", "/tmp/cros_ec.log.previous", "/var/log/cros_ec.log").Output()

	// state names are in http://cs/chromeos_public/src/platform/ec/power/common.c;l=701;rcl=47ac51c53ec5f1d095139f2cecdbe19fc4b7401f
	re := regexp.MustCompile("> [^\n]+ power state \\d = S0->")
	if outs := string(out); !re.MatchString(outs) {
		s.Fatal("Did not find power state transition out of S0: ", outs)
	}
}
