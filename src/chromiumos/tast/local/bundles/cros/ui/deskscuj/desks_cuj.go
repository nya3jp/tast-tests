// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package deskscuj contains helper util and test code for DesksCUJ.
package deskscuj

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
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

// Run runs the desks CUJ by opening up 4 different desks and switching
// between them using various workflows.
func Run(ctx context.Context, s *testing.State) {
	// deskSwitchingDuration is how long we should run each workflow for.
	// To have the full test run in 10 minutes,  we want to have each of
	// the 3 workflows run in 10/3 minutes.
	const deskSwitchingDuration = time.Minute * 10 / 3

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	var cs ash.ConnSource
	var bTconn *chrome.TestConn
	switch bt {
	case browser.TypeLacros:
		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch Lacros: ", err)
		}
		defer l.Close(cleanupCtx)
		cs = l

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to connect to the Lacros TestAPIConn: ", err)
		}
	case browser.TypeAsh:
		cs = cr
		bTconn = tconn
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kw.Close()

	mw, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal("Failed to get the mouse: ", err)
	}
	defer mw.Close()

	tpw, err := input.Trackpad(ctx)
	if err != nil {
		s.Fatal("Failed to create a trackpad device: ", err)
	}
	defer tpw.Close()

	tw, err := tpw.NewMultiTouchWriter(2)
	if err != nil {
		s.Fatal("Failed to create a multi-touch writer with 2 touches: ", err)
	}
	defer tw.Close()

	ac := uiauto.New(tconn)

	// The above preparation may take several minutes. Ensure that the
	// display is awake and will stay awake for the performance measurement.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to wake display: ", err)
	}

	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(cleanupCtx)

	if err := recorder.AddCommonMetrics(tconn, bTconn); err != nil {
		s.Fatal("Failed to add common metrics to recorder: ", err)
	}

	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))

	if _, ok := s.Var("record"); ok {
		if err := recorder.AddScreenRecorder(ctx, tconn, s.TestName()); err != nil {
			s.Fatal("Failed to add screen recorder: ", err)
		}
	}

	// Take a screenshot every 2 minutes up to a maximum of 5
	// screenshots, to capture the state of the device during each of the
	// desk switching workflows.
	if err := recorder.AddScreenshotRecorder(ctx, 2*time.Minute, 5); err != nil {
		s.Log("Failed to add screenshot recorder: ", err)
	}

	defer ash.CleanUpDesks(cleanupCtx, tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "failure")

	// Open all desks and windows for each desk. Additionally, initialize
	// unique user input actions that will be performed on each desk.
	onVisitActions, expectedNumWindows, err := setUpDesks(ctx, tconn, bTconn, cs, kw, mw, tpw, tw)
	if err != nil {
		s.Fatal("Failed to set up desks: ", err)
	}

	topRow, err := input.KeyboardTopRowLayout(ctx, kw)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}
	setOverviewModeAndWait := func(ctx context.Context) error {
		if err := kw.Accel(ctx, topRow.SelectTask); err != nil {
			s.Fatal("Failed to hit overview key: ", err)
		}
		return ash.WaitForOverviewState(ctx, tconn, ash.Shown, 10*time.Second)
	}

	if bt == browser.TypeLacros {
		if err := browser.CloseTabByTitle(ctx, bTconn, "New Tab"); err != nil {
			s.Fatal(`Failed to close "New Tab" tab: `, err)
		}
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Open a window within recorder.Run to ensure we collect
		// PageLoad.PaintTiming.NavigationToFirstContentfulPaint.
		if err := ash.ActivateDeskAtIndex(ctx, tconn, 0); err != nil {
			return errors.Wrap(err, "failed to activate leftmost desk with the autotest API")
		}
		activeDesk := 0

		slidesURL, err := cuj.GetTestSlidesURL(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get Google Slides URL")
		}

		slidesConn, err := cuj.NewTabByURL(ctx, cs, true, slidesURL)
		if err != nil {
			return errors.Wrap(err, "failed to open a Google Slides presentation")
		}
		expectedNumWindows++

		for _, deskSwitcher := range []deskSwitchWorkflow{
			getKeyboardSearchBracketWorkflow(tconn, kw),
			getKeyboardSearchNumberWorkflow(tconn, kw),
			getOverviewWorkflow(tconn, ac, setOverviewModeAndWait),
		} {
			s.Log(deskSwitcher.description)

			cycles := 0

			if startDesk := deskSwitcher.itinerary[0]; activeDesk != startDesk {
				if err := ash.ActivateDeskAtIndex(ctx, tconn, startDesk); err != nil {
					return errors.Wrapf(err, "failed to activate desk %d with the autotest API", startDesk)
				}
				activeDesk = startDesk
			}

			i := 0
			for endTime := time.Now().Add(deskSwitchingDuration); time.Now().Before(endTime); {
				i = (i + 1) % len(deskSwitcher.itinerary)
				nextDesk := deskSwitcher.itinerary[i]

				err := deskSwitcher.run(ctx, activeDesk, nextDesk)
				if err != nil {
					return errors.Wrapf(err, "failed to switch to the next desk using %s", deskSwitcher.name)
				}

				if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
					s.Log("Failed to wait for desks animations to stabilize: ", err)
				}

				info, err := ash.GetDesksInfo(ctx, tconn)
				if err != nil {
					return errors.Wrap(err, "failed to get the active desk index")
				}

				activeDesk = info.ActiveDeskIndex

				// Compare the actual active desk to the expected active desk.
				if activeDesk != nextDesk {
					return errors.Errorf("unexpected active desk: desk %d is active, expected %d to be active", activeDesk, nextDesk)
				}

				if err := onVisitActions[activeDesk](ctx); err != nil {
					return errors.Wrapf(err, "failed to perform unique action on desk %d", activeDesk)
				}
				cycles++
			}

			// Ensure that none of the windows crashed during the test.
			ws, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get all windows")
			}

			if len(ws) != expectedNumWindows {
				return errors.Errorf("unexpected number of open windows, got %d, expected %d", len(ws), expectedNumWindows)
			}

			s.Logf("Switched desk by %s %d times", deskSwitcher.name, cycles)
		}

		// Navigate away to record PageLoad.PaintTiming.NavigationToLargestContentfulPaint2.
		if err := slidesConn.Conn.Navigate(ctx, "chrome://version"); err != nil {
			return errors.Wrap(err, "failed to navigate to chrome://version")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	pv := perf.NewValues()
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the performance data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the performance data: ", err)
	}
}
