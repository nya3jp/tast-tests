// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"chromiumos/tast/local/bundles/cros/audio/device"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DevicePlay,
		Desc:         "Checks that sound devices for playing are recognized",
		SoftwareDeps: []string{"audio_play"},
	})
}

func DevicePlay(s *testing.State) {
	device.TestDeviceFiles(s, `^pcmC\d+D\d+p$`)
	device.TestALSACommand(s, "aplay")
}
