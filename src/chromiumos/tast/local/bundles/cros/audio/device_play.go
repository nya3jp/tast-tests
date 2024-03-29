// Copyright 2018 The ChromiumOS Authors
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
		HardwareDeps: hwdep.D(hwdep.Speaker()),
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"audio_stable"},
			// TODO(b/244254621) : remove "sasukette" when b/244254621 is fixed.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("sasukette")),
		}, {
			Name:              "unstable_platform",
			ExtraSoftwareDeps: []string{"audio_unstable"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "unstable_model",
			ExtraHardwareDeps: hwdep.D(hwdep.Model("sasukette")),
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func DevicePlay(ctx context.Context, s *testing.State) {
	device.TestDeviceFiles(ctx, s, `^pcmC\d+D\d+p$`)
	if err := device.TestALSACommand(ctx, "aplay"); err != nil {
		s.Fatal("aplay failed: ", err)
	}
}
