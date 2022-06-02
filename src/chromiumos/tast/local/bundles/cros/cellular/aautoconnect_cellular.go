// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AautoconnectCellular,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that host has network connectivity via cellular interface",
		Contacts:     []string{"pholla@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_sim_active"},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
	})
}

// AautoconnectCellular starts with Aa so that this test is scheduled before other tests. This prevents the test from being affected by other tests. It also mimics user experience. Being scheduled first is not necessary, but MAY help improve pass rate.
func AautoconnectCellular(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Set up the device to autoconnect and disable any roaming restrictions.
	cleanup1, err := helper.InitServiceProperty(ctx, shillconst.ServicePropertyAutoConnect, true)
	if err != nil {
		s.Fatal("Could not initialize autoconnect to true: ", err)
	}
	defer cleanup1(cleanupCtx)
	cleanup2, err := helper.InitDeviceProperty(ctx, shillconst.DevicePropertyCellularPolicyAllowRoaming, true)
	if err != nil {
		s.Fatal("Could not set PolicyAllowRoaming to true: ", err)
	}
	defer cleanup2(cleanupCtx)
	cleanup3, err := helper.InitServiceProperty(ctx, shillconst.ServicePropertyCellularAllowRoaming, true)
	if err != nil {
		s.Fatal("Could not set AllowRoaming property to true: ", err)
	}
	defer cleanup3(cleanupCtx)

	// The connection will not occur from the login screen, so we log in.
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
	if err := service.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 150*time.Second); err != nil {
		s.Fatal("Failed to get service state: ", err)
	}
}
