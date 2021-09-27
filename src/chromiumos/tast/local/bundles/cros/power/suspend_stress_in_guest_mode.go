// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

var (
	powerItrVar = testing.RegisterVarString(
		"power.SuspendStressInGuestMode.itr",
		"2",
		"It takes number of iterations as input")
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SuspendStressInGuestMode,
		Desc:         "Suspend Stress test memory check in GuestMode",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"power.SuspendStressInGuestMode.itr"},
		Timeout:      5 * time.Hour,
	})
}

// SuspendStressInGuestMode Suspend Stress test memory check in GuestMode.
func SuspendStressInGuestMode(ctx context.Context, s *testing.State) {
	var (
		SuspendFailurePattern   = regexp.MustCompile("Suspend failures: 0")
		FirmwareLogErrorPattern = regexp.MustCompile("Firmware log errors: 0")
		S0ixErrorPattern        = regexp.MustCompile("s0ix errors: 0")
	)
	s.Log("Iterations: ", powerItrVar.Value())
	opts := []chrome.Option{chrome.GuestLogin(), chrome.KeepState()}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	out, err := testexec.CommandContext(ctx, "suspend_stress_test", "--memory_check", "--suspend_min=10", "--suspend_max=10", "-c", powerItrVar.Value()).Output()
	if err != nil {
		s.Fatal("Failed to execute suspend_stress_test command: ", err)
	}
	errorCodes := []*regexp.Regexp{SuspendFailurePattern, FirmwareLogErrorPattern, S0ixErrorPattern}
	for _, errmsg := range errorCodes {
		if !errmsg.MatchString(string(out)) {
			s.Fatalf("Failed %q failures are non-zero", errmsg)
		}
	}
}
