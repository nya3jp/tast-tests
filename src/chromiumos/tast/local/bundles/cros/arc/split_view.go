// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SplitView,
		Desc:         "Tests split view with ARC",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
	})
}

func SplitView(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view", "--enable-virtual-keyboard"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	if err := a.Command(ctx, "am", "start", "-W", "com.android.storagemanager/.deletionhelper.DeletionHelperActivity").Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	if err := a.Command(ctx, "am", "start", "-W", "com.android.settings/.Settings").Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	testing.Sleep(ctx, 2*time.Second)

	if err := ash.SnapARCAppInSplitView(ctx, tconn, "com.android.settings", ash.SnapPositionLeft); err != nil {
		s.Fatal("Failed to snap app in split view: ", err)
	}

	testing.Sleep(ctx, 2*time.Second)

	if err := ash.SnapARCAppInSplitView(ctx, tconn, "com.android.storagemanager", ash.SnapPositionRight); err != nil {
		s.Fatal("Failed to snap app in split view: ", err)
	}

	testing.Sleep(ctx, 2*time.Second)

	if state, err := ash.GetARCAppWindowState(ctx, tconn, "com.android.settings"); err != nil {
		s.Fatal("Failed to get ash window state: ", err)
	} else if state != ash.WindowStateLeftSnapped {
		s.Fatal("Invalid window state: ", state)
	}

	if state, err := ash.GetARCAppWindowState(ctx, tconn, "com.android.storagemanager"); err != nil {
		s.Fatal("Failed to get ash window state: ", err)
	} else if state != ash.WindowStateRightSnapped {
		s.Fatal("Invalid window state: ", state)
	}

	if err := ash.SwapWindowsInSplitView(ctx, tconn); err != nil {
		s.Fatal("Failed to swap windows in split view: ", err)
	}

	testing.Sleep(ctx, 2*time.Second)

	if state, err := ash.GetARCAppWindowState(ctx, tconn, "com.android.settings"); err != nil {
		s.Fatal("Failed to get ash window state: ", err)
	} else if state != ash.WindowStateRightSnapped {
		s.Fatal("Invalid window state: ", state)
	}

	if state, err := ash.GetARCAppWindowState(ctx, tconn, "com.android.storagemanager"); err != nil {
		s.Fatal("Failed to get ash window state: ", err)
	} else if state != ash.WindowStateLeftSnapped {
		s.Fatal("Invalid window state: ", state)
	}

}
