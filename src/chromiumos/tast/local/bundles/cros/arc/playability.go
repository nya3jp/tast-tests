// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/motioninput"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Playability,
		Desc:         "Checks basic playability like opening an app, installing apks, and changing window state",
		Contacts:     []string{"richardfung@google.com", "arc-next@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Pre:          arc.Booted(),
	})
}

// Playability tests whether a build is "playable". It tests whether we can open an app,
// install apks, and change window state.
func Playability(ctx context.Context, s *testing.State) {
	ark := s.PreValue().(arc.PreData).ARC

	cr := s.PreValue().(arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	settingsActivity, err := arc.NewActivity(ark, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create Settings activity: ", err)
	}
	defer settingsActivity.Close()

	if err := settingsActivity.Start(ctx, tconn); err != nil {
		s.Fatal("Error starting Settings activity: ", err)
	}

	if err := settingsActivity.Stop(ctx, tconn); err != nil {
		s.Fatal("Error stopping Settings activity: ", err)
	}

	if err := ark.Install(ctx, arc.APKPath(motioninput.APK)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	act, err := arc.NewActivity(ark, motioninput.Package, motioninput.EventReportingActivity)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting app")
	if err = act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	defer act.Stop(ctx, tconn)

	if err := ash.WaitForVisible(ctx, tconn, motioninput.Package); err != nil {
		s.Fatal("Failed to wait for activity to become visible: ", err)
	}

	ws, err := ash.SetARCAppWindowState(ctx, tconn, motioninput.Package, ash.WMEventMaximize)
	if err != nil {
		s.Fatal("Failed to set window state: ", err)
	} else if ws != ash.WindowStateMaximized {
		s.Fatalf("Window state incorrect. Expected %s got %s", ash.WindowStateMaximized, ws)
	}

	//	TODO(crbug/1093518): Enable after SetWindowBounds works.
	//	window, err := ash.GetARCAppWindowInfo(ctx, tconn, motioninput.Package)
	//	if err != nil {
	//		s.Fatal("Failed to get window info: ", err)
	//	}
	//
	//	_, _, err = ash.SetWindowBounds(ctx, tconn, window.ID, coords.NewRect(0, 0, 100, 200), window.DisplayID)
	//	if err != nil {
	//		s.Fatal("Failed to set window bounds: ", err)
	//	}

	//	TODO(b/152576355): Enable these after dumpsys works in R.
	//	if err = act.ResizeWindow(ctx, arc.BorderTop, coords.NewPoint(15, 233), 3*time.Second); err != nil {
	//		s.Fatal("Failed to resize window: ", err)
	//	}
	//
	//	if err = act.MoveWindow(ctx, coords.NewPoint(0, 100), 5 *time.Second); err != nil {
	//		s.Fatal("Failed to move window: ", err)
	//	}

	// TODO(159464182): Check that input works.
}
