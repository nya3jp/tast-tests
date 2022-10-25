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
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/cuj/inputsimulations"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExampleCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Example of using the CUJ Recorder",
		Contacts:     []string{"ramsaroop@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Timeout:      15 * time.Minute,
		Vars:         []string{"record"},
		Params: []testing.Param{
			{
				Fixture: "loggedInToCUJUser",
				Val:     browser.TypeAsh,
			},
			{
				Name:    "lacros",
				Fixture: "loggedInToCUJUserLacros",
				Val:     browser.TypeLacros,
			},
		},
	})
}

// ExampleCUJ runs an example CUJ test using the CUJ recorder. This
// test peforms specific actions that trigger metrics for first
// contentful paint (FCP), largest contentful paint (LCP), mouse
// latency, key press latency, and Ash smoothness.
func ExampleCUJ(ctx context.Context, s *testing.State) {
	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Access Chrome from the fixture, as long as the fixture
	// supports the HasChrome interface.
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	browserType := s.Param().(browser.Type)

	// Use browserfixt.Setup to setup either Lacros or Ash, based on
	// the browser type set by the test variant parameters.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(closeCtx)

	// tconn is the Ash-chrome test connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API connection: ", err)
	}

	// bTconn is the browser test connection. This could either be a
	// connection to Ash or Lacros, depending on the test variant.
	bTconn, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Falied to connect to browser test API connection: ", err)
	}

	// Ensure we are in clamshell mode, because this example
	// primarily focuses on mouse actions, which are not valid
	// workflows on devices in tablet mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(closeCtx)

	// Since we are in clamshell mode, set up a mouse pointer to
	// help us click with the mouse on the device.
	pc := pointer.NewMouse(tconn)

	// Creating a new recorder with cujrecorder.NewRecorder.
	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	// [Optional] Enable tracing for the recorder.
	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))

	// [Optional] Add the pre-existing list of metrics to the
	// recorder with recorder.AddCommonMetrics.
	if err := recorder.AddCommonMetrics(tconn, bTconn); err != nil {
		s.Fatal("Failed to add common metrics to the recorder: ", err)
	}

	// [Optional] Add the screen recorder to the recorder to capture
	//  a video of the test, only if the "record" flag is passed as a
	// command line argument.
	if _, ok := s.Var("record"); ok {
		if err := recorder.AddScreenRecorder(ctx, tconn, s.TestName()); err != nil {
			s.Fatal("Failed to add screen recorder: ", err)
		}
	}

	// Create a virtual keyboard. This keyboard lets our test send key
	// presses during the test.
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	// Create a virtual mouse. This mouse lets our test perform mouse
	// actions, such as moving, scrolling, pressing, and releasing.
	mw, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal("Failed to get the mouse: ", err)
	}
	defer mw.Close()

	// Get the primary display info. In this particular test, this is
	// used to find the display bounds, for dragging the mouse to the
	// left and right of the screen.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	// The package uiauto contains multiple functions that help us
	// interact with the ui tree.
	ac := uiauto.New(tconn)

	// use faillog.DumpUITreeWithScreenshotOnError to capture the
	// device state at the time of a failure. An screenshot and text
	// file containing the ui tree is stored in the test out directory.
	defer faillog.DumpUITreeWithScreenshotOnError(closeCtx, s.OutDir(), s.HasError, cr, "failure_screenshot")

	// recorder.Run runs the provided function, and collects metrics
	// during its execution.
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		const (
			chromiumURL     = "https://chromium.org/Home"
			issueTrackerURL = "https://bugs.chromium.org/p/chromium/issues/list"
			searchQuery     = "This is my example search query for the Chromium Website"
			testDuration    = 10 * time.Minute
		)

		// To start the test, open a blank tab.
		conn, err := br.NewConn(ctx, "about:blank")
		if err != nil {
			return errors.Wrapf(err, "failed to open %s", chromiumURL)
		}

		// Lacros, by default, starts out with a New Tab window.
		// Thus, after opening about:blank, there will be 2 tabs
		// open. Close the New Tab window to make Ash/Lacros
		// variants equivalent.
		if s.Param().(browser.Type) == browser.TypeLacros {
			if err := browser.CloseTabByTitle(ctx, bTconn, "New Tab"); err != nil {
				return errors.Wrap(err, `failed to close "New Tab" tab`)
			}
		}

		// We want the test to run for about 10 minutes to
		// collect meaningful performance and power metrics. In
		// this test, we repeat the following actions until 10
		// minutes have passed.
		for endTime := time.Now().Add(testDuration); time.Now().Before(endTime); {
			// Navigate to the Chromium home page. Doing so
			// within recorder.Run generates FCP metrics.
			if err := conn.Navigate(ctx, chromiumURL); err != nil {
				return errors.Wrapf(err, "failed to navigate to %s", chromiumURL)
			}

			if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
				return errors.Wrap(err, "failed to wait for the tab to quiesce")
			}

			// uiauto.Combine combines a series of actions into a
			// single action, which helps lessen the code verboseness
			// by combining error handling into a single line. The
			// following actions interact with the device by using the
			// ui tree to find the search box and the search modal, to
			// generate mouse move/press/release and key press metrics.
			search := nodewith.NameStartingWith("Search").Role("button")
			searchModal := nodewith.HasClass("DocSearch-Container")
			if err := uiauto.Combine(
				"open the search window, type a search query, and exit the search window",
				// Move mouse to search box and click it.
				ac.MouseMoveTo(search, 500*time.Millisecond),
				ac.LeftClick(search),

				// Wait for the search modal to appear.
				ac.WaitUntilExists(searchModal),

				// Since the search modal is popped up and focused,
				// type a query into that text box.
				kw.TypeAction(searchQuery),

				// Press the Esc key to exit the search modal.
				kw.AccelAction("Esc"),

				// Wait until the search modal is completely gone from
				// view before continuing. Without waiting until gone,
				// the next action might be unexpectedly performed on
				// the search modal.
				ac.WaitUntilGone(searchModal),
			)(ctx); err != nil {
				return err
			}

			// RunDragMouseCycle clicks at the center of the
			// screen, and drags the mouse to the left and
			// right, and then back to the center. This
			// essentially highlights and unhighlights text
			// on the screen. This is a simple way to
			// generate mouse drag metrics.
			if err := inputsimulations.RunDragMouseCycle(ctx, tconn, info); err != nil {
				return errors.Wrap(err, "failed to run the drag mouse cycle")
			}

			// Navigate away from the the Chromium home page
			// to the Chromium bug tracker. Navigating away
			// from the initial page generates LCP metrics.
			if err := conn.Navigate(ctx, issueTrackerURL); err != nil {
				return errors.Wrapf(err, "failed to navigate to %s", issueTrackerURL)
			}

			if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
				return errors.Wrap(err, "failed to wait for the tab to quiesce")
			}

			// Since the Chromium bug tracker is a
			// scrollable page, scroll up and down using the
			// mouse wheel.
			for _, direction := range []string{"Down", "Up"} {
				// Scroll 50 mouse wheel ticks, with a
				// 50 milliseconds in between each tick.
				if err := inputsimulations.RepeatMouseScroll(ctx, mw, direction == "Down", 50*time.Millisecond, 50); err != nil {
					return errors.Wrapf(err, "failed to repeat mouse scroll %s", direction)
				}

				// Press the down or up arrow key 15
				// times, with 200 milliseconds in
				// between each press.
				if err := inputsimulations.RepeatKeyPress(ctx, kw, direction, 200*time.Millisecond, 15); err != nil {
					return errors.Wrapf(err, "failed to repeatedly press the %s arrow key", direction)
				}
			}

			// Generating Ash smoothness involves
			// interacting with the Ash UI. If the test
			// doesn't organically interact with the Ash UI,
			// inputsimulations.DoAshWorkflows can provide a
			// canned workflow that can generate ADF
			// metrics. In particular, it toggles the system
			// tray, and drags a window in overview mode.
			if err := inputsimulations.DoAshWorkflows(ctx, tconn, pc); err != nil {
				return errors.Wrap(err, "failed to do Ash workflow")
			}
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	// Recorder cleanup involves calling recorder.Record to actually
	// report the metrics, and pv.Save to save the metrics to the
	// test out directory.
	pv := perf.NewValues()
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
