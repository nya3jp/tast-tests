// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package taskswitchcuj contains helper util and test code for TaskSwitchCUJ.
package taskswitchcuj

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
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

// TaskSwitchTest holds parameters for the TaskSwitchCUJ test variants.
type TaskSwitchTest struct {
	Tablet      bool
	BrowserType browser.Type
	Tracing     bool // Whether to turn on tracing.
}

// Run runs the task switch CUJ by opening up ARC and browser windows
// and switching between each window using various workflows.
func Run(ctx context.Context, s *testing.State) {
	const subtestDuration = 5 * time.Minute

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	testParam := s.Param().(TaskSwitchTest)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	a := s.FixtValue().(cuj.FixtureData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	var cs ash.ConnSource
	var bTconn *chrome.TestConn
	switch testParam.BrowserType {
	case browser.TypeLacros:
		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch Lacros: ", err)
		}
		defer l.Close(ctx)
		cs = l

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to connect to the Lacros TestAPIConn: ", err)
		}
	case browser.TypeAsh:
		cs = cr
		bTconn = tconn
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kw.Close()

	tabletMode := testParam.Tablet
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure the tablet mode state [%t]: %v", tabletMode, err)
	}
	defer cleanup(ctx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to set up ARC and Play Store: ", err)
	}
	defer d.Close(ctx)

	ac := uiauto.New(tconn)

	pv := perf.NewValues()

	recorder, err := cujrecorder.NewRecorder(ctx, cr, a, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	if err := recorder.AddCommonMetrics(tconn, bTconn); err != nil {
		s.Fatal("Failed to add common metrics to the recorder: ", err)
	}
	if testParam.Tracing {
		recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))
	}
	defer recorder.Close(closeCtx)

	var setOverviewMode func(ctx context.Context) error
	var tsew *input.TouchscreenEventWriter
	var tcc *input.TouchCoordConverter
	var stw *input.SingleTouchEventWriter
	var pc pointer.Context
	if tabletMode {
		pc, err = pointer.NewTouch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller: ", err)
		}

		if tsew, tcc, err = touch.NewTouchscreenAndConverter(ctx, tconn); err != nil {
			s.Fatal("Failed to access the touchscreen: ", err)
		}
		defer tsew.Close()

		if stw, err = tsew.NewSingleTouchWriter(); err != nil {
			s.Fatal("Failed to create a single touch writer: ", err)
		}
		defer stw.Close()

		// Overview mode on tablet scrolls horizontally, limiting the number of windows
		// visible on the screen at a given time. Thus, after showing the overview
		// mode, horizontally scroll the window to the right, so the least recently
		// used window is visible on the screen.
		var startX, startY, endX, endY input.TouchCoord
		startX, startY, endX, endY = tsew.Width()-1, tsew.Height()/2, 1, tsew.Height()/2
		setOverviewMode = func(ctx context.Context) error {
			// Setting overview mode by swipe can be flaky, so set
			// overview directly.
			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				return errors.Wrap(err, "failed to enter overview mode")
			}

			if err := stw.Swipe(ctx, startX, startY, endX, endY, time.Second); err != nil {
				return errors.Wrap(err, "failed to swipe horizontally in overview mode")
			}
			return stw.End()
		}
	} else {
		pc = pointer.NewMouse(tconn)

		topRow, err := input.KeyboardTopRowLayout(ctx, kw)
		if err != nil {
			s.Fatal("Failed to obtain the top-row layout: ", err)
		}

		setOverviewMode = func(ctx context.Context) error {
			if err := kw.Accel(ctx, topRow.SelectTask); err != nil {
				return errors.Wrap(err, "failed to hit overview key")
			}
			return ash.WaitForOverviewState(ctx, tconn, ash.Shown, 10*time.Second)
		}
	}
	defer pc.Close()

	s.Log("Installing packages")
	packages := getPackages(ctx, tconn, d)
	if err := installPackages(ctx, tconn, a, d, packages); err != nil {
		s.Fatal("Failed to install packages: ", err)
	}

	defer ash.CloseAllWindows(ctx, tconn)

	// Close browser tabs before "CloseAllWindows". If we
	// simply "CloseAllWindows", the next variant of the test
	// who tries to open the browser sometimes reopens the previous
	// set of tabs (like pressing Ctrl+Shift+T after you close a
	// bunch of tabs within the same window).
	defer cuj.CloseBrowserTabs(ctx, bTconn)

	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, tconn)

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Launch packages before launching Chrome tabs to mitigate
		// flakiness when opening applications. When a lot of tabs are
		// open, sometimes the launcher does not stabilize within the
		// required timeout.
		s.Log("Launching packages")
		var numAppWindows int
		if numAppWindows, err = launchPackages(ctx, tconn, kw, ac, packages); err != nil {
			s.Fatal("Failed to launch apps: ", err)
		}

		s.Log("Opening Chrome Tabs")
		var initTabs func(ctx context.Context) error
		var cleanupTabs func(ctx context.Context) error
		var numBrowserWindows int
		if initTabs, cleanupTabs, numBrowserWindows, err = openChromeTabs(ctx, tconn, bTconn, cs, testParam.BrowserType == browser.TypeLacros, tabletMode, pv); err != nil {
			s.Fatal("Failed to open Chrome tabs: ", err)
		}
		numWindows := numAppWindows + numBrowserWindows

		if err := initTabs(ctx); err != nil {
			return errors.Wrap(err, "failed to initialize tabs")
		}
		defer cleanupTabs(ctx)

		// Initialize subtests only after launching Chrome tabs and
		// applications, because switching by Hotseat requires knowing
		// the bounds of the icons of the open windows.
		type subtest struct {
			name               string
			desc               string
			switchToNextWindow func(ctx context.Context) error
		}
		subtests := []subtest{{
			name:               "Overview",
			desc:               "Cycle through open applications using the overview mode",
			switchToNextWindow: initializeSwitchTaskByOverviewMode(ctx, tconn, pc, setOverviewMode),
		}}
		if tabletMode {
			if switchTaskByHotseat, err := initializeSwitchTaskByHotseat(ctx, tconn, stw, tcc, pc, ac, numWindows, numBrowserWindows); err != nil {
				s.Fatal("Failed to initialize switching task by hotseat: ", err)
			} else {
				subtests = append(subtests, subtest{
					name:               "Hotseat",
					desc:               "Cycle through open applications using the hotseat",
					switchToNextWindow: switchTaskByHotseat,
				})
			}
		} else {
			subtests = append(subtests, subtest{
				name:               "Alt+Tab",
				desc:               "Cycle through open applications using Alt+Tab",
				switchToNextWindow: initializeSwitchTaskByAltTab(ctx, kw, numWindows),
			})
		}

		for _, subtest := range subtests {
			s.Log(subtest.desc)
			cycles := 0
			for endTime := time.Now().Add(subtestDuration); time.Now().Before(endTime); {
				if err := action.Combine(
					"switch to next window and scroll down and up",
					subtest.switchToNextWindow,
					// Give the page some time to load.
					action.Sleep(3*time.Second),
					// Try to scroll down and up by pressing the down and
					// up arrow key. This gives us some input latency
					// metrics while delaying between each task switch.
					// This also helps increase memory pressure, because
					// it forces Chrome to load more of the page.
					func(ctx context.Context) error {
						cycles++
						for _, key := range []string{"Down", "Up"} {
							if err := inputsimulations.RepeatKeyPress(ctx, kw, key, 300*time.Millisecond, 10); err != nil {
								return errors.Wrapf(err, "failed to repeatedly press %q in between task switches", key)
							}
						}
						return nil
					},
				)(ctx); err != nil {
					return errors.Wrapf(err, "failed to switch to next window using %s", subtest.name)
				}
			}
			s.Logf("Switched task by %s %d times", subtest.name, cycles)

			// Wait for any animations from the previous subtest to
			// successfully finish.
			if err := testing.Sleep(ctx, 5*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
