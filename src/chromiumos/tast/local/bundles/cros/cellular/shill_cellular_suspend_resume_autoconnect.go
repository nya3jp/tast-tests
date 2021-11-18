// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type autoconnectTestParams struct {
	autoconnectState bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularSuspendResumeAutoconnect,
		Desc: "Verifies that cellular maintains autoconnect state around Suspend/Resume",
		Contacts: []string{
			"danielwinkler@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture:      "cellular",
		Timeout:      2 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "enabled",
			Val: autoconnectTestParams{
				autoconnectState: true,
			},
		}, {
			Name: "disabled",
			Val: autoconnectTestParams{
				autoconnectState: false,
			},
		}},
	})
}

func ShillCellularSuspendResumeAutoconnect(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	params := s.Param().(autoconnectTestParams)

	expectedStates := map[bool]string{true: shillconst.ServiceStateOnline,
		false: shillconst.ServiceStateIdle}

	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Disable Ethernet and/or WiFi if present and defer re-enabling.
	// Shill documentation shows that autoconnect will only be used if there
	// is no other service available, so it is necessary to only have
	// cellular available.
	if enableFunc, err := helper.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet); err != nil {
		s.Fatal("Unable to disable Ethernet: ", err)
	} else if enableFunc != nil {
		newCtx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
		defer cancel()
		defer enableFunc(ctx)
		ctx = newCtx
	}
	if enableFunc, err := helper.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyWifi); err != nil {
		s.Fatal("Unable to disable Wifi: ", err)
	} else if enableFunc != nil {
		newCtx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
		defer cancel()
		defer enableFunc(ctx)
		ctx = newCtx
	}

	// Enable and get service to set autoconnect based on test parameters.
	if _, err := helper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable modem: ", err)
	}

	if _, err := helper.SetServiceAutoConnect(ctx, params.autoconnectState); err != nil {
		s.Fatal("Failed to enable AutoConnect: ", err)
	}

	// Request suspend for 10 seconds.
	if err := testexec.CommandContext(ctx, "powerd_dbus_suspend", "--suspend_for_sec=10").Run(); err != nil {
		s.Fatal("Failed to perform system suspend: ", err)
	}

	// The reconnection will not occur from the login screen, so we log in.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)
	// chrome.Chrome.Close() will not log the user out.
	defer upstart.RestartJob(ctx, "ui")

	service, err := helper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatal("Unable to find Cellular Service for Device: ", err)
	}

	// Ensure service's state matches expectations.
	if err := service.WaitForProperty(ctx, shillconst.ServicePropertyState, expectedStates[params.autoconnectState], 60*time.Second); err != nil {
		s.Fatal("Failed to get service state: ", err)
	}
}
