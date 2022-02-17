// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package phonehub

import (
	"context"

	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/crossdevice/phonehub"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhoneStatus,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that Phone Hub displays the phone's battery and signal levels",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
			"chromeos-cross-device-eng@google.com",
		},
		Attr:         []string{"group:cross-device"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceOnboardedAllFeatures",
	})
}

// PhoneStatus tests that Phone Hub accurately displays the phone's battery and signal levels.
func PhoneStatus(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice
	if err := phonehub.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Phone Hub: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Phones in the lab don't have a SIM.
	ui := uiauto.New(tconn)
	if err := ui.Exists(nodewith.Role(role.Image).NameContaining("No SIM"))(ctx); err != nil {
		s.Fatal("Failed to check SIM status: ", err)
	}

	// Get the battery level from the UI.
	uiLevel, err := phonehub.BatteryLevel(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get battery level from the UI: ", err)
	}

	// Get the phone's battery level to compare.
	phoneLevel, err := androidDevice.BatteryLevel(ctx)
	if err != nil {
		s.Fatal("Failed to get battery level from adb: ", err)
	}

	if uiLevel != phoneLevel {
		s.Fatalf("Phone Hub battery level (%v) does not match level reported by ADB (%v)", uiLevel, phoneLevel)
	}
}
