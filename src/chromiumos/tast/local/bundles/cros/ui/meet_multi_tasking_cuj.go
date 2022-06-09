// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MeetMultiTaskingCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the total performance of multi-tasking with video conferencing CUJ",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      10 * time.Minute,
		Vars: []string{
			"record",
		},
		VarDeps: []string{
			"ui.bond_credentials",
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

// ashMetricConfigs returns metrics to be always collected from ash-chrome.
func ashMetricConfigs() []cujrecorder.MetricConfig {
	return []cujrecorder.MetricConfig{
		cujrecorder.NewCustomMetricConfig(
			"Ash.Smoothness.PercentDroppedFrames_1sWindow", "percent",
			perf.SmallerIsBetter),
		cujrecorder.NewCustomMetricConfig(
			"Browser.Responsiveness.JankyIntervalsPerThirtySeconds3", "janks",
			perf.SmallerIsBetter),
	}
}

// browserMetricConfigs returns browser metrics config, collected from
// ash-chrome or lacros-chrome depending on the browser being used.
func browserMetricConfigs() []cujrecorder.MetricConfig {
	configs := []cujrecorder.MetricConfig{
		cujrecorder.NewCustomMetricConfig(
			"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video", "percent",
			perf.SmallerIsBetter),
		cujrecorder.NewCustomMetricConfig(
			"Event.Latency.EndToEnd.KeyPress", "microsecond", perf.SmallerIsBetter),
		cujrecorder.NewCustomMetricConfig(
			"Event.Latency.EndToEnd.Mouse", "microsecond", perf.SmallerIsBetter),
		cujrecorder.NewCustomMetricConfig(
			"PageLoad.PaintTiming.NavigationToFirstContentfulPaint", "ms",
			perf.SmallerIsBetter),
		cujrecorder.NewCustomMetricConfig(
			"PageLoad.PaintTiming.NavigationToLargestContentfulPaint2", "ms",
			perf.SmallerIsBetter),
	}
	for _, suffix := range []string{"Capturer", "Encoder", "EncoderQueue", "RateLimiter"} {
		configs = append(configs, cujrecorder.NewCustomMetricConfig(
			"WebRTC.Video.DroppedFrames."+suffix, "percent", perf.SmallerIsBetter))
	}
	return configs
}

// MeetMultiTaskingCUJ measures the performance of critical user journeys for multi-tasking with video conference.
//
// Pre-preparation:
//   - Open a Meet window and grant permissions.
// During recording:
//   - Join the meeting.
//   - Add a participant (bot) to the meeting.
//   - Open a large Google Docs file and scroll down.
//   - Open a large Google Slides file and go down.
//   - Open the Gmail inbox and scroll down.
// After recording:
//   - Record and save metrics.
func MeetMultiTaskingCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout             = 10 * time.Second
		docsURL             = "https://docs.google.com/document/d/1NvbdoWF6OrZxenReot5HptK0xvmzK1WKY5TgifoQtko/edit?usp=sharing"
		docsScrollTimeout   = 30 * time.Second
		slidesURL           = "https://docs.google.com/presentation/d/1lItrhkgBqXF_bsP-tOqbjcbBFa86--m3DT5cLxegR2k/edit?usp=sharing&resourcekey=0-FmuN4N-UehRS2q4CdQzRXA"
		slidesScrollTimeout = 30 * time.Second
		gmailURL            = "https://gmail.com"
		gmailScrollTimeout  = 10 * time.Second
		newTabTitle         = "New Tab"
		meetTimeout         = 2 * time.Minute
		meetLayout          = "Auto"
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

	if _, ok := s.Var("record"); ok {
		screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create ScreenRecorder: ", err)
		}
		if err := screenRecorder.Start(ctx, tconn); err != nil {
			screenRecorder.Release(closeCtx)
			s.Fatal("Failed to start ScreenRecorder: ", err)
		}
		defer uiauto.ScreenRecorderStopSaveRelease(closeCtx, screenRecorder, filepath.Join(s.OutDir(), "screen_record.webm"))
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
			defer display.SetDisplayRotationSync(closeCtx, tconn, info.ID, display.Rotate0)
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

	creds := s.RequiredVar("ui.bond_credentials")
	bc, err := bond.NewClient(ctx, bond.WithCredsJSON([]byte(creds)))
	if err != nil {
		s.Fatal("Failed to create a bond client: ", err)
	}
	defer bc.Close()

	var meetingCode string
	{
		sctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		meetingCode, err = bc.CreateConference(sctx)
		if err != nil {
			s.Fatal("Failed to create a conference room: ", err)
		}
	}
	s.Log("Created a room with the code ", meetingCode)

	sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Add 30 seconds to the bot duration to make sure that bots do not leave
	// slightly earlier than the test scenario.
	if _, _, err := bc.AddBots(sctx, meetingCode, 1, meetTimeout+30*time.Second); err != nil {
		s.Fatal("Failed to create 1 bot: ", err)
	}
	defer func(ctx context.Context) {
		s.Log("Removing all bots from the call")
		if _, _, err := bc.RemoveAllBots(ctx, meetingCode); err != nil {
			s.Fatal("Failed to remove all bots: ", err)
		}
	}(closeCtx)

	meetConn, err := cs.NewConn(ctx, "https://meet.google.com/"+meetingCode, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open the hangout meet website: ", err)
	}
	defer meetConn.Close()
	defer meetConn.CloseTarget(closeCtx)
	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, tconn)

	// Lacros specific setup.
	if bt == browser.TypeLacros {
		if err := l.Browser().CloseWithURL(ctx, chrome.NewTabURL); err != nil {
			s.Fatal("Failed to close blank tab: ", err)
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
		if uiLongWait.WaitUntilExists(bubble)(ctx) != nil {
			return nil
		}
		if err := pc.Click(allow)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the allow button")
		}
		return errors.New("granting permissions")
	}, &testing.PollOptions{Interval: time.Second, Timeout: 2 * time.Minute}); err != nil {
		s.Fatal("Failed to grant permissions: ", err)
	}

	rightSnapAllWindows := func() error {
		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateRightSnapped)
		}); err != nil {
			return errors.Wrap(err, "failed to turn all windows into right snapped state")
		}
		return nil
	}

	leftSnapNonRightSnappedWindows := func() error {
		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			if w.State != ash.WindowStateRightSnapped {
				return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateLeftSnapped)
			}
			return nil
		}); err != nil {
			return errors.Wrap(err, "failed to turn non right snapped windows into left snapped state")
		}
		return nil
	}

	ensureElementGetsScrolled := func(conn *chrome.Conn, element string) error {
		var scrollTop int
		if err := conn.Eval(ctx, fmt.Sprintf("parseInt(%s.scrollTop)", element), &scrollTop); err != nil {
			return errors.Wrap(err, "failed to get the number of pixels that the scrollbar is scrolled vertically")
		}
		if scrollTop == 0 {
			return errors.Errorf("%s is not getting scrolled", element)
		}
		return nil
	}

	pv := perf.NewValues()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a new CUJ recorder: ", err)
	}
	defer func() {
		if err := recorder.Close(closeCtx); err != nil {
			s.Error("Failed to stop recorder: ", err)
		}
	}()

	if err := recorder.AddCollectedMetrics(tconn, ashMetricConfigs()...); err != nil {
		s.Fatal("Failed to add Ash recorded metrics: ", err)
	}

	if err := recorder.AddCollectedMetrics(bTconn, browserMetricConfigs()...); err != nil {
		s.Fatal("Failed to add Browser recorded metrics: ", err)
	}
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Hide notifications so that they won't overlap with other UI components.
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close all notifications")
		}
		shareMessage := "Share this info with people you want in the meeting"
		if ui.WaitUntilExists(nodewith.Name(shareMessage).Ancestor(webview))(ctx) == nil {
			// "Share this code" popup appears, dismissing by close button.
			if err := uiauto.Combine(
				"click the close button and wait for the popup to disappear",
				pc.Click(nodewith.Name("Close").Role(role.Button).Ancestor(webview)),
				ui.WaitUntilGone(nodewith.Name(shareMessage).Ancestor(webview)),
			)(ctx); err != nil {
				return err
			}
		}

		if err := meetConn.WaitForExpr(ctx, "hrTelemetryApi.isInMeeting()"); err != nil {
			return errors.Wrap(err, "failed to wait for entering meeting")
		}
		if err := meetConn.Eval(ctx, "hrTelemetryApi.setMicMuted(false)", nil); err != nil {
			return errors.Wrap(err, "failed to turn on mic")
		}
		if err := meetConn.Eval(ctx, "hrTelemetryApi.setCameraMuted(false)", nil); err != nil {
			return errors.Wrap(err, "failed to turn on camera")
		}

		var participantCount int
		if err := meetConn.Eval(ctx, "hrTelemetryApi.getParticipantCount()", &participantCount); err != nil {
			return errors.Wrap(err, "failed to get participant count")
		}
		if participantCount != 2 {
			return errors.Errorf("got %d participants, expected 2", participantCount)
		}

		// Hide notifications so that they won't overlap with other UI components.
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close all notifications")
		}
		if err := meetConn.Eval(ctx, fmt.Sprintf("hrTelemetryApi.set%sLayout()", meetLayout), nil); err != nil {
			return errors.Wrapf(err, "failed to set %s layout", meetLayout)
		}

		if err := rightSnapAllWindows(); err != nil {
			return err
		}

		// 1. Multi-tasking with Google Docs by opening a large Docs file and scrolling through the file.
		// ================================================================================

		docsConn, err := cs.NewConn(ctx, docsURL, browser.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open the google docs website")
		}
		defer docsConn.Close()
		defer docsConn.CloseTarget(closeCtx)

		// Left snap the Docs window.
		if err := leftSnapNonRightSnappedWindows(); err != nil {
			return err
		}

		// Pop-up content regarding paperless mode might show up.
		gotItButton := nodewith.Name("Got it!").Role(role.Button)
		if err := uiauto.IfSuccessThen(ui.WithTimeout(10*time.Second).WaitUntilExists(gotItButton), ui.LeftClick(gotItButton))(ctx); err != nil {
			return errors.Wrap(err, "failed to click the Got it button")
		}

		// Move mouse to the left side of screen so that mouse will be on top of the left snapped window.
		info, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get the primary display info")
		}
		leftSidePoint := coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width/4, info.WorkArea.CenterY())
		if err := mouse.Move(tconn, leftSidePoint, 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move mouse to the left side of the screen")
		}

		// Scroll down the Docs file.
		s.Logf("Scrolling down the Google Docs file for %s", docsScrollTimeout)
		if err := cuj.ScrollDownFor(ctx, tpw, tw, 500*time.Millisecond, docsScrollTimeout); err != nil {
			return err
		}

		// Ensure the file gets scrolled.
		if err := ensureElementGetsScrolled(docsConn, "document.getElementsByClassName('kix-appview-editor')[0]"); err != nil {
			return err
		}

		// Navigate away to record PageLoad.PaintTiming.NavigationToLargestContentfulPaint2.
		if err := docsConn.Navigate(ctx, "chrome://version"); err != nil {
			return errors.Wrap(err, "failed to navigate to chrome://version")
		}

		// 2. Multi-tasking with Google Slides by opening a large Slides file and going through the deck.
		// ================================================================================

		slidesConn, err := cs.NewConn(ctx, slidesURL, browser.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open the google slides website")
		}
		defer slidesConn.Close()
		defer slidesConn.CloseTarget(closeCtx)

		// Left snap the Slides window.
		if err := leftSnapNonRightSnappedWindows(); err != nil {
			return err
		}

		// Go through the Slides deck.
		s.Logf("Going through the Google Slides file for %s", slidesScrollTimeout)
		if err := cuj.RepeatKeyPressFor(ctx, kw, "Down", time.Second, slidesScrollTimeout); err != nil {
			return err
		}

		// Ensure the slides deck gets scrolled.
		if err := ensureElementGetsScrolled(slidesConn, "document.getElementsByClassName('punch-filmstrip-scroll')[0]"); err != nil {
			return err
		}

		// Navigate away to record PageLoad.PaintTiming.NavigationToLargestContentfulPaint2.
		if err := slidesConn.Navigate(ctx, "chrome://version"); err != nil {
			return errors.Wrap(err, "failed to navigate to chrome://version")
		}

		// 3. Multi-tasking with Gmail by opening the inbox and scrolling down.
		// ================================================================================

		gmailConn, err := cs.NewConn(ctx, gmailURL, browser.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open the Gmail inbox")
		}
		defer gmailConn.Close()
		defer gmailConn.CloseTarget(closeCtx)

		// Left snap the Gmail window.
		if err := leftSnapNonRightSnappedWindows(); err != nil {
			return err
		}

		// Scroll down the Gmail inbox.
		s.Logf("Scrolling down the Gmail inbox for %s", gmailScrollTimeout)
		if err := cuj.ScrollDownFor(ctx, tpw, tw, 500*time.Millisecond, gmailScrollTimeout); err != nil {
			return err
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
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}
