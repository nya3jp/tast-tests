// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
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
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verifies that Chrome PIP rounded corners are implemented with a hardware underlay",
		Contacts:     []string{"amusbach@chromium.org", "oshima@chromium.org", "chromeos-perf@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "proprietary_codecs"},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
		Data:         []string{"180p_60fps_600frames.h264.mp4", "pip_video.html"},
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
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("cave", "chell")),
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
			ExtraHardwareDeps: hwdep.D(hwdep.Model("cave", "chell")),
			Fixture:           "chromeGraphicsLacros",
			Val:               browser.TypeLacros,
		}},
	})
}

func ChromePIPRoundedCornersUnderlay(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	browserType := s.Param().(browser.Type)
	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), browserType)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(cleanupCtx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	conn, err := cs.NewConn(ctx, srv.URL+"/pip_video.html")
	if err != nil {
		s.Fatal("Failed to load pip_video.html: ", err)
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

	wID := ws[0].ID
	if err := ash.SetWindowStateAndWait(ctx, tconn, wID, ash.WindowStateMaximized); err != nil {
		s.Fatal("Failed to maximize window: ", err)
	}

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for pip_video.html to achieve quiescence: ", err)
	}

	tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check whether tablet mode is active: ", err)
	}

	var pc pointer.Context
	if tabletMode {
		pc, err = pointer.NewTouch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller: ", err)
		}
	} else {
		pc = pointer.NewMouse(tconn)
	}
	defer pc.Close()

	ac := uiauto.New(tconn)
	pipButton := nodewith.Name("PIP").Role(role.Button)

	var pipClassName string
	switch browserType {
	case browser.TypeAsh:
		pipClassName = "PictureInPictureWindow"
	case browser.TypeLacros:
		pipClassName = "Widget"
	}
	pipWindow := nodewith.Name("Picture in picture").ClassName(pipClassName).Onscreen().First()

	if err := action.Combine(
		"click/tap PIP button and wait for PIP window",
		pc.Click(pipButton),
		ac.WithTimeout(10*time.Second).WaitUntilExists(pipWindow),
	)(ctx); err != nil {
		s.Fatal("Failed to show the PIP window: ", err)
	}

	pipWindowBounds, err := ac.Location(ctx, pipWindow)
	if err != nil {
		s.Fatal("Failed to get the PIP window location (before resize): ", err)
	}

	// Drag to resize the PIP window. Begin at an offset from the
	// corner, to ensure that for touch input (which is used by this
	// test on devices that default to tablet mode), when the
	// coordinates are converted to input.TouchCoord, the rounding
	// error will not perturb the point out of the PIP window bounds.
	if err := pc.Drag(pipWindowBounds.TopLeft().Add(coords.NewPoint(1, 1)), pc.DragTo(info.WorkArea.TopLeft(), time.Second))(ctx); err != nil {
		s.Fatal("Failed to resize the PIP window: ", err)
	}

	pipWindowBounds, err = ac.Location(ctx, pipWindow)
	if err != nil {
		s.Fatal("Failed to get the PIP window location (after resize): ", err)
	}

	// For code maintainability, just check a relatively permissive expectation for
	// the maximum size of the PIP window: it should be either strictly wider than 2/5
	// of the work area width, or strictly taller than 2/5 of the work area height.
	if 5*pipWindowBounds.Width <= 2*info.WorkArea.Width && 5*pipWindowBounds.Height <= 2*info.WorkArea.Height {
		s.Fatalf("Expected a bigger PIP window. Got a %v PIP window in a %v work area", pipWindowBounds.Size(), info.WorkArea.Size())
	}

	// Minimize the main browser window to ensure that its overlay
	// strategy will not be detected when what we want to know is
	// the overlay strategy used for the PIP window.
	if err := ash.SetWindowStateAndWait(ctx, tconn, wID, ash.WindowStateMinimized); err != nil {
		s.Fatal("Failed to minimize window: ", err)
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
