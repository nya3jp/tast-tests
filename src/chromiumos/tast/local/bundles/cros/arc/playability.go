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
	a := s.PreValue().(arc.PreData).ARC

	cr := s.PreValue().(arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := a.Install(ctx, arc.APKPath(motioninput.APK)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	act, err := arc.NewActivity(a, motioninput.Package, motioninput.EventReportingActivity)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Error starting activity: ", err)
	}

	if err := ash.WaitForVisible(ctx, tconn, motioninput.Package); err != nil {
		s.Fatal("Failed to wait for activity to become visible: ", err)
	}

	ws, err := ash.SetARCAppWindowState(ctx, tconn, motioninput.Package, ash.WMEventMaximize)
	if err != nil {
		s.Fatal("Failed to set window state: ", err)
	} else if ws != ash.WindowStateMaximized {
		s.Fatalf("Window state incorrect. Expected %s got %s", ash.WindowStateMaximized, ws)
	}

	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Error stopping activity: ", err)
	}
}
