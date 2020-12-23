// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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
		Fixture:      "arcBooted",
	})
}

// Playability tests whether a build is "playable". It tests whether we can open an app,
// install apks, and change window state.
func Playability(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanupCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := a.Install(ctx, arc.APKPath(motioninput.APK)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
	defer a.Uninstall(cleanupCtx, motioninput.Package)

	act, err := arc.NewActivity(a, motioninput.Package, motioninput.EventReportingActivity)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Error starting activity: ", err)
	}
	defer func(ctx context.Context) {
		if err := act.Stop(cleanupCtx, tconn); err != nil {
			s.Error("Error stopping activity: ", err)
		}
	}(cleanupCtx)

	if err := ash.WaitForVisible(ctx, tconn, motioninput.Package); err != nil {
		s.Fatal("Failed to wait for activity to become visible: ", err)
	}

	ws, err := ash.SetARCAppWindowState(ctx, tconn, motioninput.Package, ash.WMEventMaximize)
	if err != nil {
		s.Fatal("Failed to set window state: ", err)
	} else if ws != ash.WindowStateMaximized {
		s.Fatalf("Unexpected window state. Want %s got %s", ash.WindowStateMaximized, ws)
	}
}
