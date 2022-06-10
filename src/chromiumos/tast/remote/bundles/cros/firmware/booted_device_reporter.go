// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     BootedDeviceReporter,
		Desc:     "Verifies that the BootedDevice reporter identifies if the DUT was booted from a removable device",
		Contacts: []string{"cros-fw-engprod@google.com"},
		Attr:     []string{"group:firmware", "firmware_smoke"},
		Params: []testing.Param{{
			Fixture: fixture.NormalMode,
			Val:     false,
		}, {
			Name:      "rec",
			Fixture:   fixture.RecModeNoServices,
			ExtraAttr: []string{"firmware_usb"},
			Val:       true,
		}, {
			Name:    "dev",
			Fixture: fixture.DevModeGBB,
			Val:     false,
		}, {
			Name:      "usbdev",
			Fixture:   fixture.USBDevModeGBBNoServices,
			ExtraAttr: []string{"firmware_usb"},
			Val:       true,
		}},
	})
}

func BootedDeviceReporter(ctx context.Context, s *testing.State) {
	r := reporters.New(s.DUT())

	bootedFromRemovableDevice, err := r.BootedFromRemovableDevice(ctx)
	if err != nil {
		s.Fatal("Could not determine booted device type: ", err)
	}
	s.Log("Booted device is removable: ", bootedFromRemovableDevice)
	if s.Param().(bool) != bootedFromRemovableDevice {
		s.Fatalf("Failed to get correct bootedFromRemovableDevice, got %v want %v", bootedFromRemovableDevice, s.Param())
	}

}
