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
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModeCrash,
		Desc:         "Checks USB Type C mode switch behaviour when typecd crashes",
		Contacts:     []string{"pmalani@chromium.org", "chromeos-power@google.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Params: []testing.Param{
			// For running manually.
			{
				Name: "manual",
				Val:  false,
			},
			// For automated testing.
			{
				Name:              "smoke",
				ExtraAttr:         []string{"group:mainline", "informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
				Val:               true,
			}},
	})
}

// ModeCrash does the following:
// - Login.
// - Validate that TBT alt mode is working correctly.
// - Kill typecd.
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
	// This check is for test executions which take place on
	// CQ (where TBT peripherals aren't connected).
	present, err := typecutils.CheckPortsForTBTPartner(ctx)
	if err != nil {
		s.Fatal("Couldn't determine TBT device from PD identity: ", err)
	}

	// Return early for smoke testing (CQ).
	if smoke := s.Param().(bool); smoke {
		return
	}

	if !present {
		s.Fatal("No TBT device connected to DUT")
	}

	loadSignInProfileOption := chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey"))
	cr, err := chrome.New(ctx, loadSignInProfileOption)
	if err != nil {
		s.Fatal("Failed to start Chrome with sign-in: ", err)
	}

	testConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}
	defer testConn.Close()

	s.Log("Verifying that TBT device enumerated correctly")
	tbtPollOptions := testing.PollOptions{Timeout: 10 * time.Second}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return typecutils.CheckTBTDevice(true)
	}, &tbtPollOptions); err != nil {
		s.Fatal("Failed to verify TBT devices connected after login: ", err)
	}

	s.Log("Verifying that DP monitor enumerated correctly")
	dpPollOptions := testing.PollOptions{Timeout: 20 * time.Second}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return typecutils.FindConnectedDPMonitor(ctx, testConn)
	}, &dpPollOptions); err != nil {
		s.Fatal("Failed to verify DP monitor working after login: ", err)
	}

	s.Log("Killing typecd")
	if err := testexec.CommandContext(ctx, "pkill", "typecd").Run(); err != nil {
		s.Fatal("Failed to kill typecd: ", err)
	}

	// Wait for 2 seconds for typecd to restart.
	// TODO(b/181617978): We wait for 2 seconds, to give time for typecd to:
	// a. Restart.
	// b. Re-build Type C state.
	// Come up with a better way to determine that typecd rebuilt its state.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to wait for typecd to restart: ", err)
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
	if err := cr.Close(ctx); err != nil {
		s.Fatal("Failed to close Chrome: ", err)
	}

	cr, err = chrome.New(ctx,
		chrome.DeferLogin(),
		chrome.KeepState(),
		loadSignInProfileOption)
	if err != nil {
		s.Fatal("Failed to start Chrome with login: ", err)
	}
	defer cr.Close(ctx)

	testConn, err = cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	s.Log("Verifying that no TBT devices enumerated")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return typecutils.CheckTBTDevice(false)
	}, &tbtPollOptions); err != nil {
		s.Fatal("Failed to verify no TBT devices connected at login screen: ", err)
	}

	s.Log("Verifying that DP monitor enumerated correctly")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return typecutils.FindConnectedDPMonitor(ctx, testConn)
	}, &dpPollOptions); err != nil {
		s.Fatal("Failed to verify DP monitor working at login screen: ", err)
	}
}
