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
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HTMLVideoRoundedCornersUnderlay,
		Desc:         "Verifies that HTML <video> rounded corners are implemented with a hardware underlay",
		Contacts:     []string{"amusbach@chromium.org", "oshima@chromium.org", "chromeos-perf@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "proprietary_codecs"},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
		Data:         []string{"bear-320x240.h264.mp4", "video_with_rounded_corners.html"},
		Params: []testing.Param{{
			Fixture: "chromeGraphics",
			Val:     lacros.ChromeTypeChromeOS,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraData:         []string{launcher.DataArtifact},
			Fixture:           "chromeGraphicsLacros",
			Val:               lacros.ChromeTypeLacros,
		}},
	})
}

func HTMLVideoRoundedCornersUnderlay(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// TODO(crbug.com/1127165): Remove the artifactPath argument when we can use Data in fixtures.
	var artifactPath string
	if s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
		artifactPath = s.DataPath(launcher.DataArtifact)
	}
	cr, l, _, err := lacros.Setup(ctx, s.FixtValue(), artifactPath, s.Param().(lacros.ChromeType))
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacrosChrome(cleanupCtx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	conn, err := cr.NewConn(ctx, srv.URL+"/video_with_rounded_corners.html")
	if err != nil {
		s.Fatal("Failed to load video_with_rounded_corners.html: ", err)
	}
	defer conn.Close()

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
