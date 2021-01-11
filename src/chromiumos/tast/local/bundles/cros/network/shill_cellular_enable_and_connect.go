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
		Func:     ShillCellularEnableAndConnect,
		Desc:     "Verifies that Shill can enable, disable, connect, and disconnect to a Cellular Service",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular"},
	})
}

func ShillCellularEnableAndConnect(ctx context.Context, s *testing.State) {
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Disable AutoConnect so that enable does not connect.
	service, err := helper.FindServiceForDevice()
	if err != nil {
		s.Fatal("Failed to get Cellular Service: ", err)
	}
	service.SetProperty(ctx, shillconst.ServicePropertyAutoConnect, false)

	// Test normal Disable / Enable / Connect / Disconnect
	for i := 0; i < 3; i++ {
		s.Log("Disable")
		if err := helper.Disable(); err != nil {
			s.Fatalf("Disable failed on attempt %d, err: %s", i, err)
		}
		s.Log("Enable")
		if err := helper.Enable(); err != nil {
			s.Fatalf("Enable failed on attempt %d, err: %s", i, err)
		}
		s.Log("Connect")
		if err := helper.Connect(); err != nil {
			s.Fatalf("Connect failed on attempt %d, err: %s", i, err)
		}
		s.Log("Disconnect")
		if err := helper.Disconnect(); err != nil {
			s.Fatalf("Disconnect failed on attempt %d, err: %s", i, err)
		}
	}

	// Test that Connect fails while disabled.
	if err := helper.Disable(); err != nil {
		s.Fatal("Disable failed before running Disabled tests: ", err)
	}
	if err := service.Connect(ctx); err == nil {
		s.Fatal("Connect succeeded while Disabled: ", err)
	}
}
