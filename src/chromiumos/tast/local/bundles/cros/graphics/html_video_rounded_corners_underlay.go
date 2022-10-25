// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HTMLVideoRoundedCornersUnderlay,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verifies that HTML <video> rounded corners are implemented with a hardware underlay",
		Contacts:     []string{"amusbach@chromium.org", "oshima@chromium.org", "chromeos-perf@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "proprietary_codecs"},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
		Data:         []string{"180p_60fps_600frames.h264.mp4", "video_with_rounded_corners.html"},
		Params: []testing.Param{{
			// TODO(b/246573749): Remove cave and chell when the test can pass on them.
			// TODO(b/255636769): Remove rusty and steelix when the test can pass on them.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("cave", "chell", "rusty", "steelix")),
			Fixture:           "chromeGraphics",
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			// TODO(b/246573749): Remove cave and chell when the test can pass on them.
			// TODO(b/255636769): Remove rusty and steelix when the test can pass on them.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("cave", "chell", "rusty", "steelix")),
			Fixture:           "chromeGraphicsLacros",
			Val:               browser.TypeLacros,
		}, {
			Name: "failing",
			// TODO(b/246573749): Remove cave and chell when the test can pass on them.
			// TODO(b/255636769): Remove rusty and steelix when the test can pass on them.
			ExtraHardwareDeps: hwdep.D(hwdep.Model("cave", "chell", "rusty", "steelix")),
			Fixture:           "chromeGraphics",
			Val:               browser.TypeAsh,
		}, {
			Name:              "failing_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			// TODO(b/246573749): Remove cave and chell when the test can pass on them.
			// TODO(b/255636769): Remove rusty and steelix when the test can pass on them.
			ExtraHardwareDeps: hwdep.D(hwdep.Model("cave", "chell", "rusty", "steelix")),
			Fixture:           "chromeGraphicsLacros",
			Val:               browser.TypeLacros,
		}},
	})
}

func HTMLVideoRoundedCornersUnderlay(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(cleanupCtx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	conn, err := cs.NewConn(ctx, srv.URL+"/video_with_rounded_corners.html")
	if err != nil {
		s.Fatal("Failed to load video_with_rounded_corners.html: ", err)
	}
	defer conn.Close()

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get windows: ", err)
	}

	// Verify that there is one and only one window.
	if wsCount := len(ws); wsCount != 1 {
		s.Fatal("Expected 1 window; found ", wsCount)
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, ws[0].ID, ash.WindowStateMaximized); err != nil {
		s.Fatal("Failed to maximize window: ", err)
	}

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for video_with_rounded_corners.html to achieve quiescence: ", err)
	}

	hists, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait a second")
		}
		return nil
	}, "Viz.DisplayCompositor.OverlayStrategy")
	if err != nil {
		s.Fatal("Failed to record histogram Viz.DisplayCompositor.OverlayStrategy: ", err)
	}

	hist := hists[0]
	if len(hist.Buckets) == 0 {
		s.Fatal("Got no overlay strategy data")
	}

	for _, bucket := range hist.Buckets {
		// bucket.Min will be from enum OverlayStrategies as defined
		// in tools/metrics/histograms/enums.xml in the chromium
		// code base. 1 is "No overlay", and 4 is "Underlay".
		if bucket.Min != 1 && bucket.Min != 4 {
			s.Errorf("Found %d frame(s) with an unexpected overlay strategy: got %d; want 1 or 4", bucket.Count, bucket.Min)
		}
	}
}
