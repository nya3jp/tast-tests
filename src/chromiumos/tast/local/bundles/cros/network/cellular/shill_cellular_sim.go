// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"chromiumos/tast/common/shillconst"
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

// ShillCellularSim Test
func ShillCellularSim(ctx context.Context, s *testing.State) {
	helper, err := NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create Helper")
	}
	if properties, err := helper.Device.GetProperties(ctx); err != nil {
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
