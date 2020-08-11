// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package coex

import (
	"context"

	"chromiumos/tast/local/bundles/cros/coex/phytoggle"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BluetoothPhyToggle,
		Desc: "Toggles Bluetooth setting from the login screen and restores the setting",
		Contacts: []string{
			"billyzhao@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"coex.signinProfileTestExtensionManifestKey"},
	})
}

// BluetoothPhyToggle verifies bluetooth ui toggling flow
func BluetoothPhyToggle(ctx context.Context, s *testing.State) {

	req := s.RequiredVar("coex.signinProfileTestExtensionManifestKey")
	defer phytoggle.BringPhysUp(ctx, req)

	if err := phytoggle.AssertPhysUp(ctx); err != nil {
		s.Fatal("Failed to assert network interfaces are up: ", err)
	}

	if err := phytoggle.ChangeBluetooth(ctx, "on", req); err != nil {
		s.Fatal("Test failed with reason: ", err)
	}

	if res, err := phytoggle.BluetoothStatus(ctx); err != nil {
		s.Fatal("Test failed with reason: ", err)
	} else if res {
		s.Fatal("Bluetooth was not turned off")
	}

}
