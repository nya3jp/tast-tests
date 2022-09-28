// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package taskswitchcuj contains helper util and test code for TaskSwitchCUJ.
package taskswitchcuj

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
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

// TaskSwitchTest holds parameters for the TaskSwitchCUJ test variants.
type TaskSwitchTest struct {
	BrowserType browser.Type
	Tablet      bool
}

// Run runs the task switch CUJ by opening up ARC and browser windows
// and switching among them using various workflows.
func Run(ctx context.Context, s *testing.State) {
	const taskSwitchingDuration = 5 * time.Minute

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
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
		defer l.Close(closeCtx)
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

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, testParam.Tablet)
	if err != nil {
		s.Fatalf("Failed to ensure the tablet mode state [%t]: %v", testParam.Tablet, err)
	}
	defer cleanup(closeCtx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to set up ARC and Play Store: ", err)
	}
	defer d.Close(closeCtx)

	ac := uiauto.New(tconn)

	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, a, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	if err := recorder.AddCommonMetrics(tconn, bTconn); err != nil {
		s.Fatal("Failed to add common metrics to the recorder: ", err)
	}
	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))
	defer recorder.Close(closeCtx)

	if _, ok := s.Var("record"); ok {
		if err := recorder.AddScreenRecorder(ctx, tconn, s.TestName()); err != nil {
			s.Fatal("Failed to add screen recorder: ", err)
		}
	}

	topRow, err := input.KeyboardTopRowLayout(ctx, kw)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}

	setOverviewModeWithKeyboard := func(ctx context.Context) error {
		if err := kw.Accel(ctx, topRow.SelectTask); err != nil {
			return errors.Wrap(err, "failed to hit overview key")
		}
		if err := ash.WaitForOverviewState(ctx, tconn, ash.Shown, 30*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for overview state")
		}

		// Add a stabilization delay after entering overview mode to
		// mitigate Lacros crashes on lower-end devices. This delay
		// ensures that the previews are properly loaded before we
		// interact with the windows in overview mode.
		if err := ac.WithInterval(2*time.Second).WithTimeout(10*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			s.Log("Failed to wait for overview stabilization: ", err)
		}
		return nil
	}

	setOverviewMode := setOverviewModeWithKeyboard
	var tcc *input.TouchCoordConverter
	var stw *input.SingleTouchEventWriter
	var pc pointer.Context
	if testParam.Tablet {
		pc, err = pointer.NewTouch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller: ", err)
		}
		defer pc.Close()

		var tsew *input.TouchscreenEventWriter
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
		startX, startY, endX, endY = tsew.Width()-1, tsew.Height()/2, 0, tsew.Height()/2
		setOverviewMode = func(ctx context.Context) error {
			// Press the overview button to emulate an external keyboard,
			// due to the flakiness of swiping to overview mode on
			// lower-end devices.
			if err := setOverviewModeWithKeyboard(ctx); err != nil {
				return err
			}

			if err := stw.Swipe(ctx, startX, startY, endX, endY, 2*time.Second); err != nil {
				return errors.Wrap(err, "failed to swipe horizontally in overview mode")
			}

			if err := stw.End(); err != nil {
				return errors.Wrap(err, "failed to end swipe animation")
			}
			return nil
		}
	} else {
		pc = pointer.NewMouse(tconn)
		defer pc.Close()
	}

	defer ash.CloseAllWindows(closeCtx, tconn)

	// Close browser tabs before "CloseAllWindows". If we
	// simply "CloseAllWindows", the next variant of the test
	// that tries to open the browser sometimes reopens the previous
	// set of tabs (like pressing Ctrl+Shift+T after you close a
	// bunch of tabs within the same window).
	defer browser.CloseAllTabs(closeCtx, bTconn)

	defer faillog.DumpUITreeWithScreenshotOnError(closeCtx, s.OutDir(), s.HasError, cr, "failure_screenshot")

	s.Log("Installing packages")
	packages := getPackages(ctx, tconn, d)
	if err := installPackages(ctx, tconn, a, d, packages); err != nil {
		s.Fatal("Failed to install packages: ", err)
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Launch packages before launching Chrome tabs, to mitigate
		// flakiness when opening applications. When a lot of tabs are
		// open, sometimes the launcher does not stabilize within the
		// required timeout.
		s.Log("Launching packages")
		numAppWindows, err := launchPackages(ctx, tconn, kw, ac, packages)
		if err != nil {
			return errors.Wrap(err, "failed to launch apps")
		}

		s.Log("Opening Chrome Tabs")
		numBrowserWindows, err := openChromeTabs(ctx, tconn, bTconn, cs, testParam.BrowserType, testParam.Tablet)
		if err != nil {
			return errors.Wrap(err, "failed to launch apps")
		}
		numWindows := numAppWindows + numBrowserWindows

		// Initialize task switch workflows only after launching Chrome
		// tabs and applications, because switching by Hotseat requires
		// knowing the bounds of the icons of the open windows.
		taskSwitchers := []taskSwitchWorkflow{
			initializeSwitchTaskByOverviewMode(ctx, tconn, pc, setOverviewMode),
		}
		if testParam.Tablet {
			switchTaskByHotseat, err := initializeSwitchTaskByHotseat(ctx, tconn, stw, tcc, pc, ac, numWindows, numBrowserWindows)
			if err != nil {
				return errors.Wrap(err, "failed to initialize switching task by hotseat")
			}
			taskSwitchers = append(taskSwitchers, *switchTaskByHotseat)
		} else {
			taskSwitchers = append(taskSwitchers, initializeSwitchTaskByAltTab(ctx, kw, numWindows))
		}

		for _, taskSwitcher := range taskSwitchers {
			s.Log(taskSwitcher.description)
			cycles := 0
			for endTime := time.Now().Add(taskSwitchingDuration); time.Now().Before(endTime); {
				if err := taskSwitcher.run(ctx); err != nil {
					return errors.Wrapf(err, "failed to switch to next window using %s", taskSwitcher.name)
				}

				w, err := ash.GetActiveWindow(ctx, tconn)
				if err != nil {
					return errors.Wrap(err, "failed to find active window")
				}
				if err := ash.WaitWindowFinishAnimating(ctx, tconn, w.ID); err != nil {
					return errors.Wrap(err, "failed to wait for window to finish animating")
				}

				// Wait a few seconds to let the page load. Wait a fixed amount
				// of time to keep consistency in task switches among different
				// devices.
				if err := testing.Sleep(ctx, 3*time.Second); err != nil {
					return errors.Wrap(err, "failed to sleep")
				}

				// Move mouse to center of screen to ensure we are
				// scrolling on the currently active window.
				if err := mouse.Move(tconn, info.Bounds.CenterPoint(), 500*time.Millisecond)(ctx); err != nil {
					return errors.Wrap(err, "failed to move mouse to center of screen")
				}

				// Try to scroll down and up by pressing the down and up
				// arrow key. This gives us some input latency metrics
				// while delaying between each task switch. This also
				// helps increase memory pressure, because it forces Chrome
				// to load more of the page.
				for _, key := range []string{"Down", "Up"} {
					if err := inputsimulations.RepeatKeyPress(ctx, kw, key, 300*time.Millisecond, 10); err != nil {
						return errors.Wrapf(err, "failed to repeatedly press %q in between task switches", key)
					}

					if err := inputsimulations.RepeatMouseScroll(ctx, mw, key == "Down", 100*time.Millisecond, 20); err != nil {
						return errors.Wrapf(err, "failed to repeatedly mouse scroll %s", key)
					}
				}

				cycles++
			}
			s.Logf("Switched task by %s %d times", taskSwitcher.name, cycles)

			// Ensure the right number of windows are still opened.
			if ws, err := ash.GetAllWindows(ctx, tconn); len(ws) != numWindows {
				return errors.Wrapf(err, "unexpected number of open windows, got: %d, expected: %d", len(ws), numWindows)
			}

			// Wait for any animations from the previous workflow to
			// successfully finish. Continue to the next workflow if
			// stabilization fails, because sometimes the visible
			// window does not stabilize in a timely manner.
			if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
				s.Logf("Failed to wait for the window to stabilize after running workflow %s: %v", taskSwitcher.name, err)
			}
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	pv := perf.NewValues()
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
