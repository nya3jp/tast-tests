// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualDesks,
		Desc:         "Tests the placement of an ARC app in a virtual desk",
		Contacts:     []string{"afakhry@chromium.org", "arc-framework+tast@@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		// TODO(yusukes): Change the timeout back to 4 min when we revert arc.go's BootTimeout to 120s.
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// deskContainsWindow returns true if a window whose name is windowName was found as a child of the desk container whose name is deskContainerName.
func deskContainsWindow(ctx context.Context, tconn *chrome.TestConn, deskContainerName, windowName string) (bool, error) {
	// Find the given desk container first.
	deskContainer, err := ui.Find(ctx, tconn, ui.FindParams{ClassName: deskContainerName})
	if err != nil {
		return false, errors.Wrapf(err, "failed to locate the given desk container: %s", deskContainerName)
	}
	defer deskContainer.Release(ctx)

	// Find the given window inside the desk container.
	return deskContainer.DescendantExists(ctx, ui.FindParams{ClassName: windowName})
}

func VirtualDesks(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--enable-features=VirtualDesks"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Explicitly start a browser window to test that switching to a new desk
	// doesn't cause it to change desks.
	if conn, err := cr.NewConn(ctx, ""); err != nil {
		s.Fatal("Failed to create a Chrome window: ", err)
	} else {
		defer conn.Close()
	}

	ki, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}

	// Trigger the keyboard shortcut to create a new desk, which also activates
	// the newly created desk.
	if err := ki.Accel(ctx, "Search+Shift+="); err != nil {
		s.Fatal("Failed to send the new desk accelerator: ", err)
	}

	s.Log("Starting the android settings app")

	// Create a Settings activity handle.
	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	// Launch the activity.
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	// Put the activity in "normal" (non-maximized mode).
	if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
		s.Fatal("Failed to set window state to Normal: ", err)
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
		s.Fatal("Failed to wait for window state to become Normal: ", err)
	}

	window, err := ash.GetARCAppWindowInfo(ctx, tconn, act.PackageName())
	if err != nil {
		s.Fatal("Failed to get the window info of the ARC app: ", err)
	}
	windowName := window.Name

	s.Log("Test setup complete. Beginning to verify desk window hierarchy")

	// The settings window should exist on the second desk, while the browser window
	// should be on the first desk.
	for _, tc := range []struct {
		desk   string
		window string
		want   bool
	}{
		{
			desk:   "Desk_Container_B",
			window: windowName,
			want:   true,
		},
		{
			desk:   "Desk_Container_A",
			window: "BrowserFrame",
			want:   true,
		},
		{
			desk:   "Desk_Container_A",
			window: windowName,
			want:   false,
		},
	} {
		if found, err := deskContainsWindow(ctx, tconn, tc.desk, tc.window); err != nil {
			s.Error("deskContainsWindow Failed: ", err)
		} else if found != tc.want {
			if tc.want {
				s.Errorf("Failed to find %s under %s", tc.window, tc.desk)
			} else {
				s.Errorf("%s should not be under %s", tc.window, tc.desk)
			}
		}
	}
}
