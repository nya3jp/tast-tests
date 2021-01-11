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
	helper.Service.SetProperty(ctx, shillconst.ServicePropertyAutoConnect, false)

	// Test normal Disable / Enable / Connect / Disconnect
	for i := 0; i < 3; i++ {
		if !helper.Disable() {
			s.Fatalf("Disable failed on attempt %d", i)
		}
		if !helper.Enable() {
			s.Fatalf("Enable failed on attempt %d", i)
		}
		if !helper.Connect() {
			s.Fatalf("Connect failed on attempt %d", i)
		}
		if !helper.Disconnect() {
			s.Fatalf("Disconnect failed on attempt %d", i)
		}
	}

	// Test that Connect fails while disabled.
	if !helper.Disable() {
		s.Fatal("Disable failed before Disabled tests")
	}
	for i := 0; i < 3; i++ {
		if helper.Connect() {
			s.Fatalf("Connect succeeded while Disabled on attempt %d", i)
		}
	}
}
