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
		Func:         DeviceRecord,
		Desc:         "Checks that sound devices for recording are recognized",
		SoftwareDeps: []string{"audio_record"},
	})
}

func DeviceRecord(s *testing.State) {
	device.TestDeviceFiles(s, `^pcmC\d+D\d+c$`)
	device.TestALSACommand(s, "arecord")
}
