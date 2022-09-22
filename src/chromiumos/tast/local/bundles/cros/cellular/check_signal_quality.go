// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckSignalQuality,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that host has signal quality above threshold via cellular interface",
		Contacts:     []string{"nmarupaka@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture:      "cellular",
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
	})
}

// CheckSignalQuality needs to be run to verify that the DUT has sufficient signal coverage to execute other network related test cases
func CheckSignalQuality(ctx context.Context, s *testing.State) {
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	service, err := helper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatal("Unable to find Cellular Service for Device: ", err)
	}

	// Ensure service's state matches expectations.
	if err := service.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 150*time.Second); err != nil {
		s.Fatal("Failed to get service state: ", err)
	}

	// Ensure service's signal quality matches expectations.
	signalStrength, _ := service.GetSignalStrength(ctx)
	testing.ContextLog(ctx, "SignalStrength: ", signalStrength)
	if signalStrength < shillconst.CellularServiceMinSignalStrength {
		s.Fatalf("Signal strength below minimum acceptable threshold - %d < %d ", signalStrength, shillconst.CellularServiceMinSignalStrength)
	}
}
