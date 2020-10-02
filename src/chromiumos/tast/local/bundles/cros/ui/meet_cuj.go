// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/bond"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// meetTest specifies the setting of a Hangouts Meet journey.
type meetTest struct {
	num    int    // Number of the participants in the meeting.
	layout string // Type of the layout in the meeting: Spotlight/Tiled/Sidebar/Auto.
	pre    bool   // Whether it is presenting or not.
	docs   bool   // Whether it is running with a Google Docs window.
	split  bool   // Whether it is in split screen mode.
	cam    bool   // Whether the camera is on or not.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MeetCUJ,
		Desc:         "Measures the performance of critical user journey for Google Meet",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      3 * time.Minute,
		Pre:          cuj.LoggedInToCUJUser(),
		Vars: []string{
			"mute",
			"ui.MeetCUJ.bond_credentials",
			"ui.cuj_username",
			"ui.cuj_password",
		},
		Params: []testing.Param{{
			Name: "base_case",
			Val: meetTest{
				num:    4,
				layout: "Tiled",
				pre:    false,
				docs:   false,
				split:  false,
				cam:    false,
			},
		}, {
			Name: "worst_case",
			Val: meetTest{
				num:    4,
				layout: "Tiled",
				pre:    true,
				docs:   true,
				split:  true,
				cam:    true,
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
//   - Turn off mic (if necessary).
// During recording:
//   - Join the meeting.
//   - Add participants(bots) to the meeting.
//   - Set up the layout.
//   - Max out the number of the maximum tiles (if necessary).
//   - Start to present (if necessary).
//   - Wait for 30 seconds before ending the meeting.
func MeetCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout     = 10 * time.Second
		botDuration = 2 * time.Minute
		docsURL     = "https://docs.google.com/document/d/1qREN9w1WgjgdGYBT_eEtE6T21ErlW_4nQoBJVhrR1S0/edit"
	)

	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: timeout}

	meet := s.Param().(meetTest)

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

	meetConn, err := cr.NewConn(ctx, "https://meet.google.com/", cdputil.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open the hangout meet website: ", err)
	}
	defer meetConn.Close()
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Make it into a maximized window if it is in clamshell-mode.
	inTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	s.Logf("Is in tablet-mode: %t", inTabletMode)
	if err != nil {
		s.Fatal("Failed to detect it is in tablet-mode or not: ", err)
	}
	if !inTabletMode {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain the window list: ", err)
		}
		for _, w := range ws {
			if _, err := ash.SetWindowState(ctx, tconn, w.ID, ash.WMEventMaximize); err != nil {
				s.Fatalf("Failed to change the window %s (%d) to maximized: %v", w.Name, w.ID, err)
			}
		}
	}

	var pc pointer.Controller
	if !inTabletMode {
		pc = pointer.NewMouseController(tconn)
	} else {
		pc, err = pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller: ", err)
		}
	}
	defer pc.Close()

	// Find the web view of Meet window.
	webview, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeWebView, ClassName: "WebView"}, timeout)
	if err != nil {
		s.Fatal("Failed to find webview: ", err)
	}
	defer webview.Release(ctx)

	// Assume that the meeting code is the only textfield in the webpage.
	enter, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeTextField}, timeout)
	if err != nil {
		s.Fatal("Failed to find the meeting code: ", err)
	}
	defer enter.Release(ctx)
	if err := enter.LeftClick(ctx); err != nil {
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
		s.Fatal("Failed to type the enter key: ", err)
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
		defer container.Release(ctx)
		for i := 0; i < 5; i++ {
			bubble, err := container.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "BubbleDialogDelegateView"}, timeout)
			if err != nil {
				// It is fine not finding the bubble.
				return nil
			}
			defer bubble.Release(ctx)
			allowButton, err := bubble.Descendant(ctx, ui.FindParams{Name: "Allow", Role: ui.RoleTypeButton})
			if err != nil {
				return errors.Wrap(err, "failed to find the allow button")
			}
			defer allowButton.Release(ctx)
			if err := allowButton.LeftClick(ctx); err != nil {
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

	if meet.docs {
		// Create another browser window and open a Google Docs file.
		docsConn, err := cr.NewConn(ctx, docsURL, cdputil.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open the google docs website: ", err)
		}
		defer docsConn.Close()
		s.Log("Create a Google Docs window")
	}

	// Enter overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the window list: ", err)
	}
	var meetWindow, docsWindow *ash.Window
	for _, w := range ws {
		if w.Title == "Chrome - Meet - "+meetingCode {
			meetWindow = w
		} else {
			docsWindow = w
		}
	}
	// There should be always a Hangouts Meet window.
	if meetWindow == nil {
		s.Fatal("Failed to find Meet window")
	}
	if meet.split {
		// If it is in split mode, snap Meet window to the left and Ggoogle Docs window to the right.
		info, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the primary display info: ", err)
		}
		snapLeftPoint := coords.NewPoint(info.WorkArea.Left+1, info.WorkArea.CenterY())
		snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())
		if inTabletMode {
			// In tablet mode, dragging windows from overview needs a long press before dragging.
			if err := pc.Press(ctx, meetWindow.OverviewInfo.Bounds.CenterPoint()); err != nil {
				s.Fatal("Failed to start drag the Meet window to snap to the right: ", err)
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to wait for touch to become long press: ", err)
			}
			if err := pc.Move(ctx, meetWindow.OverviewInfo.Bounds.CenterPoint(), snapRightPoint, time.Second); err != nil {
				s.Fatal("Failed to the Meet window to snap: ", err)
			}
			if err := pc.Release(ctx); err != nil {
				s.Fatal("Failed to end dragging the Meet window to snap: ", err)
			}
			if docsWindow != nil {
				if err := pc.Press(ctx, docsWindow.OverviewInfo.Bounds.CenterPoint()); err != nil {
					s.Fatal("Failed to start drag the Google Docs window to snap to the left: ", err)
				}
				if err := testing.Sleep(ctx, time.Second); err != nil {
					s.Fatal("Failed to wait for touch to become long press: ", err)
				}
				if err := pc.Move(ctx, docsWindow.OverviewInfo.Bounds.CenterPoint(), snapLeftPoint, time.Second); err != nil {
					s.Fatal("Failed to the Google Docs window to snap: ", err)
				}
				if err := pc.Release(ctx); err != nil {
					s.Fatal("Failed to end dragging the Google Docs window to snap: ", err)
				}
			}
		} else {
			if err := pointer.Drag(ctx, pc, meetWindow.OverviewInfo.Bounds.CenterPoint(), snapRightPoint, time.Second); err != nil {
				s.Fatal("Failed to drag the Meet window to snap to the left", err)
			}
			if docsWindow != nil {
				if err := pointer.Drag(ctx, pc, docsWindow.OverviewInfo.Bounds.CenterPoint(), snapLeftPoint, time.Second); err != nil {
					s.Fatal("Failed to drag the Docs window to snap to the right", err)
				}
			}
		}
	} else {
		// If it is not in split screen mode, click the Meet window and make it on top.
		if err := pointer.Click(ctx, pc, meetWindow.OverviewInfo.Bounds.CenterPoint()); err != nil {
			s.Fatal("Failed to click the Meet window", err)
		}
	}

	// If camera needs to be turned off, it is turned off before joining the meeting.
	if !meet.cam {
		turnOffCam, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Turn off camera (ctrl + e)", Role: ui.RoleTypeButton}, timeout)
		if err != nil {
			s.Fatal("Failed to find turn-off-camera button: ", err)
		}
		defer turnOffCam.Release(ctx)
		if err := turnOffCam.StableLeftClick(ctx, &pollOpts); err != nil {
			s.Fatal("Failed to click the turn-off-camera button: ", err)
		}
		// Make sure the camera is turned off.
		if _, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Camera is off"}, timeout); err != nil {
			s.Fatal("Failed to turn off the camera: ", err)
		}
	}

	askToJoin, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Ask to join"}, timeout)
	if err != nil {
		s.Fatal("Failed to find join-now button: ", err)
	}
	defer askToJoin.Release(ctx)

	configs := []cuj.MetricConfig{cuj.NewCustomMetricConfig(
		"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
		"percent", perf.SmallerIsBetter, []int64{50, 80})}
	for _, suffix := range []string{"Capturer", "Encoder", "EncoderQueue", "RateLimiter"} {
		configs = append(configs, cuj.NewCustomMetricConfig(
			"WebRTC.Video.DroppedFrames."+suffix, "percent", perf.SmallerIsBetter,
			[]int64{50, 80}))
	}
	recorder, err := cuj.NewRecorder(ctx, configs...)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	pv := perf.NewValues()
	if err := recorder.Run(ctx, tconn, func(ctx context.Context) error {
		// Hide notifications so that they won't overlap with other UI components.
		if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
			s.Fatal("Failed to hide visible notifications: ", err)
		}
		if err := askToJoin.StableLeftClick(ctx, &pollOpts); err != nil {
			return errors.Wrap(err, `failed to click "Join now" button`)
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

		// Change the layout after entering the meeting.
		if err := webview.WaitUntilDescendantExists(ctx, ui.FindParams{Name: "Present now", Role: ui.RoleTypePopUpButton}, timeout); err != nil {
			s.Fatal("Failed to wait for the meeting page to show up: ", err)
		}
		// Hide notifications so that they won't overlap with other UI components.
		if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
			s.Fatal("Failed to hide visible notifications: ", err)
		}
		// Make sure the bottom panel is active by moving mouse on Meet page.
		meetArea, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Meet - " + meetingCode, Role: ui.RoleTypeRootWebArea}, timeout)
		if err != nil {
			s.Fatal("Failed to find the Meet area: ", err)
		}
		defer meetArea.Release(ctx)
		if err := pc.Move(ctx, meetArea.Location.CenterPoint(), meetArea.Location.CenterPoint().Add(coords.Point{X: 5, Y: 5}), 0); err != nil {
			s.Fatal("Failed to simulate a move event on Meet area: ", err)
		}
		// Now the options button is visible.
		options, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "More options", Role: ui.RoleTypePopUpButton}, timeout)
		if err != nil {
			s.Fatal("Failed to find the options button: ", err)
		}
		defer options.Release(ctx)
		if err := options.StableLeftClick(ctx, &pollOpts); err != nil {
			s.Fatal("Failed to click the options button: ", err)
		}
		changeLayout, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Change layout", Role: ui.RoleTypeMenuItem}, timeout)
		if err != nil {
			s.Fatal("Failed to find the change-layout item (likely because options button was not clicked correctly): ", err)
		}
		defer changeLayout.Release(ctx)
		if err := changeLayout.StableDoubleClick(ctx, &pollOpts); err != nil {
			s.Fatal("Failed to click the change-layout button: ", err)
		}
		layout, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: meet.layout, Role: ui.RoleTypeRadioButton}, timeout)
		if err != nil {
			s.Fatal("Failed to find the layout button: ", err)
		}
		defer layout.Release(ctx)
		if err := layout.StableLeftClick(ctx, &pollOpts); err != nil {
			s.Fatal("Failed to click the layout item: ", err)
		}
		if meet.layout == "Auto" || meet.layout == "Tiled" {
			if err := kw.Accel(ctx, "Tab"); err != nil {
				s.Fatal("Failed to type the tab key: ", err)
			}
			// Max up the number of the maximum tiles to display.
			for i := 0; i < 5; i++ {
				if err := kw.Accel(ctx, "Right"); err != nil {
					s.Fatal("Failed to type the right key: ", err)
				}
			}
		}
		// Close the layout change view.
		close, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Close", Role: ui.RoleTypeButton}, timeout)
		if err != nil {
			s.Fatal("Failed to find the close button: ", err)
		}
		defer close.Release(ctx)
		if err := close.StableLeftClick(ctx, &pollOpts); err != nil {
			s.Fatal("Failed to click the close button: ", err)
		}

		if meet.pre {
			// Start presenting the Meet tab itself.
			present, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Present now", Role: ui.RoleTypePopUpButton}, timeout)
			if err != nil {
				s.Fatal("Failed to find the present button: ", err)
			}
			defer present.Release(ctx)
			if err := present.LeftClickUntil(ctx,
				func(ctx context.Context) (bool, error) {
					return ui.Exists(ctx, tconn, ui.FindParams{Name: "A Chrome tab"})
				}, &pollOpts); err != nil {
				s.Fatal("Failed to click the present button: ", err)
			}
			presentTab, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "A Chrome tab"}, timeout)
			if err != nil {
				s.Fatal("Failed to find the present-a-chrome-tab item: ", err)
			}
			defer presentTab.Release(ctx)
			if err := presentTab.LeftClickUntil(ctx,
				func(ctx context.Context) (bool, error) {
					return ui.Exists(ctx, tconn, ui.FindParams{Name: "Meet - " + meetingCode, Role: ui.RoleTypeCell})
				}, &pollOpts); err != nil {
				s.Fatal("Failed to click the present-a-chrome-tab button: ", err)
			}
			tabs, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Chrome Tab", Role: ui.RoleTypeListGrid}, timeout)
			// meetTab, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "AXVirtualView", Role: ui.RoleTypeCell}, timeout)
			if err != nil {
				s.Fatal("Failed to find the tab list: ", err)
			}
			defer tabs.Release(ctx)
			if err := tabs.StableDoubleClick(ctx, &pollOpts); err != nil {
				s.Fatal("Failed to click the tab list:", err)
			}
			// Select the first tab (Meet tab) to present.
			if err := kw.Accel(ctx, "Up"); err != nil {
				s.Fatal("Failed to type the up key: ", err)
			}
			if err := kw.Accel(ctx, "Enter"); err != nil {
				s.Fatal("Failed to type the enter key: ", err)
			}
		}

		errc := make(chan error)
		go func() {
			// Using goroutine to measure GPU counters asynchronously because:
			// - we will add some test scenarios (controlling windows / meet sessions)
			//   rather than just sleeping.
			// - graphics.MeasureGPUCounters may quit immediately when the hardware or
			//   kernel does not support the reporting mechanism.
			errc <- graphics.MeasureGPUCounters(ctx, 30*time.Second, pv)
		}()
		s.Log("Waiting for 30 seconds")
		if err := testing.Sleep(ctx, 30*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait")
		}

		// Close the Meet window to finish meeting.
		if err := meetConn.CloseTarget(ctx); err != nil {
			return errors.Wrap(err, "failed to close Meet")
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

	if err := recorder.Record(pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}
