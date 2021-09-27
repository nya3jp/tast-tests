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

func init() {
	testing.AddTest(&testing.Test{
		Func:         SuspendStressInGuestMode,
		Desc:         "Suspend Stress test memory check in GuestMode",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"power.boardtypeIter"},
		Timeout:      5 * time.Hour,
	})
}

// SuspendStressInGuestMode Suspend Stress test memory check in GuestMode.
func SuspendStressInGuestMode(ctx context.Context, s *testing.State) {
	var (
		SuspndFailurePattern  = regexp.MustCompile("Suspend failures: 0")
		FrmwreLogErrorPattern = regexp.MustCompile("Firmware log errors: 0")
		S0ixErrorPattern      = regexp.MustCompile("s0ix errors: 0")
	)
	defaultIter := "2"
	newIter, ok := s.Var("power.boardtypeIter")
	if ok {
		defaultIter = newIter
		s.Log("Iteration value: ", defaultIter)
	}
	var opts []chrome.Option
	opts = append(opts, chrome.GuestLogin())
	opts = append(opts, chrome.KeepState())
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	out, err := testexec.CommandContext(ctx, "suspend_stress_test", "--memory_check", "--suspend_min=10", "--suspend_max=10", "-c", defaultIter).Output()
	if err != nil {
		s.Fatal("Failed to execute suspend_stress_test command: ", err)
	}
	var errorCodes []*regexp.Regexp
	errorCodes = []*regexp.Regexp{SuspndFailurePattern, FrmwreLogErrorPattern, S0ixErrorPattern}
	for _, errmsg := range errorCodes {
		if !(errmsg).MatchString(string(out)) {
			s.Fatalf("Failed %q failures are non-zero", errmsg)
		}
	}
}
