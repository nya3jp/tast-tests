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
		Func:     ShillCellularSanity,
		Desc:     "Verifies that Shill is running and a Cellular Device and Service are present",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular"},
	})
}

func ShillCellularSanity(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create Manager object")
	}
	managerProperties, err := manager.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Manager properties from shill")
	}
	technologies, err := managerProperties.GetStrings(shillconst.ManagerPropertyEnabledTechnologies)
	if err != nil {
		s.Fatal("Failed go get enabled technologies property")
	}
	hasCellular := false
	for _, t := range technologies {
		if t == string(shill.TechnologyCellular) {
			hasCellular = true
			break
		}
	}
	if !hasCellular {
		s.Fatal("Cellular not enabled")
	}
	cellularDevice, err := manager.DeviceByType(ctx, shillconst.TypeCellular)
	if err != nil || cellularDevice == nil {
		s.Fatal("Failed to get Cellular Device")
	}

	cellularProps := map[string]interface{}{
		shillconst.ServicePropertyConnectable: true,
		shillconst.ServicePropertyType:        shillconst.TypeCellular,
	}
	cellularService, err := manager.FindMatchingService(ctx, cellularProps)
	if err != nil || cellularService == nil {
		s.Fatal("Failed to get Cellular Service")
	}
}
