// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/bond"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/profiler"
	"chromiumos/tast/testing"
)

type meetLayoutType string

const (
	meetLayoutSpotlight meetLayoutType = "Spotlight"
	meetLayoutTiled     meetLayoutType = "Tiled"
	meetLayoutSidebar   meetLayoutType = "Sidebar"
	meetLayoutAuto      meetLayoutType = "Auto"
)

// meetTest specifies the setting of a Hangouts Meet journey.
type meetTest struct {
	num     int            // Number of the participants in the meeting.
	layout  meetLayoutType // Type of the layout in the meeting.
	present bool           // Whether it is presenting the Google Docs or not. It can not be true if docs is false.
	docs    bool           // Whether it is running with a Google Docs window.
	split   bool           // Whether it is in split screen mode. It can not be true if docs is false.
	cam     bool           // Whether the camera is on or not.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MeetCUJ,
		Desc:         "Measures the performance of critical user journey for Google Meet",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      3 * time.Minute,
		Pre:          cuj.LoggedInToCUJUser(),
		Vars: []string{
			"mute",
			"record",
			"ui.MeetCUJ.bond_credentials",
			"ui.cuj_username",
			"ui.cuj_password",
		},
		Params: []testing.Param{{
			Name: "base_case",
			Val: meetTest{
				num:     4,
				layout:  meetLayoutTiled,
				present: false,
				docs:    false,
				split:   false,
				cam:     true,
			},
		}, {
			Name: "worst_case",
			Val: meetTest{
				num:     4,
				layout:  meetLayoutTiled,
				present: true,
				docs:    true,
				split:   true,
				cam:     true,
			},
		}, {
			Name: "big_meeting",
			Val: meetTest{
				num:     16,
				layout:  meetLayoutTiled,
				present: false,
				docs:    false,
				split:   false,
				cam:     true,
			},
		}, {
			Name: "big_meeting_with_notes",
			Val: meetTest{
				num:     16,
				layout:  meetLayoutTiled,
				present: false,
				docs:    true,
				split:   false,
				cam:     true,
			},
		}},
	})
}

// MeetCUJ measures the performance of critical user journeys for Google Meet.
// Journeys for Google Meet are specified by testing parameters.
//
// Pre-preparation:
//   - Open a Meet window.
//   - Create and enter the meeting code.
//   - Open a Google Docs window (if necessary).
//   - Enter split mode (if necessary).
//   - Turn off camera (if necessary).
// During recording:
//   - Join the meeting.
//   - Add participants(bots) to the meeting.
//   - Set up the layout.
//   - Max out the number of the maximum tiles (if necessary).
//   - Start to present (if necessary).
//   - Input notes to Google Docs file (if necessary).
//   - Wait for 30 seconds before ending the meeting.
// After recording:
//   - Record and save metrics.
func MeetCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout     = 10 * time.Second
		botDuration = 2 * time.Minute
		docsURL     = "https://docs.google.com/document/d/1qREN9w1WgjgdGYBT_eEtE6T21ErlW_4nQoBJVhrR1S0/edit"
		notes       = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."
		meetTimeout = 30 * time.Second
	)

	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: timeout}

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	meet := s.Param().(meetTest)

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	creds := s.RequiredVar("ui.MeetCUJ.bond_credentials")
	bc, err := bond.NewClient(ctx, bond.WithCredsJSON([]byte(creds)))
	if err != nil {
		s.Fatal("Failed to create a bond client: ", err)
	}
	defer bc.Close()

	var meetingCode string
	func() {
		sctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		meetingCode, err = bc.CreateConference(sctx)
		if err != nil {
			s.Fatal("Failed to create a conference room: ", err)
		}
	}()
	s.Log("Created a room with the code ", meetingCode)

	cr := s.PreValue().(cuj.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	if _, ok := s.Var("record"); ok {
		screenRecorder, err := cuj.NewScreenRecorder(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create ScreenRecorder: ", err)
		}
		defer func() {
			screenRecorder.Stop(ctx)
			dir, ok := testing.ContextOutDir(ctx)
			if ok && dir != "" {
				if _, err := os.Stat(dir); err == nil {
					testing.ContextLogf(ctx, "Saving screen record to %s", dir)
					if err := screenRecorder.SaveInString(ctx, filepath.Join(dir, "screen_record.txt")); err != nil {
						s.Fatal("Failed to save screen record in string: ", err)
					}
					if err := screenRecorder.SaveInBytes(ctx, filepath.Join(dir, "screen_record.webm")); err != nil {
						s.Fatal("Failed to save screen record in bytes: ", err)
					}
				}
			}
			screenRecorder.Release(ctx)
		}()
		screenRecorder.Start(ctx)
	}

	configs := []cuj.MetricConfig{cuj.NewCustomMetricConfig(
		"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
		"percent", perf.SmallerIsBetter, []int64{50, 80})}
	for _, suffix := range []string{"Capturer", "Encoder", "EncoderQueue", "RateLimiter"} {
		configs = append(configs, cuj.NewCustomMetricConfig(
			"WebRTC.Video.DroppedFrames."+suffix, "percent", perf.SmallerIsBetter,
			[]int64{50, 80}))
	}
	if meet.docs {
		configs = append(configs, cuj.NewCustomMetricConfig(
			"Event.Latency.EndToEnd.KeyPress", "microsecond", perf.SmallerIsBetter,
			[]int64{80000, 160000}))
	}

	recorder, err := cuj.NewRecorder(ctx, tconn, configs...)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	meetConn, err := cr.NewConn(ctx, "https://meet.google.com/", cdputil.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open the hangout meet website: ", err)
	}
	defer meetConn.Close()
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	inTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	s.Logf("Is in tablet-mode: %t", inTabletMode)
	if err != nil {
		s.Fatal("Failed to detect it is in tablet-mode or not: ", err)
	}
	var pc pointer.Controller
	if inTabletMode {
		// If it is in tablet mode, ensure it it in landscape orientation.
		// TODO(crbug/1135239): test portrait orientation as well.
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
			defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
		}
		pc, err = pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller: ", err)
		}
	} else {
		// Make it into a maximized window if it is in clamshell-mode.
		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized)
		}); err != nil {
			s.Fatal("Failed to turn all windows into maximized state: ", err)
		}
		pc = pointer.NewMouseController(tconn)
	}
	defer pc.Close()

	// Find the web view of Meet window.
	webview, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeWebView, ClassName: "WebView"}, timeout)
	if err != nil {
		s.Fatal("Failed to find webview: ", err)
	}
	defer webview.Release(closeCtx)

	// Assume that the meeting code is the only textfield in the webpage.
	enter, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeTextField}, timeout)
	if err != nil {
		s.Fatal("Failed to find the meeting code: ", err)
	}
	defer enter.Release(closeCtx)
	if err := enter.StableLeftClick(ctx, &pollOpts); err != nil {
		s.Fatal("Failed to click the input form: ", err)
	}
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	if err := kw.Type(ctx, meetingCode); err != nil {
		s.Fatal("Failed to type the meeting code: ", err)
	}
	if err := kw.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to hit the enter key: ", err)
	}

	if err := func() error {
		// Meet will ask the permission: wait for the permission bubble to appear.
		// Note that there may be some other bubbles, so find only within the main
		// container -- which should be named as "Desk_Container_A", the primary
		// desk.
		container, err := ui.Find(ctx, tconn, ui.FindParams{ClassName: "Desk_Container_A"})
		if err != nil {
			return errors.Wrap(err, "failed to find the container")
		}
		defer container.Release(closeCtx)
		for i := 0; i < 5; i++ {
			bubble, err := container.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "BubbleDialogDelegateView"}, timeout)
			if err != nil {
				// It is fine not finding the bubble.
				return nil
			}
			defer bubble.Release(closeCtx)
			allowButton, err := bubble.Descendant(ctx, ui.FindParams{Name: "Allow", Role: ui.RoleTypeButton})
			if err != nil {
				return errors.Wrap(err, "failed to find the allow button")
			}
			defer allowButton.Release(closeCtx)
			if err := allowButton.StableLeftClick(ctx, &pollOpts); err != nil {
				return errors.Wrap(err, "failed to click the allow button")
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to wait for the next cycle of permission")
			}
		}
		return errors.New("too many permission requests")
	}(); err != nil {
		s.Fatal("Failed to skip the permission requests: ", err)
	}

	defer func() {
		// Close the Meet window to finish meeting.
		if err := meetConn.CloseTarget(closeCtx); err != nil {
			s.Error("Failed to close the meeting: ", err)
		}
	}()

	if meet.docs {
		// Create another browser window and open a Google Docs file.
		docsConn, err := cr.NewConn(ctx, docsURL, cdputil.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open the google docs website: ", err)
		}
		defer docsConn.Close()
		s.Log("Creating a Google Docs window")
	}

	if meet.split {
		// If it is in split mode, snap Meet window to the left and Google Docs window to the right.
		// Enter overview mode to enter split mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to set overview mode: ", err)
		}
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the window list: ", err)
		}
		var meetWindow, docsWindow *ash.Window
		re := regexp.MustCompile(`\bMeet\b`)
		for _, w := range ws {
			if re.MatchString(w.Title) {
				meetWindow = w
			} else {
				docsWindow = w
			}
		}
		// There should be always a Hangouts Meet window.
		if meetWindow == nil {
			s.Fatal("Failed to find Meet window")
		}
		info, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the primary display info: ", err)
		}
		snapLeftPoint := coords.NewPoint(info.WorkArea.Left+1, info.WorkArea.CenterY())
		snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())
		if inTabletMode {
			if docsWindow != nil {
				// In tablet mode, dragging windows from overview needs a long press before dragging.
				if err := pc.Press(ctx, docsWindow.OverviewInfo.Bounds.CenterPoint()); err != nil {
					s.Fatal("Failed to start drag the Google Docs window to snap to the left: ", err)
				}
				if err := testing.Sleep(ctx, time.Second); err != nil {
					s.Fatal("Failed to wait for touch to become long press: ", err)
				}
				if err := pc.Move(ctx, docsWindow.OverviewInfo.Bounds.CenterPoint(), snapLeftPoint, time.Second); err != nil {
					s.Fatal("Failed to drag the Google Docs window: ", err)
				}
				if err := pc.Release(closeCtx); err != nil {
					s.Fatal("Failed to end dragging the Google Docs window: ", err)
				}
			}
			if err := pc.Press(ctx, meetWindow.OverviewInfo.Bounds.CenterPoint()); err != nil {
				s.Fatal("Failed to start drag the Meet window to snap to the right: ", err)
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to wait for touch to become long press: ", err)
			}
			if err := pc.Move(ctx, meetWindow.OverviewInfo.Bounds.CenterPoint(), snapRightPoint, time.Second); err != nil {
				s.Fatal("Failed to drag the Meet window: ", err)
			}
			if err := pc.Release(closeCtx); err != nil {
				s.Fatal("Failed to end dragging the Meet window: ", err)
			}
		} else {
			if docsWindow != nil {
				if err := pointer.Drag(ctx, pc, docsWindow.OverviewInfo.Bounds.CenterPoint(), snapLeftPoint, time.Second); err != nil {
					s.Fatal("Failed to drag the Docs window to snap to the left: ", err)
				}
			}
			if err := pointer.Drag(ctx, pc, meetWindow.OverviewInfo.Bounds.CenterPoint(), snapRightPoint, time.Second); err != nil {
				s.Fatal("Failed to drag the Meet window to snap to the right: ", err)
			}
		}
	} else {
		// If it is not in split screen mode, alt-tab to switch to Meet window on top.
		if err := kw.Accel(ctx, "Alt+Tab"); err != nil {
			s.Fatal("Failed to hit alt-tab and switch to Meet window: ", err)
		}
	}

	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Hide notifications so that they won't overlap with other UI components.
		if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to hide visible notifications")
		}
		shareMessage := "Share this info with people you want in the meeting"
		if err := webview.WaitUntilDescendantExists(ctx, ui.FindParams{Name: shareMessage}, timeout); err == nil {
			// "Share this code" popup appears, dismissing by close button.
			// Close button
			closeButton, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeButton, Name: "Close"}, timeout)
			if err != nil {
				return errors.Wrap(err, "close button should be in the popup")
			}
			if err := closeButton.StableLeftClick(ctx, &pollOpts); err != nil {
				return errors.Wrap(err, "failed to click the close button")
			}
			if err := webview.WaitUntilDescendantGone(ctx, ui.FindParams{Name: shareMessage}, timeout); err != nil {
				return errors.Wrap(err, "popup does not disappear")
			}
		}

		sctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if _, err := bc.AddBots(sctx, meetingCode, meet.num, botDuration); err != nil {
			return errors.Wrap(err, "failed to create bots")
		}
		if err := meetConn.WaitForExpr(ctx, "hrTelemetryApi.isInMeeting() === true"); err != nil {
			return errors.Wrap(err, "failed to wait for entering meeting")
		}

		if !meet.cam {
			if err := meetConn.Eval(ctx, "hrTelemetryApi.setCameraMuted(true)", nil); err != nil {
				return errors.Wrap(err, "failed to turn off camera")
			}
		}

		// Hide notifications so that they won't overlap with other UI components.
		if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to hide visible notifications")
		}
		if err := meetConn.Eval(ctx, fmt.Sprintf("hrTelemetryApi.set%sLayout()", string(meet.layout)), nil); err != nil {
			return errors.Wrapf(err, "failed to set %s layout", string(meet.layout))
		}

		if meet.present {
			if meet.docs == false {
				return errors.New("need a Google Docs tab to present")
			}
			// Start presenting the Google Docs tab.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				exists, err := ui.Exists(ctx, tconn, ui.FindParams{Name: "Chrome Tab", Role: ui.RoleTypeListGrid})
				if err != nil {
					return errors.Wrap(err, "failed to find the list of tabs")
				}
				if exists {
					return nil
				}
				if err := meetConn.Eval(ctx, "hrTelemetryApi.presentation.presentTab()", nil); err != nil {
					return errors.Wrap(err, "failed to start to present a tab")
				}
				return errors.New("presentation hasn't started yet")
			}, &pollOpts); err != nil {
				return errors.Wrap(err, "failed to start presentation")
			}
			tabs, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Chrome Tab", Role: ui.RoleTypeListGrid}, timeout)
			if err != nil {
				return errors.Wrap(err, "failed to find the tab list")
			}
			defer tabs.Release(closeCtx)
			if err := tabs.StableLeftClick(ctx, &pollOpts); err != nil {
				return errors.Wrap(err, "failed to click the tab list")
			}
			// Select the second tab (Google Docs tab) to present.
			for i := 0; i < 2; i++ {
				if err := kw.Accel(ctx, "Down"); err != nil {
					return errors.Wrap(err, "failed to hit the down key")
				}
			}
			if err := kw.Accel(ctx, "Enter"); err != nil {
				return errors.Wrap(err, "failed to hit the enter key")
			}
		}

		prof, err := profiler.Start(ctx, s.OutDir(), profiler.Perf(profiler.PerfRecordOpts()))
		if err != nil {
			if errors.Is(err, profiler.ErrUnsupportedPlatform) {
				s.Log("Profiler is not supported: ", err)
			} else {
				return errors.Wrap(err, "failed to start the profiler")
			}
		}
		if prof != nil {
			defer func() {
				if err := prof.End(); err != nil {
					s.Error("Failed to stop profiler: ", err)
				}
			}()
		}

		errc := make(chan error)
		s.Log("Keeping the meet session for ", meetTimeout)
		go func() {
			// Using goroutine to measure GPU counters asynchronously because:
			// - we will add some other test scenarios (controlling windows / meet sessions).
			// - graphics.MeasureGPUCounters may quit immediately when the hardware or
			//   kernel does not support the reporting mechanism.
			errc <- graphics.MeasureGPUCounters(ctx, meetTimeout, pv)
		}()

		// Simulate notes input.
		if meet.docs {
			if err := kw.Accel(ctx, "Alt+Tab"); err != nil {
				return errors.Wrap(err, "failed to hit alt-tab and focus on Docs tab")
			}
			docsTextfield, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Document content", Role: ui.RoleTypeTextField}, timeout)
			if err != nil {
				return errors.Wrap(err, "failed to find the docs text field")
			}
			defer docsTextfield.Release(closeCtx)
			if err := docsTextfield.StableLeftClick(ctx, &pollOpts); err != nil {
				return errors.Wrap(err, "failed to click on the docs text field")
			}
			if err := kw.Accel(ctx, "Ctrl+A"); err != nil {
				return errors.Wrap(err, "failed to hit ctrl-a and select all text")
			}
			// Type notes three times with 5 seconds pauses.
			for i := 0; i < 3; i++ {
				if err := kw.Type(ctx, notes); err != nil {
					return errors.Wrap(err, "failed to type the notes")
				}
				if err := testing.Sleep(ctx, 5*time.Second); err != nil {
					return errors.Wrap(err, "failed to wait")
				}
			}
			if err := kw.Accel(ctx, "Alt+Tab"); err != nil {
				return errors.Wrap(err, "failed to hit alt-tab and focus back to Meet tab")
			}
		}

		// Ensures that meet session is long enough. graphics.MeasureGPUCounters
		// exits early without errors on ARM where there is no i915 counters.
		if err := testing.Sleep(ctx, meetTimeout); err != nil {
			return errors.Wrap(err, "failed to wait")
		}

		if err := <-errc; err != nil {
			return errors.Wrap(err, "failed to collect GPU counters")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	// Before recording the metrics, check if there is any tab crashed.
	if err := tabChecker.Check(ctx); err != nil {
		s.Fatal("Tab renderer crashed: ", err)
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}
