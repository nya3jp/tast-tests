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
		Func: DeviceRecord,
		Desc: "Checks that sound devices for recording are recognized",
		Contacts: []string{
			"cychiang@chromium.org", // Media team
			"nya@chromium.org",      // Tast port author
		},
		HardwareDeps: hwdep.D(hwdep.Microphone()),
		Attr:         []string{"group:mainline", "informational"},
	})
}

func DeviceRecord(ctx context.Context, s *testing.State) {
	device.TestDeviceFiles(ctx, s, `^pcmC\d+D\d+c$`)
	device.TestALSACommand(ctx, s, "arecord")
}
