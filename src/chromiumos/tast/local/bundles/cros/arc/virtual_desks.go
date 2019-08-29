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
		Func:     VirtualDesks,
		Desc:     "Tests the placement of an ARC app in a virtual desk",
		Contacts: []string{"afakhry@chromium.org", "arc-framework+tast@@google.com"},
		Attr:     []string{"informational"},
		// TODO(ricadoq): add support for Android NYC once https://crbug.com/989595 gets fixed.
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
	})
}

// deskContainsWindow returns true if a window whose name is windowName was found as a child of the desk container whose name is deskContainerName.
func deskContainsWindow(ctx context.Context, tconn *chrome.Conn, deskContainerName, windowName string) (bool, error) {
	var found bool
	err := tconn.EvalPromise(ctx,
		fmt.Sprintf(
			`new Promise(function(resolve, reject) {
			  chrome.automation.getDesktop(function(root) {
			    // Find the given desk container first.
			    const deskContainer = root.find({attributes: {className: '%[1]s'}});
			    if (!deskContainer) {
			      reject(new Error("Failed to locate the given desk container: %[1]s"));
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

	if conn, err := cr.NewConn(ctx, ""); err != nil {
		s.Fatal("Failed to create a chrome window: ", err)
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
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	// Put the activity in "normal" (non-maximized mode).
	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set window state to Normal: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

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
			window: "ExoShellSurface",
			want:   true,
		},
		{
			desk:   "Desk_Container_A",
			window: "BrowserFrame",
			want:   true,
		},
		{
			desk:   "Desk_Container_A",
			window: "ExoShellSurface",
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
