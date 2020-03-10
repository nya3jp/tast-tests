// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowOutsideDisplay,
		Desc:         "Ensures an ARC window can move outside the display",
		Contacts:     []string{"tetsui@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               []string{},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               []string{"--enable-arcvm"},
		}},
	})
}

func WindowOutsideDisplay(ctx context.Context, s *testing.State) {
	args := s.Param().([]string)
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=clamshell"), chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	disp, err := arc.NewDisplay(a, arc.DefaultDisplayID)
	if err != nil {
		s.Fatal("Could not create a new Display: ", err)
	}
	defer disp.Close()
	sz, err := disp.Size(ctx)
	if err != nil {
		s.Fatal("Failed to get the display size: ", err)
	}

	w, h := sz.Width, sz.Height

	const (
		pkg          = "com.android.settings"
		activityName = ".Settings"
		swipeDur     = time.Second
	)

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatal("Failed to create the settings activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the settings activity: ", err)
	}
	defer act.Stop(ctx)

	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set the window state to normal: ", err)
	}

	if err := act.MoveWindow(ctx, coords.Point{0, 0}, swipeDur); err != nil {
		s.Fatal("Failed to move the activity: ", err)
	}

	if err := act.ResizeWindow(ctx, arc.BorderBottomRight, coords.Point{w / 2, h / 2}, swipeDur); err != nil {
		s.Fatal("Failed to resize the activity: ", err)
	}

	outset := w / 8

	left := -outset
	top := 0
	right := w/2 + outset
	bottom := h/2 + outset

	for _, origin := range []coords.Point{{left, top}, {right, top}, {right, bottom}, {left, bottom}} {
		before, err := act.WindowBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get the window bounds: ", err)
		}

		if err := act.MoveWindow(ctx, origin, swipeDur); err != nil {
			s.Fatal("Failed to move the activity: ", err)
		}

		after, err := act.WindowBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get the window bounds: ", err)
		}

		if before.Size() != after.Size() {
			s.Fatalf("Unexpected window size change: got %v, want %v", after.Size(), before.Size())
		}
	}
}
