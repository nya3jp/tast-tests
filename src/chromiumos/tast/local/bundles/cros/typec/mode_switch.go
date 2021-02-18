// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// The maximum number of USB Type C ports that a Chromebook supports.
const maxTypeCPorts = 8

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModeSwitch,
		Desc:         "Checks USB Type C mode switch behaviour on login",
		Contacts:     []string{"pmalani@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Params: []testing.Param{
			// For running manually.
			{},
			// For automated testing.
			{
				Name:              "test",
				ExtraAttr:         []string{"group:mainline", "informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
			}},
	})
}

// ModeSwitch does the following:
// - Go to the login screen.
// - Validate USB+DP alt mode is working correctly.
// - Login.
// - Validate that TBT alt mode is working correctly.
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> Thunderbolt3 (>= Titan Ridge) dock -----> DP monitor.
//      (USB4)
//
func ModeSwitch(ctx context.Context, s *testing.State) {
	// Check if a TBT device is connected. If one isn't, we should skip
	// execution.
	// Check each port successively. If a port returns an error, that means
	// we are out of ports.
	// This check is for test executions which take place on
	// CQ (where TBT peripherals aren't connected).
	for i := 0; i < maxTypeCPorts; i++ {
		if present, err := typecutils.CheckPortForTBTPartner(ctx, i); err != nil {
			s.Log("Couldn't find TBT device from PD identity: ", err)
			return
		} else if present {
			s.Log("Found a TBT device, proceeding with test")
			break
		}
	}

	// Get to the Chrome login screen.
	cr, err := chrome.New(ctx,
		chrome.DeferLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome at login screen: ", err)
	}
	defer cr.Close(ctx)

	testConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
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

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to login: ", err)
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
}
