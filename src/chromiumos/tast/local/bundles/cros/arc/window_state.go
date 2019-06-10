// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WindowState,
		Desc:     "Checks that ARC++ applications correctly change the window state",
		Contacts: []string{"phshah@chromium.org", "arc-eng@google.com"},
		Attr:     []string{"informational"},
		// Adding 'tablet_mode' since moving/resizing the window requires screen touch support.
		SoftwareDeps: []string{"android_p", "chrome", "tablet_mode"},
		Timeout:      4 * time.Minute,
	})
}

func WindowState(ctx context.Context, s *testing.State) {
	// Force Chrome to be in clamshell mode, where windows are resizable.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Start ARC++
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Start the Settings app
	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	for i := 0; i < 50; i++ {
		changeActivityToMaximizeWindowState(ctx, s, act)
		changeActivityToMinimizeWindowState(ctx, s, act)
	}

	for i := 0; i < 50; i++ {
		changeActivityToNormalWindowState(ctx, s, act)
		changeActivityToMinimizeWindowState(ctx, s, act)
	}
}

func changeActivityToMinimizeWindowState(ctx context.Context, s *testing.State, act *arc.Activity) {
	if err := act.SetWindowState(ctx, arc.WindowStateMinimized); err != nil {
		s.Fatal("Failed to set window state to Minimized: ", err)
	}
	windowState, err := act.GetWindowState(ctx)
	if err != nil {
		s.Fatal("Failed to get the window state: ", err)
	}
	if windowState == "minimized" {
		s.Fatal("Received incorrect window state. Expected: minimized Actual: " + windowState)
	}
}

func changeActivityToMaximizeWindowState(ctx context.Context, s *testing.State, act *arc.Activity) {
	if err := act.SetWindowState(ctx, arc.WindowStateMaximized); err != nil {
		s.Fatal("Failed to set window state to Maximized: ", err)
	}
	windowState, err := act.GetWindowState(ctx)
	if err != nil {
		s.Fatal("Failed to get the window state: ", err)
	}
	if windowState == "maximized" {
		s.Fatal("Received incorrect window state. Expected: maximized Actual: " + windowState)
	}
}

func changeActivityToNormalWindowState(ctx context.Context, s *testing.State, act *arc.Activity) {
	if err := act.SetWindowState(ctx, arc.WindowStateMaximized); err != nil {
		s.Fatal("Failed to set window state to Maximized: ", err)
	}
	windowState, err := act.GetWindowState(ctx)
	if err != nil {
		s.Fatal("Failed to get the window state: ", err)
	}
	if windowState == "normal" {
		s.Fatal("Received incorrect window state. Expected: normal Actual: " + windowState)
	}
}
