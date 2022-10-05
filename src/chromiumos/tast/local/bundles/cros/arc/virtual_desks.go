// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualDesks,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests the placement of an ARC app in a virtual desk",
		Contacts:     []string{"afakhry@chromium.org", "arc-framework+tast@@google.com", "chromeos-wmp@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "lacrosWithArcBooted",
			Val:               browser.TypeLacros,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros_vm",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "lacrosWithArcBooted",
			Val:               browser.TypeLacros,
		}},
	})
}

// deskContainsWindow returns true if a window whose name is windowName was found as a child of the desk container whose name is deskContainerName.
func deskContainsWindow(ctx context.Context, tconn *chrome.TestConn, deskContainerName string, finder *nodewith.Finder) (bool, error) {
	// Find the given desk container first.
	deskContainer := nodewith.HasClass(deskContainerName)
	ui := uiauto.New(tconn)
	if err := ui.Exists(deskContainer)(ctx); err != nil {
		return false, errors.Wrapf(err, "failed to locate the given desk container: %s", deskContainerName)
	}

	// Find the given finder inside the desk container.
	return ui.IsNodeFound(ctx, finder.Ancestor(deskContainer))
}

func VirtualDesks(ctx context.Context, s *testing.State) {
	// Reserve few seconds for various cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Explicitly start a browser window to test that switching to a new desk
	// doesn't cause it to change desks.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	conn, err := br.NewConn(ctx, "about:blank")
	if err != nil {
		s.Fatal("Could not open the browser window: ", err)
	}
	defer conn.Close()

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
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
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
	windowFinder := nodewith.HasClass(window.Name).Role(role.Window)

	s.Log("Test setup complete. Beginning to verify desk window hierarchy")

	// The settings window should exist on the second desk, while the browser window
	// should be on the first desk.
	for _, tc := range []struct {
		desk   string
		finder *nodewith.Finder
		want   bool
	}{
		{
			desk:   "Desk_Container_B",
			finder: windowFinder,
			want:   true,
		},
		{
			desk:   "Desk_Container_A",
			finder: nodewith.NameContaining("about:blank").First(),
			want:   true,
		},
		{
			desk:   "Desk_Container_A",
			finder: windowFinder,
			want:   false,
		},
	} {
		if found, err := deskContainsWindow(ctx, tconn, tc.desk, tc.finder); err != nil {
			s.Error("deskContainsWindow Failed: ", err)
		} else if found != tc.want {
			if tc.want {
				s.Errorf("Failed to find %s under %s", tc.finder.Pretty(), tc.desk)
			} else {
				s.Errorf("%s should not be under %s", tc.finder.Pretty(), tc.desk)
			}
		}
	}
}
