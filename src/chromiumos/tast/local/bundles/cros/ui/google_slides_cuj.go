// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/cuj/inputsimulations"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
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
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Timeout:      15 * time.Minute,
		Vars:         []string{"record"},
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
	const slidesScrollTimeout = 10 * time.Minute

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
	var bTconn *chrome.TestConn
	switch bt {
	case browser.TypeLacros:
		var err error
		if cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), browser.TypeLacros); err != nil {
			s.Fatal("Failed to initialize test: ", err)
		}
		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get lacros TestAPIConn: ", err)
		}
		defer lacros.CloseLacros(closeCtx, l)
	case browser.TypeAsh:
		cs = cr
		bTconn = tconn
	default:
		s.Fatal("Unrecognized browser type: ", bt)
	}

	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a CUJ recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	if err := recorder.AddCommonMetrics(tconn, bTconn); err != nil {
		s.Fatal("Failed to add common metrics to recorder: ", err)
	}

	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))

	if _, ok := s.Var("record"); ok {
		if err := recorder.AddScreenRecorder(ctx, tconn, s.TestName()); err != nil {
			s.Fatal("Failed to add screen recorder: ", err)
		}
	}

	// Create a virtual keyboard.
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	slidesURL, err := cuj.GetDriveURL(cuj.DriveTypeSlides)
	if err != nil {
		s.Fatal("Failed to get Google Slides URL: ", err)
	}

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

		// At fixed intervals, stop scrolling and click a menu item
		// to ensure we collect mouse metrics.
		ac := uiauto.New(tconn)
		for endTime := time.Now().Add(slidesScrollTimeout); time.Now().Before(endTime); {
			if err := inputsimulations.RepeatKeyPress(ctx, kw, "Down", time.Second, 10); err != nil {
				return errors.Wrap(err, "failed to scroll down with down arrow")
			}

			fileMenu := nodewith.Name("File").HasClass("menu-button")
			if err := action.Combine(
				"open and close the file menu and then refocus on the presentation",
				// Open file menu.
				ac.MouseMoveTo(fileMenu, 500*time.Millisecond),
				ac.LeftClick(fileMenu),
				action.Sleep(time.Second),

				// Close file menu.
				ac.LeftClick(fileMenu),

				// Refocus on the presentation.
				mouse.Move(tconn, info.Bounds.CenterPoint(), 500*time.Millisecond),
			)(ctx); err != nil {
				return err
			}

			if err := inputsimulations.RunDragMouseCycle(ctx, tconn, info); err != nil {
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
