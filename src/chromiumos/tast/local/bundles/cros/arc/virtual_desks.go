// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualDesks,
		Desc:         "Sample test to manipulate an app with UI automator",
		Contacts:     []string{"afakhry@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Timeout:      4 * time.Minute,
	})
}

// Returns true if a window whose name is `windowName` was found as a child of the desk container whose name is `deskContainerName`.
func deskContainsWindow(ctx context.Context, tconn *chrome.Conn, deskContainerName string, windowName string) (bool, error) {
	var found bool
	err := tconn.EvalPromise(ctx,
		fmt.Sprintf(`new Promise(function(resolve, reject) {
				chrome.automation.getDesktop(function(root) {
					// Find the given desk container first.
					const deskContainer = root.find({attributes: {className: '%[1]s'}});
					if (!deskContainer) {
						var error = "" + deskContainer.toString();
						error += "Failed to locate the given desk container: %[1]s";
						reject(new Error(error));
						return;
					}

					// Find the given window inside the desk container.
					const window = deskContainer.find({attributes: {className: '%[2]s'}});
					resolve(window != null);
				})
			})`, deskContainerName, windowName), &found)
	return found, err
}

func VirtualDesks(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--enable-features=VirtualDesks"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	ki, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}

	// Trigger the keyboard shortcut to create a new desk, which also activates
	// the newly created desk.
	ki.Accel(ctx, "Ctrl+Search+=")

	s.Log("Starting the android settings app")

	// Create a Settings activity handle.
	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	// Launch the activity.
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	// Put the activity in "normal" (non-maximized mode).
	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set window state to Normal: ", err)
	}

	assertTrue := func(condition bool, err error) {
		if !condition || err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

	assertFalse := func(condition bool, err error) {
		if condition || err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// The seetings window should exist on the second desk, while the browser window
	// should be on the first desk.
	assertTrue(deskContainsWindow(ctx, tconn, "Desk_Container_B", "ExoShellSurface"))
	assertTrue(deskContainsWindow(ctx, tconn, "Desk_Container_A", "BrowserFrame"))
	assertFalse(deskContainsWindow(ctx, tconn, "Desk_Container_A", "ExoShellSurface"))
}
