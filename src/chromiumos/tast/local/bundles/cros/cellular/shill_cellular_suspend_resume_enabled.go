// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularSuspendResumeEnabled,
		Desc: "Verifies that cellular maintains enabled state around Suspend/Resume",
		Contacts: []string{
			"danielwinkler@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture: "cellular",
		Timeout: 2 * time.Minute,
	})
}

func ShillCellularSuspendResumeEnabled(ctx context.Context, s *testing.State) {
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Verify powered and service available
	err = helper.Enable(ctx)
	if err != nil {
		s.Fatal("Failed to enable modem: ", err)
	}
	_, err = helper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatal("Unable to find Cellular Service for Device: ", err)
	}

	// Request suspend for 10 seconds
	if _, err := helper.Suspend(ctx, 10*time.Second, true /* checkEarlyWake */); err != nil {
		s.Fatal("Failed to suspend: ", err)
	}

	// Verify powered persisted and service available
	if err := helper.Device.WaitForProperty(ctx, shillconst.DevicePropertyPowered, true, shillconst.DefaultTimeout); err != nil {
		s.Fatal("Expected powered to become true, got false")
	}
	_, err = helper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatal("Unable to find Cellular Service for Device: ", err)
	}
}
