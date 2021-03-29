// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/ui"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromePIPRoundedCornersUnderlay,
		Desc:         "Verifies that Chrome PIP rounded corners are implemented with a hardware underlay",
		Contacts:     []string{"amusbach@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "proprietary_codecs"},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
		Data:         []string{"bear-320x240.h264.mp4", "pip_video.html"},
	})
}

func ChromePIPRoundedCornersUnderlay(ctx context.Context, s *testing.State) {
	// Reserve one minute for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=PipRoundedCorners"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	conn, err := cr.NewConn(ctx, srv.URL+"/pip_video.html")
	if err != nil {
		s.Fatal("Failed to load pip_video.html: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for pip_video.html to achieve quiescence: ", err)
	}

	var pipButtonCenterString string
	if err := conn.Call(ctx, &pipButtonCenterString, "getPIPButtonCenter"); err != nil {
		s.Fatal("Failed to get center of PIP button: ", err)
	}

	var pipButtonCenterInWebContents coords.Point
	if n, err := fmt.Sscanf(pipButtonCenterString, "%v,%v", &pipButtonCenterInWebContents.X, &pipButtonCenterInWebContents.Y); err != nil {
		s.Fatalf("Failed to parse center of PIP button (successfully parsed %v of 2 tokens): %v", n, err)
	}

	webContentsView, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: "WebContentsViewAura"})
	if err != nil {
		s.Fatal("Failed to get web contents view: ", err)
	}
	defer webContentsView.Release(cleanupCtx)

	if err := mouse.Click(ctx, tconn, webContentsView.Location.TopLeft().Add(pipButtonCenterInWebContents), mouse.LeftButton); err != nil {
		s.Fatal("Failed to click PIP button: ", err)
	}

	if err := chromeui.WaitUntilExists(ctx, tconn, chromeui.FindParams{Name: "Picture in picture", ClassName: "PictureInPictureWindow"}, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for PIP window: ", err)
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
