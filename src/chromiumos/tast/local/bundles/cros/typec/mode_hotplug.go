// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModeHotplug,
		Desc:         "Checks USB Type C mode switch behaviour when a Thunderbolt dock is unplugged/replugged",
		Contacts:     []string{"pmalani@chromium.org", "chromeos-power@google.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Data:         []string{"testcert.p12"},
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

// ModeHotplug does the following:
// - Login.
// - Validate that the Thunderbolt dock is enumerated correctly.
// - Simulate unplug of the dock.
// - Validate that the Thunderbolt dock is no longer enumerated.
// - Simulate re-plug of the dock.
// - Validate that the Thunderbolt dock is re-enumerated correctly.
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> Thunderbolt3 (>= Titan Ridge) dock -----> DP monitor.
//      (USB4)
//
// The Thunderbolt dock is assumed connected on port index 1.
func ModeHotplug(ctx context.Context, s *testing.State) {
	// This check is for test executions which take place on
	// CQ (where TBT peripherals aren't connected).
	present, err := typecutils.CheckPortsForTBTPartner(ctx)
	if err != nil {
		s.Fatal("Failed to determine TBT device from PD identity: ", err)
	}

	// Return early for smoke testing (CQ).
	if smoke := s.Param().(bool); smoke {
		return
	}

	if !present {
		s.Fatal("No TBT device connected to DUT")
	}

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Get to the Chrome login screen.
	loadSignInProfileOption := chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey"))
	cr, err := chrome.New(ctx, chrome.DeferLogin(), loadSignInProfileOption)
	if err != nil {
		s.Fatal("Failed to start Chrome at login screen: ", err)
	}

	testConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}
	defer testConn.Close()

	if err := typecutils.EnablePeripheralDataAccess(ctx, s.DataPath("testcert.p12")); err != nil {
		s.Fatal("Failed to enable peripheral data access setting: ", err)
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to login: ", err)
	}

	if err := checkTBTAndDP(ctx, testConn); err != nil {
		s.Fatal("Failed to verify TBT & DP after login: ", err)
	}

	if err := testexec.CommandContext(ctx, "ectool", "pdcontrol", "suspend", "1").Run(); err != nil {
		s.Fatal("Failed to simulate unplug: ", err)
	}
	defer func() {
		if err := testexec.CommandContext(ctxForCleanUp, "ectool", "pdcontrol", "resume", "1").Run(); err != nil {
			s.Error("Failed to perform replug: ", err)
		}
	}()

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return typecutils.CheckTBTDevice(false)
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to verify no TBT devices connected after unplug: ", err)
	}

	if err := testexec.CommandContext(ctx, "ectool", "pdcontrol", "resume", "1").Run(); err != nil {
		s.Fatal("Failed to simulate replug: ", err)
	}

	if err := checkTBTAndDP(ctx, testConn); err != nil {
		s.Fatal("Failed to verify TBT & DP after replug: ", err)
	}
}

// checkTBTAndDP is a convenience function that checks for TBT and DP enumeration.
// It returns nil on success, and the relevant error otherwise.
func checkTBTAndDP(ctx context.Context, tc *chrome.TestConn) error {
	tbtPollOptions := testing.PollOptions{Timeout: 10 * time.Second}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return typecutils.CheckTBTDevice(true)
	}, &tbtPollOptions); err != nil {
		return errors.Wrap(err, "failed to verify TBT devices connected")
	}

	dpPollOptions := testing.PollOptions{Timeout: 20 * time.Second}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return typecutils.FindConnectedDPMonitor(ctx, tc)
	}, &dpPollOptions); err != nil {
		return errors.Wrap(err, "failed to verify DP monitor working")
	}

	return nil
}
