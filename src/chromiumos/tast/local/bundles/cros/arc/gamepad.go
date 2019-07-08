// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Gamepad,
		Desc:         "Checks gamepad support works on Android",
		Contacts:     []string{"tetsui@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func Gamepad(ctx context.Context, s *testing.State) {
	gp, err := input.Gamepad(ctx)
	if err != nil {
		s.Fatal("Failed to create a gamepad: ", err)
	}

	s.Log("Created a virtual gamepad device ", gp.Device())
	testing.Sleep(ctx, 4*time.Minute)

	defer gp.Close()
}
