// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FastInk,
		Desc:         "Verifies that fast ink is working as evidenced by a hardware overlay",
		Contacts:     []string{"amusbach@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"d-canvas/2d.js", "d-canvas/main.html", "d-canvas/main.js", "d-canvas/sw.js", "d-canvas/webgl.js"},
		Pre:          chrome.LoggedIn(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "clamshell",
				Val:  false,
			},
			{
				Name:              "tablet",
				Val:               true,
				ExtraSoftwareDeps: []string{"tablet_mode"},
			},
		},
	})
}

// centerMouseInWindow moves the mouse to the center of the specified
// window. It is used to avoid hover bubbles, tooltips, resize
// shadows, etc., because such things may trigger the underlay overlay
// strategy. It should only be called in clamshell mode.
func centerMouseInWindow(ctx context.Context, tconn *chrome.TestConn, wID int) error {
	w, err := ash.GetWindow(ctx, tconn, wID)
	if err != nil {
		return errors.Wrap(err, "failed to get window info")
	}

	if err := mouse.Move(ctx, tconn, w.TargetBounds.CenterPoint(), time.Second); err != nil {
		return errors.Wrap(err, "failed to move mouse")
	}

	return nil
}

// expectOverlayStrategy waits for all location change events, and
// then verifies that the specified overlay strategy is in use.
func expectOverlayStrategy(ctx context.Context, s *testing.State, tconn *chrome.TestConn, expectedOverlayStrategy int64) {
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Error("Failed to wait for location change events to be completed: ", err)
	}

	hists, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait a second")
		}
		return nil
	}, "Viz.DisplayCompositor.OverlayStrategy")
	if err != nil {
		s.Error("Error while recording histogram Viz.DisplayCompositor.OverlayStrategy: ", err)
		return
	}

	buckets := hists[0].Buckets
	descriptions := []string{"No overlay", "Fullscreen", "SingleOnTop", "Underlay"}
	switch len(buckets) {
	case 0:
		s.Error("Got no overlay strategy data")

	case 1:
		if actualOverlayStrategy := buckets[0].Min; actualOverlayStrategy != expectedOverlayStrategy {
			s.Errorf("Detected overlay strategy %s; expected %s", descriptions[actualOverlayStrategy-1], descriptions[expectedOverlayStrategy-1])
		}

	default:
		s.Error("Detected different overlay strategies on different frames")
		for _, bucket := range buckets {
			s.Errorf("%d frame(s) used %s", bucket.Count, descriptions[bucket.Min-1])
		}
	}
}

// testClamshell tests fast ink for all window state types. It should
// only be called in clamshell mode.
func testClamshell(ctx context.Context, s *testing.State, tconn *chrome.TestConn, wID int) {
	s.Run(ctx, "Normal", func(ctx context.Context, s *testing.State) {
		if _, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventNormal); err != nil {
			s.Fatal("Failed to set window state to Normal: ", err)
		}

		primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the primary display info: ", err)
		}

		// Make the window reasonably large, with no part behind the shelf.
		desiredBounds := primaryInfo.WorkArea.WithInset(10, 10)
		bounds, displayID, err := ash.SetWindowBounds(ctx, tconn, wID, desiredBounds, primaryInfo.ID)
		if err != nil {
			s.Fatal("Failed to set the window bounds: ", err)
		}
		if displayID != primaryInfo.ID {
			info, err := display.FindInfo(ctx, tconn, func(info *display.Info) bool { return info.ID == displayID })
			if err != nil {
				s.Fatal("Window not on primary display after setting bounds (also failed to get info on the display where it is)")
			}
			s.Fatalf("Window on %s; tried to put it on %s", info.Name, primaryInfo.Name)
		}
		if bounds != desiredBounds {
			s.Fatalf("Window bounds are %v; tried to set them to %v", bounds, desiredBounds)
		}

		if err := centerMouseInWindow(ctx, tconn, wID); err != nil {
			s.Fatal("Failed to center mouse in window: ", err)
		}

		expectOverlayStrategy(ctx, s, tconn, 3) // SingleOnTop
	})

	s.Run(ctx, "LeftSnapped", func(ctx context.Context, s *testing.State) {
		if _, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventSnapLeft); err != nil {
			s.Fatal("Failed to set window state to LeftSnapped: ", err)
		}

		if err := centerMouseInWindow(ctx, tconn, wID); err != nil {
			s.Fatal("Failed to center mouse in window: ", err)
		}

		expectOverlayStrategy(ctx, s, tconn, 3) // SingleOnTop
	})

	s.Run(ctx, "Maximized", func(ctx context.Context, s *testing.State) {
		if _, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventMaximize); err != nil {
			s.Fatal("Failed to set window state to Maximized: ", err)
		}

		if err := centerMouseInWindow(ctx, tconn, wID); err != nil {
			s.Fatal("Failed to center mouse in window: ", err)
		}

		expectOverlayStrategy(ctx, s, tconn, 3) // SingleOnTop
	})

	s.Run(ctx, "Fullscreen", func(ctx context.Context, s *testing.State) {
		if _, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventFullscreen); err != nil {
			s.Fatal("Failed to set window state to Fullscreen: ", err)
		}

		if err := centerMouseInWindow(ctx, tconn, wID); err != nil {
			s.Fatal("Failed to center mouse in window: ", err)
		}

		expectOverlayStrategy(ctx, s, tconn, 2) // Fullscreen
	})
}

// testTablet tests fast ink for all window state types. It should
// only be called in tablet mode.
func testTablet(ctx context.Context, s *testing.State, tconn *chrome.TestConn, wID int) {
	tew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to access to the touch screen: ", err)
	}
	defer tew.Close()

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}

	if err := tew.SetRotation(-orientation.Angle); err != nil {
		s.Fatal("Failed to set rotation on the touchscreen event writer: ", err)
	}

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create a single touch writer: ", err)
	}
	defer stw.Close()

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	tcc := tew.NewTouchCoordConverter(info.Bounds.Size())
	tewW := tew.Width()
	tewH := tew.Height()
	centerX := tewW / 2
	centerY := tewH / 2

	var snapX, snapY, dividerEndX, dividerEndY input.TouchCoord
	switch orientation.Type {
	case display.OrientationPortraitPrimary:
		snapX = centerX
		snapY = tewH - 1
		dividerEndX = centerX
		dividerEndY = 0
	case display.OrientationPortraitSecondary:
		snapX = centerX
		snapY = 0
		dividerEndX = centerX
		dividerEndY = tewH - 1
	case display.OrientationLandscapePrimary:
		snapX = 0
		snapY = centerY
		dividerEndX = tewW - 1
		dividerEndY = centerY
	case display.OrientationLandscapeSecondary:
		snapX = tewW - 1
		snapY = centerY
		dividerEndX = 0
		dividerEndY = centerY
	}

	s.Run(ctx, "LeftSnapped", func(ctx context.Context, s *testing.State) {
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter overview: ", err)
		}

		w, err := ash.GetWindow(ctx, tconn, wID)
		if err != nil {
			s.Fatal("Failed to get window info: ", err)
		}

		// The window may already be snapped if a previous call to testTablet
		// successfully snapped it but then failed to unsnap it.
		if w.State != ash.WindowStateLeftSnapped {
			windowX, windowY := tcc.ConvertLocation(w.OverviewInfo.Bounds.CenterPoint())

			if err := stw.LongPressAt(ctx, windowX, windowY); err != nil {
				s.Fatal("Failed to long-press to initiate window drag from overview: ", err)
			}

			if err := stw.Swipe(ctx, windowX, windowY, snapX, snapY, time.Second); err != nil {
				s.Fatal("Failed while swiping window from overview to snap: ", err)
			}

			if err := stw.End(); err != nil {
				s.Fatal("Failed to end the window swipe from overview to snap: ", err)
			}
		}

		expectOverlayStrategy(ctx, s, tconn, 3) // SingleOnTop
	})

	// Clean up before further tests.
	w, err := ash.GetWindow(ctx, tconn, wID)
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}
	if w.State == ash.WindowStateLeftSnapped {
		if err := stw.Swipe(ctx, centerX, centerY, dividerEndX, dividerEndY, time.Second); err != nil {
			s.Fatal("Failed while swiping split view divider to end split view: ", err)
		}

		if err := stw.End(); err != nil {
			s.Fatal("Failed to end the split view divider swipe to end split view: ", err)
		}
	} else if w.OverviewInfo != nil {
		if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			s.Fatal("Failed to exit overview: ", err)
		}
	}
	if w.State == ash.WindowStateFullscreen {
		if _, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventMaximize); err != nil {
			s.Fatal("Failed to set window state to Maximized: ", err)
		}
	}

	s.Run(ctx, "Maximized", func(ctx context.Context, s *testing.State) {
		expectOverlayStrategy(ctx, s, tconn, 3) // SingleOnTop
	})

	s.Run(ctx, "Fullscreen", func(ctx context.Context, s *testing.State) {
		if _, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventFullscreen); err != nil {
			s.Fatal("Failed to set window state to Fullscreen: ", err)
		}

		expectOverlayStrategy(ctx, s, tconn, 2) // Fullscreen
	})
}

func FastInk(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	tabletMode := s.Param().(bool)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide notifications: ", err)
	}

	// The display info would be stale when we rotate the display.
	// To be safe, we limit the scope of info used to get the ID.
	var primaryDisplayID string
	{
		info, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the primary display info: ", err)
		}
		primaryDisplayID = info.ID
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	conn, err := cr.NewConn(ctx, srv.URL+"/d-canvas/main.html")
	if err != nil {
		s.Fatal("Failed to load d-canvas/main.html: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for d-canvas/main.html to achieve quiescence: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get windows: ", err)
	}

	if wsCount := len(ws); wsCount != 1 {
		s.Fatal("Expected 1 window; found ", wsCount)
	}

	wID := ws[0].ID

	var test func(context.Context, *testing.State, *chrome.TestConn, int)
	if tabletMode {
		test = testTablet
	} else {
		test = testClamshell
	}

	defer display.SetDisplayRotationSync(cleanupCtx, tconn, primaryDisplayID, display.Rotate0)
	for _, displayRotation := range []display.RotationAngle{display.Rotate0, display.Rotate90, display.Rotate180, display.Rotate270} {
		s.Run(ctx, string(displayRotation), func(ctx context.Context, s *testing.State) {
			if err := display.SetDisplayRotationSync(ctx, tconn, primaryDisplayID, displayRotation); err != nil {
				s.Fatal("Failed to rotate display: ", err)
			}

			test(ctx, s, tconn, wID)
		})
	}
}
