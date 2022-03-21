// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/bond"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MeetMultiTaskingCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the total performance of multi-tasking with video conferencing CUJ",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      20 * time.Minute,
		Vars: []string{
			"record",
		},
		VarDeps: []string{
			"ui.MeetMultiTaskingCUJ.bond_credentials",
		},
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

// MeetMultiTaskingCUJ measures the performance of critical user journeys for multi-tasking with video conference.
//
// Pre-preparation:
//   - Open a Meet window and grant permissions.
// During recording:
//   - Join the meeting.
//   - Add a participant(bot) to the meeting.
//   - Open a large Google Docs file and scroll down.
//   - Open a large Google Slides file and go down.
//   - Open the Gmail inbox and scroll down.
// After recording:
//   - Record and save metrics.
func MeetMultiTaskingCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout       = 10 * time.Second
		docsURL       = "https://docs.google.com/document/d/1NvbdoWF6OrZxenReot5HptK0xvmzK1WKY5TgifoQtko/edit?usp=sharing"
		docsTimeout   = 30 * time.Second
		slidesURL     = "https://docs.google.com/presentation/d/1lItrhkgBqXF_bsP-tOqbjcbBFa86--m3DT5cLxegR2k/edit?usp=sharing&resourcekey=0-FmuN4N-UehRS2q4CdQzRXA"
		slidesTimeout = 30 * time.Second
		gmailURL      = "https://gmail.com"
		gmailTimeout  = 10 * time.Second
		newTabTitle   = "New Tab"
		meetTimeout   = 2 * time.Minute
		meetLayout    = "Auto"
	)

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)

	var cs ash.ConnSource
	var cr *chrome.Chrome
	var bTconn *chrome.TestConn

	if bt == browser.TypeLacros {
		cr = s.FixtValue().(lacrosfixt.FixtValue).Chrome()
	} else {
		cr = s.FixtValue().(cuj.FixtureData).Chrome
		cs = cr

		var err error
		if bTconn, err = cr.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get TestAPIConn: ", err)
		}
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	if bt == browser.TypeLacros {
		// Launch lacros via shelf.
		f := s.FixtValue().(lacrosfixt.FixtValue)

		l, err := lacros.LaunchFromShelf(ctx, tconn, f.LacrosPath())
		if err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		defer l.Close(ctx)
		cs = l

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get lacros TestAPIConn: ", err)
		}
	}

	if _, ok := s.Var("record"); ok {
		screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create ScreenRecorder: ", err)
		}
		defer func(ctx context.Context) {
			screenRecorder.Stop(ctx)
			dir, ok := testing.ContextOutDir(ctx)
			if ok && dir != "" {
				if _, err := os.Stat(dir); err == nil {
					testing.ContextLogf(ctx, "Saving screen record to %s", dir)
					if err := screenRecorder.SaveInBytes(ctx, filepath.Join(dir, "screen_record.webm")); err != nil {
						s.Fatal("Failed to save screen record in bytes: ", err)
					}
				}
			}
			screenRecorder.Release(ctx)
		}(closeCtx)
		screenRecorder.Start(ctx, tconn)
	}

	inTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to detect it is in tablet-mode or not: ", err)
	}
	var pc pointer.Context
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
			defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
		}
		pc, err = pointer.NewTouch(ctx, tconn)
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
		pc = pointer.NewMouse(tconn)
	}
	defer pc.Close()

	creds := s.RequiredVar("ui.MeetMultiTaskingCUJ.bond_credentials")
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

	meetConn, err := cs.NewConn(ctx, "https://meet.google.com/"+meetingCode, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open the hangout meet website: ", err)
	}
	defer meetConn.Close()
	defer meetConn.CloseTarget(closeCtx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Lacros specific setup.
	if bt == browser.TypeLacros {
		// Close "New Tab" window after creating the meet window.
		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return strings.HasPrefix(w.Title, newTabTitle) && strings.HasPrefix(w.Name, "ExoShellSurface")
		})
		if err != nil {
			s.Fatal("Failed to find New Tab window: ", err)
		}
		if err := w.CloseWindow(ctx, tconn); err != nil {
			s.Fatal("Failed to close New Tab window: ", err)
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

	ui := uiauto.New(tconn)

	// Find the web view of Meet window.
	webview := nodewith.ClassName("ContentsWebView").Role(role.WebView)

	uiLongWait := ui.WithTimeout(time.Minute)
	bubble := nodewith.ClassName("PermissionPromptBubbleView").First()
	allow := nodewith.Name("Allow").Role(role.Button).Ancestor(bubble)
	// Check and grant permissions.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Long wait for permission bubble and break poll loop when it times out.
		if err := uiLongWait.WaitUntilExists(bubble)(ctx); err != nil {
			return nil
		}
		if err := pc.Click(allow)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the allow button")
		}
		return errors.New("granting permissions")
	}, &testing.PollOptions{Interval: time.Second, Timeout: 2 * time.Minute}); err != nil {
		s.Fatal("Failed to grant permissions: ", err)
	}

	configs := []cuj.MetricConfig{
		// Ash metrics config, always collected from ash-chrome.
		cuj.NewCustomMetricConfig(
			"Ash.Smoothness.PercentDroppedFrames_1sWindow", "percent",
			perf.SmallerIsBetter, []int64{50, 80}),
		cuj.NewCustomMetricConfig(
			"Browser.Responsiveness.JankyIntervalsPerThirtySeconds3", "janks",
			perf.SmallerIsBetter, []int64{0, 3}),
		// Browser metrics config, collected from ash-chrome or lacros-chrome
		// depending on the browser being used.
		cuj.NewCustomMetricConfigWithTestConn(
			"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video", "percent",
			perf.SmallerIsBetter, []int64{5, 10}, bTconn),
	}
	for _, suffix := range []string{"Capturer", "Encoder", "EncoderQueue", "RateLimiter"} {
		configs = append(configs, cuj.NewCustomMetricConfigWithTestConn(
			"WebRTC.Video.DroppedFrames."+suffix, "percent", perf.SmallerIsBetter,
			[]int64{50, 80}, bTconn))
	}
	configs = append(configs, cuj.NewCustomMetricConfigWithTestConn(
		"Event.Latency.EndToEnd.KeyPress", "microsecond", perf.SmallerIsBetter,
		[]int64{80000, 400000}, bTconn))
	configs = append(configs, cuj.NewCustomMetricConfigWithTestConn(
		"Event.Latency.EndToEnd.Mouse", "microsecond", perf.SmallerIsBetter,
		[]int64{80000, 400000}, bTconn))
	configs = append(configs, cuj.NewCustomMetricConfigWithTestConn(
		"PageLoad.PaintTiming.NavigationToFirstContentfulPaint", "ms",
		perf.SmallerIsBetter, []int64{4000, 5000}, bTconn))
	configs = append(configs, cuj.NewCustomMetricConfigWithTestConn(
		"PageLoad.PaintTiming.NavigationToLargestContentfulPaint2", "ms",
		perf.SmallerIsBetter, []int64{4000, 5000}, bTconn))

	pv := perf.NewValues()
	recorder, err := cuj.NewRecorder(ctx, cr, nil, configs...)
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Hide notifications so that they won't overlap with other UI components.
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close all notifications")
		}
		shareMessage := "Share this info with people you want in the meeting"
		if err := ui.WaitUntilExists(nodewith.Name(shareMessage).Ancestor(webview))(ctx); err == nil {
			// "Share this code" popup appears, dismissing by close button.
			if err := uiauto.Combine(
				"click the close button and wait for the popup to disappear",
				pc.Click(nodewith.Name("Close").Role(role.Button).Ancestor(webview)),
				ui.WaitUntilGone(nodewith.Name(shareMessage).Ancestor(webview)),
			)(ctx); err != nil {
				return err
			}
		}

		sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		// Add 30 seconds to the bot duration to make sure that bots do not leave
		// slightly earlier than the test scenario.
		if _, err := bc.AddBots(sctx, meetingCode, 1, meetTimeout+30*time.Second); err != nil {
			return errors.Wrap(err, "failed to create bots")
		}
		if err := meetConn.WaitForExpr(ctx, "hrTelemetryApi.isInMeeting() === true"); err != nil {
			return errors.Wrap(err, "failed to wait for entering meeting")
		}
		if err := meetConn.Eval(ctx, "hrTelemetryApi.setMicMuted(false)", nil); err != nil {
			return errors.Wrap(err, "failed to turn on mic")
		}
		if err := meetConn.Eval(ctx, "hrTelemetryApi.setCameraMuted(false)", nil); err != nil {
			return errors.Wrap(err, "failed to turn on camera")
		}

		// Hide notifications so that they won't overlap with other UI components.
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close all notifications")
		}
		if err := meetConn.Eval(ctx, fmt.Sprintf("hrTelemetryApi.set%sLayout()", meetLayout), nil); err != nil {
			return errors.Wrapf(err, "failed to set %s layout", meetLayout)
		}

		// 1. Multi-tasking with Google Docs by opening a large Docs file and scrolling through the file.

		docsConn, err := cs.NewConn(ctx, docsURL, browser.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open the google docs website")
		}
		defer docsConn.Close()
		defer docsConn.CloseTarget(closeCtx)

		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized)
		}); err != nil {
			return errors.Wrap(err, "failed to turn all windows into maximized state")
		}

		// Pop-up content regarding view history privacy might show up.
		privacyButton := nodewith.Name("I understand").Role(role.Button)
		if err := ui.IfSuccessThen(ui.WaitUntilExists(privacyButton), ui.LeftClick(privacyButton))(ctx); err != nil {
			return errors.Wrap(err, "failed to click the privacy button")
		}

		// Scroll down the Docs file.
		fingerSpacing := tpw.Width() / 4
		end := time.Now().Add(docsTimeout)
		// Swipe and scroll down the spreadsheet.
		s.Logf("Scrolling down the Google Sheets file for %d seconds", int(docsTimeout.Seconds()))
		for end.Sub(time.Now()).Seconds() > 0 {
			// Double swipe from the middle bottom to the middle top of the touchpad.
			var startX, startY, endX, endY input.TouchCoord
			startX, startY, endX, endY = tpw.Width()/2, 1, tpw.Width()/2, tpw.Height()-1
			fingerNum := 2
			if err := tw.Swipe(ctx, startX, startY, endX, endY, fingerSpacing,
				fingerNum, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to swipe")
			}
		}

		// Navigate away to record PageLoad.PaintTiming.NavigationToLargestContentfulPaint2.
		if err := docsConn.Navigate(ctx, "chrome://version"); err != nil {
			return errors.Wrap(err, "failed to navigate to chrome://version")
		}

		// 2. Multi-tasking with Google Docs by opening a large Slides file and going through the deck.

		slidesConn, err := cs.NewConn(ctx, slidesURL, browser.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open the google slides website")
		}
		defer slidesConn.Close()
		defer slidesConn.CloseTarget(closeCtx)

		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized)
		}); err != nil {
			return errors.Wrap(err, "failed to turn all windows into maximized state")
		}

		// Go through the Slides deck.
		s.Logf("Going through the Google Slides file for %d seconds", int(slidesTimeout.Seconds()))
		end = time.Now().Add(slidesTimeout)
		for end.Sub(time.Now()).Seconds() > 0 {
			if err := uiauto.Combine(
				"sleep and press down",
				action.Sleep(time.Second),
				kw.AccelAction("Down"),
			)(ctx); err != nil {
				return err
			}
		}

		// Navigate away to record PageLoad.PaintTiming.NavigationToLargestContentfulPaint2.
		if err := slidesConn.Navigate(ctx, "chrome://version"); err != nil {
			return errors.Wrap(err, "failed to navigate to chrome://version")
		}

		// 3. Multi-tasking with Gmail by opening the inbox and scrolling down.

		gmailConn, err := cs.NewConn(ctx, gmailURL, browser.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open the Gmail inbox")
		}
		defer gmailConn.Close()
		defer gmailConn.CloseTarget(closeCtx)

		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized)
		}); err != nil {
			return errors.Wrap(err, "failed to turn all windows into maximized state")
		}

		// Swipe and scroll down the gmail inbox.
		end = time.Now().Add(gmailTimeout)
		s.Logf("Scrolling down the Gmail inbox for %d seconds", int(gmailTimeout.Seconds()))
		for end.Sub(time.Now()).Seconds() > 0 {
			// Double swipe from the middle bottom to the middle top of the touchpad.
			var startX, startY, endX, endY input.TouchCoord
			startX, startY, endX, endY = tpw.Width()/2, 1, tpw.Width()/2, tpw.Height()-1
			fingerNum := 2
			if err := tw.Swipe(ctx, startX, startY, endX, endY, fingerSpacing,
				fingerNum, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to swipe")
			}
		}

		// Navigate away to record PageLoad.PaintTiming.NavigationToLargestContentfulPaint2.
		if err := gmailConn.Navigate(ctx, "chrome://version"); err != nil {
			return errors.Wrap(err, "failed to navigate to chrome://version")
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}

}
