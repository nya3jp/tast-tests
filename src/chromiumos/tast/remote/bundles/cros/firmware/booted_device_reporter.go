// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootedDeviceReporter,
		Desc:         "Verifies that the BootedDevice reporter identifies which device mode the DUT was booted from",
		Contacts:     []string{"cros-fw-engprod@google.com"},
		SoftwareDeps: []string{"crossystem"},
		Attr:         []string{"group:firmware", "firmware_smoke"},
		Data:         []string{firmware.ConfigFile},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		VarDeps:      []string{"servo"},
		Params: []testing.Param{{
			Pre:       pre.NormalMode(),
			ExtraAttr: []string{"firmware_smoke"},
		}, {
			Name:      "rec",
			Pre:       pre.RecMode(),
			ExtraAttr: []string{"firmware_smoke", "firmware_usb"},
		}, {
			Name: "dev",
			Pre:  pre.DevMode(),
			// TODO(gredelston): Reenable when b/183044117 is resolved
			// ExtraAttr: []string{"firmware_smoke"},
		}},
	})
}

func BootedDeviceReporter(ctx context.Context, s *testing.State) {
	r := reporters.New(s.DUT())

	bootType, err := r.BootedDevice(ctx)
	if err != nil {
		s.Fatal("Could not determine booted device type: ", err)
	}
	s.Log("Booted device type is: ", bootType)
}
