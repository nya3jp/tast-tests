// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WorkArea,
		Desc:         "Checks work area update",
		Contacts:     []string{"tetsui@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android"},
		Pre:          arc.Booted(),
	})
}

func WorkArea(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	a := p.ARC
	cr := p.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	defer act.Stop(ctx)

	if err := act.SetWindowState(ctx, arc.WindowStateMaximized); err != nil {
		s.Fatal("Failed to set the window state to normal: ", err)
	}

	if err := tconn.Eval(ctx, `chrome.settingsPrivate.setPref('ash.docked_magnifier.enabled', true);`, nil); err != nil {
		s.Fatal("Failed: ", err)
	}
	testing.Sleep(ctx, 10*time.Second)
	if err := tconn.Eval(ctx, `chrome.settingsPrivate.setPref('ash.docked_magnifier.enabled', false);`, nil); err != nil {
		s.Fatal("Failed: ", err)
	}
	testing.Sleep(ctx, 10*time.Second)

	dispInfo, _ := display.GetInternalInfo(ctx, tconn)

	for _, alignment := range []ash.ShelfAlignment{ash.ShelfAlignmentLeft, ash.ShelfAlignmentRight, ash.ShelfAlignmentBottom} {
		if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, alignment); err != nil {
			s.Fatal("Failed: ", err)
		}
		testing.Sleep(ctx, 5*time.Second)
	}
}
