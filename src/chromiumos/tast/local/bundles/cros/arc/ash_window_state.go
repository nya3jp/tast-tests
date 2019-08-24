// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AshWindowState,
		Desc:         "Checks that sending Ash WM event will change ARC app window state correctly",
		Contacts:     []string{"xdai@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(),
	})
}

func AshWindowState(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Restore tablet mode to its original state on exit.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	// Force Chrome to be in clamshell mode to test window state related functionalities.
	// TODO(xdai): Test window state functionalities in tablet mode as well.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to disable tablet mode: ", err)
	}

	// Start the Settings app.
	const pkg = "com.android.settings"
	act, err := arc.NewActivity(a, pkg, ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the Settings activity: ", err)
	}

	for _, test := range []struct {
		wmEvent             ash.WMEventType
		expectedWindowState ash.WindowStateType
	}{
		{ash.WMEventNormal, ash.WindowStateNormal},
		{ash.WMEventMaximize, ash.WindowStateMaximized},
		{ash.WMEventMinimize, ash.WindowStateMinimized},
		{ash.WMEventFullscreen, ash.WindowStateFullscreen},
		{ash.WMEventSnapLeft, ash.WindowStateLeftSnapped},
		{ash.WMEventSnapRight, ash.WindowStateRightSnapped},
	} {
		s.Logf("Sending event %s to Settings app", test.wmEvent)

		if state, err := ash.SetARCAppWindowState(ctx, tconn, pkg, test.wmEvent); err != nil {
			s.Errorf("Failed to set window state to %s for Settings app", test.expectedWindowState)
		} else if state != test.expectedWindowState {
			s.Errorf("Unexpected window state: got %s; want %s", state, test.expectedWindowState)
		}
	}
}
