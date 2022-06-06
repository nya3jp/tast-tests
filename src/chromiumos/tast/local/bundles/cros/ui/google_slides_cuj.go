// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleSlidesCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the total performance of critical user journey for Google Slides",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild", "group:cuj"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "loggedInToCUJUser",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "loggedInToCUJUserLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func GoogleSlidesCUJ(ctx context.Context, s *testing.State) {
	const (
		slidesURL           = "https://docs.google.com/presentation/d/1lItrhkgBqXF_bsP-tOqbjcbBFa86--m3DT5cLxegR2k/edit?usp=sharing&resourcekey=0-FmuN4N-UehRS2q4CdQzRXA"
		slidesScrollTimeout = 10 * time.Minute
	)

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	var l *lacros.Lacros
	var cs ash.ConnSource
	var br *browser.Browser
	switch bt {
	case browser.TypeLacros:
		var err error
		if cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), browser.TypeLacros); err != nil {
			s.Fatal("Failed to initialize test: ", err)
		}
		br = l.Browser()
		defer lacros.CloseLacros(closeCtx, l)
	case browser.TypeAsh:
		cs = cr
		br = cr.Browser()
	default:
		s.Fatal("Unrecognized browser type: ", bt)
	}

	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a CUJ recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	if err := recorder.AddCollectedMetrics(br, cujrecorder.DeprecatedMetricConfigs()...); err != nil {
		s.Fatal("Failed to add metrics to recorder: ", err)
	}

	// Create a virtual keyboard.
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	if err := recorder.Run(ctx, func(ctx context.Context) (retErr error) {
		hasError := func() bool { return retErr != nil }
		slidesConn, err := cs.NewConn(ctx, slidesURL, browser.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open the google slides website")
		}
		defer slidesConn.Close()
		defer slidesConn.CloseTarget(closeCtx)

		defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), hasError, tconn)
		defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), hasError, cr, "ui_dump")

		// Go through the Slides deck.
		s.Logf("Going through the Google Slides file for %s", slidesScrollTimeout)
		for end := time.Now().Add(slidesScrollTimeout); time.Now().Before(end); {
			if err := uiauto.Combine(
				"sleep and press down",
				action.Sleep(time.Second),
				kw.AccelAction("Down"),
			)(ctx); err != nil {
				return err
			}
		}

		// Ensure the slides deck gets scrolled.
		var scrollTop int
		if err := slidesConn.Eval(ctx, "parseInt(document.getElementsByClassName('punch-filmstrip-scroll')[0].scrollTop)", &scrollTop); err != nil {
			return errors.Wrap(err, "failed to get the number of pixels that the scrollbar is scrolled vertically")
		}
		if scrollTop == 0 {
			return errors.New("file is not getting scrolled")
		}

		// Navigate away to record PageLoad.PaintTiming.NavigationToLargestContentfulPaint2.
		if err := slidesConn.Navigate(ctx, "chrome://version"); err != nil {
			return errors.Wrap(err, "failed to navigate to chrome://version")
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to run the test scenario: ", err)
	}

	pv := perf.NewValues()
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
	if err := recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Error("Failed to save histogram raw data: ", err)
	}
}
