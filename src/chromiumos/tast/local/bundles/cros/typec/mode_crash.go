// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModeCrash,
		Desc:         "Checks USB Type C mode switch behaviour when typecd crashes",
		Contacts:     []string{"pmalani@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

// ModeCrash does the following:
// - Login.
// - Validate that TBT alt mode is working correctly.
// - Kill typecd
// - Logout.
// - Validate that the dock in in USB+Dp alt mode.
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> Thunderbolt3 (>= Titan Ridge) dock -----> DP monitor.
//      (USB4)
//
func ModeCrash(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome with login: ", err)
	}

	testConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	s.Log("Verifying that TBT device enumerated correctly")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return typecutils.CheckTBTDevice(true)
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 10 * time.Second})
	if err != nil {
		s.Fatal("Failed to verify TBT devices connected after login: ", err)
	}

	s.Log("Verifying that DP monitor enumerated correctly")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return typecutils.FindConnectedDPMonitor(ctx, testConn)
	}, &testing.PollOptions{Interval: 200 * time.Millisecond, Timeout: 20 * time.Second})
	if err != nil {
		s.Fatal("Failed to verify DP monitor working after login: ", err)
	}

	s.Log("Killing typecd")
	if err := testexec.CommandContext(ctx, "pkill", "typecd").Run(); err != nil {
		s.Fatal("Failed to kill typecd: ", err)
	}

	// Wait for 2 seconds for typecd to restart.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Sleep after typecd kill failed: ", err)
	}

	s.Log("Checking that typecd restarted")
	out, err := testexec.CommandContext(ctx, "pgrep", "typecd").Output()
	if err != nil {
		s.Fatal("Failed to run pgrep to check typecd restart: ", err)
	}

	// A valid PID is sufficient for us to know typecd is running again.
	if pid, err := strconv.Atoi(strings.TrimSpace(string(out))); err != nil {
		s.Fatal("Failed to convert pgrep output: ", err)
	} else if pid < 0 {
		s.Fatalf("typecd doesn't have a valid PID on restart: %d", pid)
	}

	// Log out;
	cr.Close(ctx)
	cr, err = chrome.New(ctx,
		chrome.DeferLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome with login: ", err)
	}
	defer cr.Close(ctx)

	testConn, err = cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get pre-login Test API connection: ", err)
	}

	s.Log("Verifying that no TBT devices enumerated")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return typecutils.CheckTBTDevice(false)
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 10 * time.Second})
	if err != nil {
		s.Fatal("Failed to verify no TBT devices connected at login screen: ", err)
	}

	s.Log("Verifying that DP monitor enumerated correctly")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return typecutils.FindConnectedDPMonitor(ctx, testConn)
	}, &testing.PollOptions{Interval: 200 * time.Millisecond, Timeout: 20 * time.Second})
	if err != nil {
		s.Fatal("Failed to verify DP monitor working at login screen: ", err)
	}
}
