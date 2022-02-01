// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MojoCellularToggle,
		Desc:         "Enable/disable Cellular service using Mojo and confirms using shill",
		Contacts:     []string{"shijinabraham@google.com", "cros-network-health@google.com", "chromeos-cellular-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable"},
		Timeout:      10 * time.Minute,
		Fixture:      "cellular",
	})
}

// MojoCellularToggle enables/distable cellular network using Mojo and confirms using shill helper
func MojoCellularToggle(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed start Chrome: ", err)
	}
	defer cr.Close(ctx)

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	_, err = helper.Enable(ctx)
	if err != nil {
		s.Fatal("Failed to enable cellular: ", err)
	}

	netConn, err := netconfig.NewCrosNetworkConfig(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network Mojo Object: ", err)
	}

	const iterations = 5
	for i := 0; i < iterations; i++ {
		var isEnabled bool
		if i%2 == 0 {
			isEnabled = false
		} else {
			isEnabled = true
		}

		s.Logf("Toggling Cellular state to %t (iteration %d of %d)", isEnabled, i+1, iterations)

		if err = netConn.SetNetworkTypeEnabledState(ctx, netconfig.Cellular, isEnabled); err != nil {
			s.Fatal("Failed to set cellular state: ", err)
		}

		if err = helper.WaitForEnabledState(ctx, isEnabled); err != nil {
			s.Fatal("cellular state is not as expected: ", err)
		}
	}
}
