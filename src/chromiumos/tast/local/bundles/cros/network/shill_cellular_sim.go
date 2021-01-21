// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSim,
		Desc:     "Verifies that a Cellular Device and matching Service are present with a valid SIM card",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular"},
	})
}

func ShillCellularSim(ctx context.Context, s *testing.State) {
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper")
	}
	if properties, err := helper.Device.GetShillProperties(ctx); err != nil {
		s.Fatal("Failed to get Device properties: ", err)
	} else if simPresent, err := properties.GetBool(shillconst.DevicePropertyCellularSIMPresent); err != nil {
		s.Fatal("Failed to get SIMPresent property: ", err)
	} else if !simPresent {
		s.Fatal("SIM not present")
	}
	if _, err = helper.FindServiceForDevice(ctx); err != nil {
		s.Fatal("Failed to get Cellular Service for Device: ", err)
	}
}
