// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillManagerSanity,
		Desc:     "Verifies that Shill is running and the Manager interface returns expected results",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func ShillManagerSanity(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create Manager object")
	}
	ethernetEnabled, err := manager.IsEnabled(ctx, shill.TechnologyEthernet)
	if !ethernetEnabled {
		s.Fatal("Ethernet not enabled")
	}
}
