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
		Func: ShillCellularSuspendResumePersistEnabled,
		Desc: "Verifies that cellular maintains enabled state around Suspend/Resume",
		Contacts: []string{
			"danielwinkler@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture: "cellular",
		Timeout: 2 * time.Minute,
		Params: []testing.Param{{
			Name: "enabled",
			Val:  true,
		}, {
			Name: "disabled",
			Val:  false,
		}},
	})
}

func ShillCellularSuspendResumePersistEnabled(ctx context.Context, s *testing.State) {
	enabledState := s.Param().(bool)

	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Apply required enabled state
	if enabledState {
		err = helper.Enable(ctx)
	} else {
		err = helper.Disable(ctx)
	}

	if err != nil {
		s.Fatal("Failed to set initial enabled setting to ", enabledState, ": ", err)
	}

	// Request suspend for 10 seconds
	if _, err := helper.Suspend(ctx, 10*time.Second, true /* checkEarlyWake */); err != nil {
		s.Fatal("Failed to suspend: ", err)
	}

	// Verify enabled setting persisted
	if err := helper.Device.WaitForProperty(ctx, shillconst.DevicePropertyPowered, enabledState, shillconst.DefaultTimeout); err != nil {
		s.Fatal("Failed to set enabled to ", enabledState, ": ", err)
	}

	// Return to enabled state and confirm service available
	if err := helper.Enable(ctx); err != nil {
		s.Fatal("Failed to re-enable modem after resume: ", err)
	}

	if _, err := helper.FindServiceForDevice(ctx); err != nil {
		s.Fatal("Unable to find Cellular Service after resume: ", err)
	}
}
