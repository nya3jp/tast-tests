// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package coex

import (
	"context"

	"chromiumos/tast/local/bundles/cros/coex/phy_toggle"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BluetoothPhyToggle,
		Desc: "Toggles Bluetooth setting from the login screen",
		Contacts: []string{
			"billyzhao@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"coex.signinProfileTestExtensionManifestKey"},
	})
}

func BluetoothPhyToggle(ctx context.Context, s *testing.State) {

	req := s.RequiredVar("coex.signinProfileTestExtensionManifestKey")
	defer phy_toggle.BringIfUp(ctx, req)

	if err := phy_toggle.AssertIfUp(ctx); err != nil {
		s.Fatal("Failed to assert network interfaces are up: ", err)
	}
	if err := phy_toggle.ChangeBluetooth(ctx, "on", req); err != nil {
		s.Fatal("Test failed with reason: ", err)
	}
}
