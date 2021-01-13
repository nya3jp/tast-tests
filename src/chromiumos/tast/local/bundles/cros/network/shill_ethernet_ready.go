// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillEthernetReady,
		Desc:     "Verifies that Shill is running and an Ethernet Device and Service is available",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func ShillEthernetReady(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create Manager object: ", err)
	}

	if ethernetAvailable, err := manager.IsAvailable(ctx, shill.TechnologyEthernet); err != nil {
		s.Fatal("Error calling IsAvailable: ", err)
	} else if !ethernetAvailable {
		s.Fatal("Ethernet not available")
	}

	if ethernetEnabled, err := manager.IsEnabled(ctx, shill.TechnologyEthernet); err != nil {
		s.Fatal("Error calling IsEnabled: ", err)
	} else if !ethernetEnabled {
		s.Fatal("Ethernet not enabled")
	}

	if _, err := manager.DeviceByType(ctx, shillconst.TypeEthernet); err != nil {
		s.Fatal("No Ethernet Device: ", err)
	}

	props := map[string]interface{}{
		shillconst.ServicePropertyType: shillconst.TypeEthernet,
	}
	if _, err := manager.FindMatchingService(ctx, props); err != nil {
		s.Fatal("No Ethernet Service: ", err)
	}
}
