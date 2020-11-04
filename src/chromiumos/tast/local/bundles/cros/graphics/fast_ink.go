// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FastInk,
		Desc:         "Verifies that fast ink is working as evidenced by a hardware overlay",
		Contacts:     []string{"amusbach@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays(), hwdep.InternalDisplay(), hwdep.TouchScreen()),
		Data:         []string{"d-canvas/2d.js", "d-canvas/main.html", "d-canvas/main.js", "d-canvas/sw.js", "d-canvas/webgl.js"},
		Fixture:      "chromeLoggedIn",
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "clamshell",
			},
		},
	})
}

// expectOverlay moves the mouse to the center of the specified
// window, waits for all location change events to complete,
// and then verifies that a hardware overlay is used.
func expectOverlay(ctx context.Context, tconn *chrome.TestConn, wID int) error {
	w, err := ash.GetWindow(ctx, tconn, wID)
	if err != nil {
		return errors.Wrap(err, "failed to get window info")
	}

	if err := mouse.Move(ctx, tconn, w.TargetBounds.CenterPoint(), time.Second); err != nil {
		return errors.Wrap(err, "failed to move mouse")
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location change events to be completed")
	}

	hists, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait a second")
		}
		return nil
	}, "Viz.DisplayCompositor.OverlayStrategy")
	if err != nil {
		return errors.Wrap(err, "error while recording histogram Viz.DisplayCompositor.OverlayStrategy")
	}

	buckets := hists[0].Buckets
	if len(buckets) == 0 {
		return errors.New("got no overlay strategy data")
	}

	framesWithoutOverlay := int64(0)
	totalFrames := int64(0)
	for _, bucket := range buckets {
		if bucket.Min == int64(1) {
			framesWithoutOverlay = bucket.Count
		}
		totalFrames += bucket.Count
	}
	if framesWithoutOverlay != int64(0) {
		return errors.Errorf("%d of %d frame(s) had no overlay", framesWithoutOverlay, totalFrames)
	}

	return nil
}

func FastInk(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide notifications: ", err)
	}

	// The display info would be stale when we rotate the display.
	// To be safe, we limit the scope of info used to get the ID.
	var internalDisplayID string
	{
		info, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the internal display info: ", err)
		}
		internalDisplayID = info.ID
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

	defer display.SetDisplayRotationSync(cleanupCtx, tconn, internalDisplayID, display.Rotate0)
	for _, displayRotation := range []display.RotationAngle{display.Rotate0, display.Rotate90, display.Rotate180, display.Rotate270} {
		s.Run(ctx, string(displayRotation), func(ctx context.Context, s *testing.State) {
			if err := display.SetDisplayRotationSync(ctx, tconn, internalDisplayID, displayRotation); err != nil {
				s.Fatal("Failed to rotate display: ", err)
			}

			s.Run(ctx, "Normal", func(ctx context.Context, s *testing.State) {
				if _, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventNormal); err != nil {
					s.Fatal("Failed to set window state to Normal: ", err)
				}

				internalInfo, err := display.GetInternalInfo(ctx, tconn)
				if err != nil {
					s.Fatal("Failed to get the internal display info: ", err)
				}

				// Make the window reasonably large, with no part behind the shelf.
				desiredBounds := internalInfo.WorkArea.WithInset(10, 10)
				bounds, displayID, err := ash.SetWindowBounds(ctx, tconn, wID, desiredBounds, internalInfo.ID)
				if err != nil {
					s.Fatal("Failed to set the window bounds: ", err)
				}
				if displayID != internalInfo.ID {
					info, err := display.FindInfo(ctx, tconn, func(info *display.Info) bool { return info.ID == displayID })
					if err != nil {
						s.Fatal("Window not on internal display after setting bounds (also failed to get info on the display where it is)")
					}
					s.Fatalf("Window on %s; tried to put it on %s", info.Name, internalInfo.Name)
				}
				if bounds != desiredBounds {
					s.Fatalf("Window bounds are %v; tried to set them to %v", bounds, desiredBounds)
				}

				if err := expectOverlay(ctx, tconn, wID); err != nil {
					s.Fatal("Failed to verify that an overlay is used: ", err)
				}
			})

			s.Run(ctx, "Maximized", func(ctx context.Context, s *testing.State) {
				if _, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventMaximize); err != nil {
					s.Fatal("Failed to set window state to Maximized: ", err)
				}

				if err := expectOverlay(ctx, tconn, wID); err != nil {
					s.Fatal("Failed to verify that an overlay is used: ", err)
				}
			})

			s.Run(ctx, "Fullscreen", func(ctx context.Context, s *testing.State) {
				if _, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventFullscreen); err != nil {
					s.Fatal("Failed to set window state to Fullscreen: ", err)
				}

				if err := expectOverlay(ctx, tconn, wID); err != nil {
					s.Fatal("Failed to verify that an overlay is used: ", err)
				}
			})
		})
	}
}
