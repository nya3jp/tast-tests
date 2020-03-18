// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CheckServoKeyPresses,
		Desc:     "Verifies that key presses can be initiated on the servo's keyboard emulator and that the DUT can receive them",
		Contacts: []string{"kmshelton@chromium.org", "cros-fw-engprod@google.com", "chromeos-firmware@google.com"},
		Attr:     []string{"disabled", "informational"},
	})
}

func ServoEcho(ctx context.Context, s *testing.State) {
	svo, err := servo.Default(ctx)
	if err != nil {
		s.Fatal("Servo init error: ", err)
	}

	//TODO read from /dev/input/event* and confirm that the keys sent by the servo are seen by the event device interface on the DUT.  might need to stop UI to avoid Chrome intercepts.

	svo.CtrlD()
	svo.EnterKey()
}
