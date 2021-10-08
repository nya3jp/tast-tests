// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModeSuspend,
		Desc:         "Checks USB Type C mode switch behaviour with a Thunderbolt dock during suspend/resume",
		Contacts:     []string{"pmalani@chromium.org", "chromeos-power@google.com"},
		Attr:         []string{"group:typec"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Data:         []string{"testcert.p12"},
		Params: []testing.Param{
			// For running manually.
			{
				Name:      "manual",
				ExtraAttr: []string{"typec_lab"},
				Val:       false,
			},
			// For automated testing.
			{
				Name:      "smoke",
				ExtraAttr: []string{"group:mainline", "informational"},
				// Only run on the devices currently supporting USB4/Thunderbolt.
				ExtraHardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
				Val:               true,
			}},
	})
}

// ModeSuspend does the following:
// - Login.
// - Validate that the Thunderbolt dock (and connected display) is enumerated correctly.
// - Enter Suspend (S0ix).
// - Validate that the system entered and exited suspend successfully.
// - Validate that the Thunderbolt dock (and connected display) is still enumerated correctly.
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> Thunderbolt3 (>= Titan Ridge) dock -----> DP monitor.
//      (USB4)
func ModeSuspend(ctx context.Context, s *testing.State) {
	// This check is for test executions which take place on
	// CQ (where TBT peripherals aren't connected).
	port, err := typecutils.CheckPortsForTBTPartner(ctx)
	if err != nil {
		s.Fatal("Failed to determine TBT device from PD identity: ", err)
	}

	// Return early for smoke testing (CQ).
	if s.Param().(bool) {
		return
	}

	if port == -1 {
		s.Fatal("No TBT device connected to DUT")
	}

	// Get to the Chrome login screen.
	loadSigninProfileOption := chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey"))
	cr, err := chrome.New(ctx, chrome.DeferLogin(), loadSigninProfileOption)
	if err != nil {
		s.Fatal("Failed to start Chrome at login screen: ", err)
	}

	testConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	if err := typecutils.EnablePeripheralDataAccess(ctx, s.DataPath("testcert.p12")); err != nil {
		s.Fatal("Failed to enable peripheral data access setting: ", err)
	}

	// Peripherals will only switch to Thunderbolt mode once the user logs in.
	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to login: ", err)
	}

	if err := typecutils.CheckTBTAndDP(ctx, testConn); err != nil {
		s.Fatal("Failed to verify TBT & DP after login: ", err)
	}

	if err := testexec.CommandContext(ctx, "powerd_dbus_suspend", "--suspend_for_sec=10").Run(); err != nil {
		s.Fatal("Failed to perform system suspend: ", err)
	}

	if err := cr.Reconnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to the Chrome session: ", err)
	}

	testConn, err = cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to re-establish the Test API connection: ", err)
	}

	if err := typecutils.CheckTBTAndDP(ctx, testConn); err != nil {
		s.Fatal("Failed to verify TBT & DP after resume: ", err)
	}
}
