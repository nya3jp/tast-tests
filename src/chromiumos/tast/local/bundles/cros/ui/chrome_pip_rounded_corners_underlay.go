// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const histName = "Viz.DisplayCompositor.OverlayStrategy"

// Values of the enum OverlayStrategies defined in
// tools/metrics/histograms/enums.xml in the chromium code base.
const (
	overlayStrategyNoOverlay = 1
	overlayStrategyUnderlay  = 4
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromePIPRoundedCornersUnderlay,
		Desc:         "Verifies that Chrome PIP rounded corners are implemented with a hardware underlay",
		Contacts:     []string{"amusbach@chromium.org", "oshima@chromium.org", "chromeos-perf@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "proprietary_codecs"},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
		Data:         []string{"bear-320x240.h264.mp4", "pip_video.html"},
		Fixture:      "gpuWatchDog",
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

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

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

	ac := uiauto.New(tconn)
	pipButton := nodewith.Name("PIP").Role(role.Button)
	pipWindow := nodewith.Name("Picture in picture").ClassName("PictureInPictureWindow")
	if err := action.Combine(
		"click PIP button and wait for PIP window",
		ac.LeftClick(pipButton),
		ac.WithTimeout(10*time.Second).WaitUntilExists(pipWindow),
	)(ctx); err != nil {
		s.Fatal("Failed to show the PIP window: ", err)
	}

	initialHist, err := metrics.GetHistogram(ctx, tconn, histName)
	if err != nil {
		s.Fatal("Failed to get overlay strategy histogram: ", err)
	}

	// Wait for the Underlay overlay strategy because the PIP video
	// takes a moment to be promoted to overlay and then sometimes
	// uses the SingleOnTop overlay strategy for just a few frames.
	// failOnError is set to true if we detect an overlay (possibly
	// SingleOnTop) or a poll-breaking error (such as one returned
	// by GetHistogram). If we time out without ever detecting an
	// overlay, failOnError is false meaning the test should pass.
	failOnError := false
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		currentHist, err := metrics.GetHistogram(ctx, tconn, histName)
		if err != nil {
			failOnError = true
			return testing.PollBreak(errors.Wrap(err, "failed to get overlay strategy histogram"))
		}

		diffHist, err := currentHist.Diff(initialHist)
		if err != nil {
			failOnError = true
			return testing.PollBreak(errors.Wrap(err, "failed to diff overlay strategy histograms"))
		}

		for _, bucket := range diffHist.Buckets {
			if bucket.Min != overlayStrategyNoOverlay {
				failOnError = true
			}
			if bucket.Min == overlayStrategyUnderlay {
				return nil
			}
		}
		return errors.New("overlay strategy Underlay not found")
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		if failOnError {
			s.Fatal("Failed to wait for overlay strategy Underlay: ", err)
		}
		s.Log("PIP video not promoted to overlay")
		return
	}

	// Verify consistent use of the Underlay overlay strategy now.
	hists, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait a second")
		}
		return nil
	}, histName)
	if err != nil {
		s.Fatal("Failed to record overlay strategy data: ", err)
	}

	hist := hists[0]
	if len(hist.Buckets) == 0 {
		s.Fatal("Got no overlay strategy data")
	}

	for _, bucket := range hist.Buckets {
		if bucket.Min != overlayStrategyUnderlay {
			s.Errorf("Found %d frame(s) with an unexpected overlay strategy: got %d; want %d", bucket.Count, bucket.Min, overlayStrategyUnderlay)
		}
	}
}
