// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

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
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleSheetsCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the total performance of critical user journey for Google Sheets",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome", "arc"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      13 * time.Minute,
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

func GoogleSheetsCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout                 = 10 * time.Second
		overallScrollTimeout    = 10 * time.Minute
		individualScrollTimeout = overallScrollTimeout / 4
	)

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	var cs ash.ConnSource
	var bTconn *chrome.TestConn
	switch s.Param().(browser.Type) {
	case browser.TypeAsh:
		cs = cr
		bTconn = tconn
	case browser.TypeLacros:
		// Launch lacros.
		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		defer l.Close(closeCtx)
		cs = l

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get lacros TestAPIConn: ", err)
		}
	}

	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, tconn)

	inTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to detect it is in tablet-mode or not: ", err)
	}
	if inTabletMode {
		// If it is in tablet mode, ensure it it in landscape orientation.
		orientation, err := display.GetOrientation(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get display orientation: ", err)
		}
		if orientation.Type == display.OrientationPortraitPrimary {
			info, err := display.GetPrimaryInfo(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get the primary display info: ", err)
			}
			s.Log("Rotating display 90 degrees")
			if err := display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
				s.Fatal("Failed to rotate display: ", err)
			}
			defer display.SetDisplayRotationSync(closeCtx, tconn, info.ID, display.Rotate0)
		}
	}
	s.Logf("Is in tablet-mode: %t", inTabletMode)

	ui := uiauto.New(tconn)

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

	// Create a virtual trackpad.
	tpw, err := input.Trackpad(ctx)
	if err != nil {
		s.Fatal("Failed to create a trackpad device: ", err)
	}
	defer tpw.Close()
	tw, err := tpw.NewMultiTouchWriter(2)
	if err != nil {
		s.Fatal("Failed to create a multi touch writer: ", err)
	}
	defer tw.Close()

	// Create a virtual keyboard.
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	// Create a virtual mouse.
	mw, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal("Failed to create a mouse: ", err)
	}
	defer mw.Close()

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	sheetURL, err := cuj.GetDriveURL(cuj.DriveTypeSheets)
	if err != nil {
		s.Fatal("Failed to get Google Sheets URL: ", err)
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Open Google Sheets file.
		sheetConn, err := cs.NewConn(ctx, sheetURL, browser.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open the Google Sheets website")
		}
		defer sheetConn.Close()
		defer sheetConn.CloseTarget(closeCtx)
		s.Log("Creating a Google Sheets window")

		// Pop-up content regarding view history privacy might show up.
		privacyButton := nodewith.Name("I understand").Role(role.Button)
		if err := uiauto.IfSuccessThen(ui.WaitUntilExists(privacyButton), ui.LeftClick(privacyButton))(ctx); err != nil {
			return errors.Wrap(err, "failed to click the spreadsheet privacy button")
		}

		s.Logf("Scrolling down the Google Sheets file for %s", overallScrollTimeout)

		for _, scroller := range []struct {
			description string
			run         func(ctx context.Context) error
		}{
			{
				description: "Pressing the down arrow with the mouse",
				run: func(ctx context.Context) error {
					sheetBounds, err := ui.Location(ctx, nodewith.Role("genericContainer").HasClass("grid-scrollable-wrapper"))
					if err != nil {
						return errors.Wrap(err, "failed to get the sheet location on the display")
					}

					// Select the point slightly to the right and above the bottom
					// corner of the sheet bounds. This is where the down arrow is.
					scrollArrowOffset := coords.NewPoint(4, -4)
					downArrow := sheetBounds.BottomRight().Add(scrollArrowOffset)
					if err := mouse.Move(tconn, downArrow, time.Second)(ctx); err != nil {
						return errors.Wrap(err, "failed to move mouse to the down arrow")
					}

					return inputsimulations.RepeatMousePressFor(ctx, mw, 500*time.Millisecond, 3*time.Second, individualScrollTimeout)
				},
			},
			{
				description: "Using the scroll wheel",
				run: func(ctx context.Context) error {
					return inputsimulations.ScrollMouseDownFor(ctx, mw, 200*time.Millisecond, individualScrollTimeout)
				},
			},
			{
				description: "Using trackpad gestures",
				run: func(ctx context.Context) error {
					return inputsimulations.ScrollDownFor(ctx, tpw, tw, 500*time.Millisecond, individualScrollTimeout)
				},
			},
			{
				description: "Using the down arrow key",
				run: func(ctx context.Context) error {
					return inputsimulations.RepeatKeyPressFor(ctx, kw, "Down", 500*time.Millisecond, individualScrollTimeout)
				},
			},
		} {
			s.Log(scroller.description)
			if err := scroller.run(ctx); err != nil {
				return errors.Wrapf(err, "failed to scroll %s", scroller.description)
			}

			if err := inputsimulations.RunDragMouseCycle(ctx, tconn, info); err != nil {
				return err
			}
		}

		var scrollTop int
		// Ensure scrollbar gets scrolled.
		if err := sheetConn.Eval(ctx, "parseInt(document.getElementsByClassName('native-scrollbar-y')[0].scrollTop)", &scrollTop); err != nil {
			return errors.Wrap(err, "failed to get the number of pixels that the scrollbar is scrolled vertically")
		}
		if scrollTop == 0 {
			return errors.New("scroll didn't happen")
		}

		// Navigate away to record PageLoad.PaintTiming.NavigationToLargestContentfulPaint2.
		if err := sheetConn.Navigate(ctx, "chrome://version"); err != nil {
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
