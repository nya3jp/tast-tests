// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

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

// ShillCellularEnableAndConnect Test
func ShillCellularEnableAndConnect(ctx context.Context, s *testing.State) {
	helper, err := NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create Helper: ", err)
	}

	// Disable AutoConnect so that enable does not connect.
	if wasAutoConnect, err := helper.SetServiceAutoConnect(ctx, false); err != nil {
		s.Fatal("Failed to disable AutoConnect: ", err)
	} else if wasAutoConnect {
		defer func() {
			if _, err := helper.SetServiceAutoConnect(ctx, true); err != nil {
				s.Fatal("Failed to enable AutoConnect: ", err)
			}
		}()
	}

	// Test Disable / Enable / Connect / Disconnect.
	// Run the test a second time to test Disable after Connect/Disconnect.
	// Run the test a third time to help test against flakiness.
	for i := 0; i < 3; i++ {
		s.Logf("Disable %d", i)
		if err := helper.Disable(ctx); err != nil {
			s.Fatalf("Disable failed on attempt %d: %s", i, err)
		}
		s.Logf("Enable %d", i)
		if err := helper.Enable(ctx); err != nil {
			s.Fatalf("Enable failed on attempt %d: %s", i, err)
		}
		s.Logf("Connect %d", i)
		if err := helper.Connect(ctx); err != nil {
			s.Fatalf("Connect failed on attempt %d: %s", i, err)
		}
		s.Logf("Disconnect %d", i)
		if err := helper.Disconnect(ctx); err != nil {
			s.Fatalf("Disconnect failed on attempt %d: %s", i, err)
		}
	}

	// Test that Disconnect fails while not connected.
	if err := helper.Disconnect(ctx); err == nil {
		s.Fatal("Disconnect succeeded while disconnected: ", err)
	}

	s.Log("Reconnect")
	if err := helper.Connect(ctx); err != nil {
		s.Fatal("Reconnect failed: ", err)
	}

	// Test Disable while connected.
	s.Log("Disable")
	if err := helper.Disable(ctx); err != nil {
		s.Fatal("Disable failed while connected: ", err)
	}

	s.Log("Connect while disabled")
	if err := helper.Connect(ctx); err == nil {
		s.Fatal("Connect succeeded while Disabled: ", err)
	}

	s.Log("Disconnect while disabled")
	if err := helper.Disconnect(ctx); err == nil {
		s.Fatal("Disconnect succeeded while Disabled: ", err)
	}

	s.Log("Final Enable")
	if err := helper.Enable(ctx); err != nil {
		s.Fatal("Final Enable failed: ", err)
	}
}
