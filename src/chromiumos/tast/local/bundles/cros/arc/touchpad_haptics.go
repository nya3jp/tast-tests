// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
		Func:         TouchpadHaptics,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks haptic feedback APIs on Android",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      8 * time.Minute,
	})
}

func TouchpadHaptics(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	trackpad, err := input.HapticTrackpad(ctx)
	if err != nil {
		s.Fatal(err, "failed to set up the trackpad with haptic feedback")
	}
	defer trackpad.Close()

	const (
		apk          = "ArcTouchpadHapticsTest.apk"
		pkg          = "org.chromium.arc.testapp.touchpad"
		activityName = ".MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q: %v", activityName, err)
	}

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity:")
	}
}
