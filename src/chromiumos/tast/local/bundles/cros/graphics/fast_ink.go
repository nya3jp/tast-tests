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
		Data:         []string{"d-canvas/main.html", "d-canvas/2d.js"},
		Fixture:      "chromeLoggedIn",
		Timeout:      5 * time.Minute,
	})
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

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	// The above call to ash.EnsureTabletModeEnabled may trigger a
	// notification about gesture navigation.
	if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide notifications: ", err)
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

	// The display info would be stale when we rotate the display.
	// To be safe, we limit the scope of info used to get the ID.
	var internalDisplayID string
	if info, err := display.GetInternalInfo(ctx, tconn); err == nil {
		internalDisplayID = info.ID
	} else {
		s.Fatal("Failed to get the internal display info: ", err)
	}

	if err := display.SetDisplayRotationSync(ctx, tconn, internalDisplayID, display.Rotate90); err != nil {
		s.Fatal("Failed to rotate display: ", err)
	}

	if _, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventFullscreen); err != nil {
		s.Fatal("Failed to set window state to Fullscreen: ", err)
	}

	w, err := ash.GetWindow(ctx, tconn, wID)
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}

	// Move the mouse to the center of the window, where it will not
	// bring extraneous UI stuff. Particularly, in full screen, a
	// mouse near the bottom could make the shelf visible there, or
	// a mouse near the top could make the browser frame visible
	// there. We do this step even in tablet mode, because
	// ash.HideVisibleNotifications causes the mouse cursor to show.
	if err := mouse.Move(ctx, tconn, w.TargetBounds.CenterPoint(), time.Second); err != nil {
		s.Fatal("Failed to move mouse: ", err)
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for location change events to be completed: ", err)
	}

	hists, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait a second")
		}
		return nil
	}, "Viz.DisplayCompositor.OverlayStrategy")
	if err != nil {
		s.Fatal("Error while recording histogram Viz.DisplayCompositor.OverlayStrategy: ", err)
	}

	hist := hists[0]
	if len(hist.Buckets) == 0 {
		s.Fatal("Got no overlay strategy data")
	}

	for _, bucket := range hist.Buckets {
		// bucket.Min will be from enum OverlayStrategies as defined
		// in tools/metrics/histograms/enums.xml in the chromium
		// code base. Here we check for 1 which is "No overlay."
		if bucket.Min == 1 {
			s.Fatalf("%d of %d frame(s) had no overlay", bucket.Count, hist.TotalCount())
		}
	}
}
