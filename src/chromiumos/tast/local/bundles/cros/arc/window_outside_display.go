// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowOutsideDisplay,
		Desc:         "Checks an ARC window can move outside the display",
		Contacts:     []string{"tetsui@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome", "android"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          arc.Booted(),
	})
}

func WindowOutsideDisplay(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	a := p.ARC

	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create the settings activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the settings activity: ", err)
	}
	// defer act.Stop(ctx)

	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set the window state to normal: ", err)
	}

	if err := act.ResizeWindow(ctx, arc.BorderBottomRight, coords.Point{500, 500}, 500*time.Millisecond); err != nil {
		s.Fatal("Failed to resize the activity: ", err)
	}

	d, err := arc.NewDisplay(a, arc.DefaultDisplayID)
	if err != nil {
		s.Fatal("Could not create a new Display: ", err)
	}
	defer d.Close()
	sz, err := d.Size(ctx)
	if err != nil {
		s.Fatal("Failed to get the display size: ", err)
	}
	w, h := sz.Width, sz.Height

	dsts := []coords.Point{{-100, 0}, {w - 400, 0}, {w - 400, h - 400}, {-100, h - 400}}
	for _, dst := range dsts {
		before, err := act.WindowBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get the window bounds: ", err)
		}

		if err := act.MoveWindow(ctx, dst, 1*time.Second); err != nil {
			s.Fatal("Failed to move the activity: ", err)
		}

		after, err := act.WindowBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get the window bounds: ", err)
		}

		if before.Width != after.Width || before.Height != after.Height {
			s.Fatalf("window size changed from (%d, %d) to (%d, %d)", before.Width, before.Height, after.Width, after.Height)
		}
	}
}
