// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"

	"chromiumos/tast/local/bundles/cros/audio/device"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DevicePlay,
		Desc: "Checks that sound devices for playing are recognized",
		Contacts: []string{
			"cychiang@chromium.org", // Media team
			"nya@chromium.org",      // Tast port author
		},
		// TODO(b/240269271): remove "octopus" and "hatch" when b/240269271 is fixed.
		// TODO(b/240271671): remove "nocturne" when b/240271671 is fixed.
		HardwareDeps: hwdep.D(hwdep.Speaker(), hwdep.SkipOnModel("octopus", "hatch", "nocturne")),
		Attr:         []string{"group:mainline", "informational"},
	})
}

func DevicePlay(ctx context.Context, s *testing.State) {
	device.TestDeviceFiles(ctx, s, `^pcmC\d+D\d+p$`)
	device.TestALSACommand(ctx, s, "aplay")
}
