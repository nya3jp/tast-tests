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
	properties, err := manager.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Manager properties from shill")
	}
	technologies, err := properties.GetStrings(shillconst.ManagerPropertyEnabledTechnologies)
	if err != nil {
		s.Fatal("Failed go get enabled technologies property")
	}
	hasEthernet := false
	for _, t := range technologies {
		if t == string(shill.TechnologyEthernet) {
			hasEthernet = true
			break
		}
	}
	if !hasEthernet {
		s.Fatal("Ethernet not enabled")
	}
}
