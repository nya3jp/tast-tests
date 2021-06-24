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
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/profiler"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type meetLayoutType string

const (
	meetLayoutSpotlight meetLayoutType = "Spotlight"
	meetLayoutTiled     meetLayoutType = "Tiled"
	meetLayoutSidebar   meetLayoutType = "Sidebar"
	meetLayoutAuto      meetLayoutType = "Auto"
)

// meetTest specifies the setting of a Hangouts Meet journey. More info at go/cros-meet-tests.
type meetTest struct {
	num      int            // Number of the participants in the meeting.
	layout   meetLayoutType // Type of the layout in the meeting.
	present  bool           // Whether it is presenting the Google Docs or not. It can not be true if docs is false.
	docs     bool           // Whether it is running with a Google Docs window.
	jamboard bool           // Whether it is running with a Jamboard window.
	split    bool           // Whether it is in split screen mode. It can not be true if docs is false.
	cam      bool           // Whether the camera is on or not.
	power    bool           // Whether to collect power metrics.
	duration time.Duration  // Duration of the meet call. Must be less than test timeout.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MeetCUJ,
		Desc:         "Measures the performance of critical user journey for Google Meet",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc", caps.BuiltinOrVividCamera},
		Fixture:      "loggedInToCUJUser",
		Vars: []string{
			"mute",
			"record",
			"meeting_code",
			"ui.MeetCUJ.bond_credentials",
			"ui.MeetCUJ.doc",
		},
		Params: []testing.Param{{
			// Base case. Note this runs a 30 min meet call.
			Name:    "4p",
			Timeout: 37 * time.Minute,
			Val: meetTest{
				num:      4,
				layout:   meetLayoutTiled,
				cam:      true,
				duration: 30 * time.Minute,
			},
		}, {
			// Small meeting.
			Name:    "4p_present_notes_split",
			Timeout: 7 * time.Minute,
			Val: meetTest{
				num:     4,
				layout:  meetLayoutTiled,
				present: true,
				docs:    true,
				split:   true,
				cam:     true,
			},
		}, {
			// Big meeting.
			Name:    "16p",
			Timeout: 7 * time.Minute,
			Val: meetTest{
				num:    16,
				layout: meetLayoutTiled,
				cam:    true,
			},
		}, {
			// Big meeting with notes.
			Name:    "16p_notes",
			Timeout: 7 * time.Minute,
			Val: meetTest{
				num:    16,
				layout: meetLayoutTiled,
				docs:   true,
				split:  true,
				cam:    true,
			},
		}, {
			// 4p power test.
			Name:              "power_4p",
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: meetTest{
				num:    4,
				layout: meetLayoutTiled,
				cam:    true,
				power:  true,
			},
		}, {
			// 16p power test.
			Name:              "power_16p",
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: meetTest{
				num:    16,
				layout: meetLayoutTiled,
				cam:    true,
				power:  true,
			},
		}, {
			// 16p with jamboard test.
			Name:    "16p_jamboard",
			Timeout: 7 * time.Minute,
			Val: meetTest{
				num:      16,
				layout:   meetLayoutTiled,
				jamboard: true,
				split:    true,
				cam:      true,
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
		timeout        = 10 * time.Second
		defaultDocsURL = "https://docs.new/"
		jamboardURL    = "https://jamboard.google.com"
		notes          = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."
	)

	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: timeout}

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	meet := s.Param().(meetTest)
	if meet.docs && meet.jamboard {
		s.Fatal("Tried to open both Google Docs and Jamboard at the same time")
	}

	// Determines the meet call duration. Use the meet duration specified in
	// test param if there is one. Otherwise, default to 30 seconds for the base
	// calls, 1 min for calls with doc or jamboard, or 3 min for power tests.
	meetTimeout := 30 * time.Second
	if meet.duration != 0 {
		meetTimeout = meet.duration
	} else {
		if meet.docs || meet.jamboard {
			meetTimeout = 60 * time.Second
		}
		if meet.power {
			meetTimeout = 3 * time.Minute
		}
	}
	s.Log("Run meeting for ", meetTimeout)

	// Shorten context to allow for cleanup. Reserve one minute in case of power
	// test.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	creds := s.RequiredVar("ui.MeetCUJ.bond_credentials")
	bc, err := bond.NewClient(ctx, bond.WithCredsJSON([]byte(creds)))
	if err != nil {
		s.Fatal("Failed to create a bond client: ", err)
	}
	defer bc.Close()

	var meetingCode string
	customCode, codeOk := s.Var("meeting_code")
	if codeOk {
		meetingCode = customCode
	} else {
		func() {
			sctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			meetingCode, err = bc.CreateConference(sctx)
			if err != nil {
				s.Fatal("Failed to create a conference room: ", err)
			}
		}()
		s.Log("Created a room with the code ", meetingCode)
	}

	cr := s.FixtValue().(cuj.FixtureData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	if _, ok := s.Var("record"); ok {
		screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
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
		screenRecorder.Start(ctx, tconn)
	}

	tweakPerfValues := func(pv *perf.Values) error { return nil }
	if meet.power {
		s.Log("Preparing for power metrics collection")

		// Setup needs to happen before power.TestMetrics() to disable wifi first
		// so that the thermal sensor for wifi is excluded from the metrics.
		sup, cleanup := setup.New("meet call power")
		sup.Add(setup.PowerTest(ctx, tconn, setup.PowerTestOptions{
			Wifi:       setup.DisableWifiInterfaces,
			Battery:    setup.ForceBatteryDischarge,
			NightLight: setup.DisableNightLight}))
		defer func() {
			if err := cleanup(closeCtx); err != nil {
				s.Error("Cleanup meet power setup failed: ", err)
			}
		}()

		// Power tests need to record power metrics; they are separated from
		// cuj.Recorder's timeline as it is for a different purpose and mixing them
		// might cause a risk of taking too much time of collecting data.
		timeline, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Prefix("Power."))
		if err != nil {
			s.Fatal("Failed to create power metrics: ", err)
		}
		if err = timeline.Start(ctx); err != nil {
			s.Fatal("Failed to start power timeline: ", err)
		}
		if err = timeline.StartRecording(ctx); err != nil {
			s.Fatal("Failed to start recording the power metrics: ", err)
		}
		tweakPerfValues = func(pv *perf.Values) error {
			values, err := timeline.StopRecording(ctx)
			if err != nil {
				return err
			}
			pv.Merge(values)
			return nil
		}
	}

	configs := []cuj.MetricConfig{cuj.NewCustomMetricConfig(
		"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
		"percent", perf.SmallerIsBetter, []int64{5, 10})}
	for _, suffix := range []string{"Capturer", "Encoder", "EncoderQueue", "RateLimiter"} {
		configs = append(configs, cuj.NewCustomMetricConfig(
			"WebRTC.Video.DroppedFrames."+suffix, "percent", perf.SmallerIsBetter,
			[]int64{50, 80}))
	}
	// Jank criteria for input event latencies. The 1st number is the
	// threshold to be marked as jank and the 2nd one is to be marked
	// very jank.
	jankCriteria := []int64{80000, 400000}
	if meet.docs {
		configs = append(configs, cuj.NewCustomMetricConfig(
			"Event.Latency.EndToEnd.KeyPress", "microsecond", perf.SmallerIsBetter,
			jankCriteria))
	} else if meet.jamboard {
		configs = append(configs, cuj.NewCustomMetricConfig(
			"Event.Latency.EndToEnd.Mouse", "microsecond", perf.SmallerIsBetter,
			jankCriteria))
	}

	recorder, err := cuj.NewRecorder(ctx, tconn, configs...)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer func() {
		if err := recorder.Close(closeCtx); err != nil {
			s.Error("Failed to stop recorder: ", err)
		}
	}()

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
	var pc pointer.Context
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

	ui := uiauto.New(tconn)

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	// Find the web view of Meet window.
	webview := nodewith.ClassName("ContentsWebView").Role(role.WebView)
	if err := action.Combine(
		"click and type meeting code",
		// Assume that the meeting code is the only textfield in the webpage.
		ui.LeftClick(nodewith.Role(role.TextField).Ancestor(webview)),
		kw.TypeAction(meetingCode),
		kw.AccelAction("Enter"),
	)(ctx); err != nil {
		s.Fatal("Failed to input the meeting code: ", err)
	}

	bubble := nodewith.ClassName("PermissionPromptBubbleView").First()
	allow := nodewith.Name("Allow").Role(role.Button).Ancestor(bubble)
	// Check and grant permissions.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		needPermission, err := needToGrantPermission(ctx, meetConn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check if it needs to grant permissions"))
		}
		if !needPermission {
			return nil
		}
		if err := pc.Click(allow)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the allow button")
		}
		return errors.New("granting permissions")
	}, &testing.PollOptions{Interval: time.Second, Timeout: 2 * time.Minute}); err != nil {
		s.Fatal("Failed to grant permissions: ", err)
	}

	defer func() {
		// Close the Meet window to finish meeting.
		if err := meetConn.CloseTarget(closeCtx); err != nil {
			s.Error("Failed to close the meeting: ", err)
		}
	}()

	if meet.docs {
		docsURL := defaultDocsURL
		if docsURLOverride, ok := s.Var("ui.MeetCUJ.doc"); ok {
			docsURL = docsURLOverride
		}

		// Create another browser window and open a Google Docs file.
		docsConn, err := cr.NewConn(ctx, docsURL, cdputil.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open the google docs website: ", err)
		}
		defer docsConn.Close()
		s.Log("Creating a Google Docs window")
	} else if meet.jamboard {
		// Create another browser window and open a new Jamboard file.
		jamboardConn, err := cr.NewConn(ctx, jamboardURL, cdputil.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open the jamboard website: ", err)
		}
		defer jamboardConn.Close()
		s.Log("Creating a Jamboard window")
		if err := ui.LeftClick(nodewith.Name("New Jam").Role(role.Button))(ctx); err != nil {
			s.Fatal("Failed to click the new jam button: ", err)
		}
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
				if err := pc.Drag(
					docsWindow.OverviewInfo.Bounds.CenterPoint(),
					// Sleep is needed in tablet mode
					action.Sleep(time.Second),
					pc.DragTo(snapLeftPoint, time.Second),
				)(ctx); err != nil {
					s.Fatal("Failed to drag the Google Docs window to the left: ", err)
				}
			}
			if err := pc.Drag(
				meetWindow.OverviewInfo.Bounds.CenterPoint(),
				action.Sleep(time.Second),
				pc.DragTo(snapRightPoint, time.Second),
			)(ctx); err != nil {
				s.Fatal("Failed to drag the Meet window to the right: ", err)
			}
		} else {
			if docsWindow != nil {
				if err := pc.Drag(docsWindow.OverviewInfo.Bounds.CenterPoint(), pc.DragTo(snapLeftPoint, time.Second))(ctx); err != nil {
					s.Fatal("Failed to drag the Docs window to snap to the left: ", err)
				}
			}
			if err := pc.Drag(meetWindow.OverviewInfo.Bounds.CenterPoint(), pc.DragTo(snapRightPoint, time.Second))(ctx); err != nil {
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

		sctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		// Add 30 seconds to the bot duration to make sure that bots do not leave
		// slightly earlier than the test scenario.
		if !codeOk {
			if _, err := bc.AddBots(sctx, meetingCode, meet.num, meetTimeout+30*time.Second); err != nil {
				return errors.Wrap(err, "failed to create bots")
			}
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
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close all notifications")
		}
		if err := meetConn.Eval(ctx, fmt.Sprintf("hrTelemetryApi.set%sLayout()", string(meet.layout)), nil); err != nil {
			return errors.Wrapf(err, "failed to set %s layout", string(meet.layout))
		}

		if meet.present {
			if !meet.docs && !meet.jamboard {
				return errors.New("need a Google Docs or Jamboard tab to present")
			}
			// Start presenting the tab.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := ui.Exists(nodewith.Name("Chrome Tab").Role(role.ListGrid))(ctx); err == nil {
					return nil
				}
				if err := meetConn.Eval(ctx, "hrTelemetryApi.presentation.presentTab()", nil); err != nil {
					return errors.Wrap(err, "failed to start to present a tab")
				}
				return errors.New("presentation hasn't started yet")
			}, &pollOpts); err != nil {
				return errors.Wrap(err, "failed to start presentation")
			}

			// Select the second tab (Google Docs tab) to present.
			if err := action.Combine(
				"select Google Docs tab",
				pc.Click(nodewith.Name("Chrome Tab").Role(role.ListGrid)),
				// Press down twice to select the second tab, which is Google Docs.
				kw.AccelAction("Down"),
				kw.AccelAction("Down"),
				kw.AccelAction("Enter"),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to select the Google Docs tab")
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

		if meet.docs {
			if err := action.Combine(
				"select Google Docs",
				kw.AccelAction("Alt+Tab"),
				pc.Click(nodewith.Name("Document content").Role(role.TextField)),
				kw.AccelAction("Ctrl+Alt+["),
				kw.AccelAction("Ctrl+A"),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to select Google Docs")
			}
			end := time.Now().Add(meetTimeout)
			// Wait for 5 seconds, type notes for 12.4 seconds then until the time is
			// elapsed (3 times by default). Wait before the first typing to reduce
			// the overlap between typing and joining the meeting.
			for end.Sub(time.Now()).Seconds() > 18 {
				if err := uiauto.Combine(
					"sleep and type",
					action.Sleep(5*time.Second),
					kw.TypeAction(notes),
				)(ctx); err != nil {
					return err
				}
			}
			if err := kw.Accel(ctx, "Alt+Tab"); err != nil {
				return errors.Wrap(err, "failed to hit alt-tab and focus back to Meet tab")
			}
			meetTimeout = end.Sub(time.Now())
		} else if meet.jamboard {
			// Simulate mouse input on jamboard.
			if err := ui.LeftClick(nodewith.Name("Pen").Role(role.ToggleButton))(ctx); err != nil {
				s.Fatal("Failed to click the pen toggle button: ", err)
			}
			contentArea, err := ui.Location(ctx, nodewith.ClassName("jam-content-area").Role(role.GenericContainer))
			if err != nil {
				s.Fatal("Failed to find the location of jamboard content area: ", err)
			}
			centerX, centerY, offsetX, offsetY := contentArea.CenterPoint().X, contentArea.CenterPoint().Y, 10, 10
			end := time.Now().Add(meetTimeout)
			for end.Sub(time.Now()).Seconds() > 42 {
				for i := 1; i <= 10; i++ {
					if err := uiauto.Combine(
						"simulate mouse movement",
						mouse.Move(tconn, coords.NewPoint(centerX-i*offsetX, centerY-i*offsetY), 0),
						mouse.Press(tconn, mouse.LeftButton),
						mouse.Move(tconn, coords.NewPoint(centerX-i*offsetX, centerY+i*offsetY), time.Second),
						mouse.Move(tconn, coords.NewPoint(centerX+i*offsetX, centerY+i*offsetY), time.Second),
						mouse.Move(tconn, coords.NewPoint(centerX+i*offsetX, centerY-i*offsetY), time.Second),
						mouse.Move(tconn, coords.NewPoint(centerX-i*offsetX, centerY-i*offsetY), time.Second),
						mouse.Release(tconn, mouse.LeftButton),
					)(ctx); err != nil {
						s.Fatal("Failed to simulate mouse movement on jamboard: ", err)
					}
				}
			}
			meetTimeout = end.Sub(time.Now())
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

	if err := tweakPerfValues(pv); err != nil {
		s.Fatal("Failed to tweak the perf values: ", err)
	}
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}

// needToGrantPermission checks if we need to grant permission before joining meetings.
// If camera/microphone/notifications permissions are not granted, we need to skip
// the permission bubbles later.
func needToGrantPermission(ctx context.Context, conn *chrome.Conn) (bool, error) {
	perms := []string{"microphone", "camera", "notifications"}
	for _, perm := range perms {
		var state string
		if err := conn.Eval(ctx, fmt.Sprintf(
			`new Promise(function(resolve, reject) {
				navigator.permissions.query({name: '%v'})
				.then((permission) => {
					resolve(permission.state);
				})
				.catch((error) => {
					reject(error);
				});
			 })`, perm), &state); err != nil {
			return true, errors.Errorf("failed to query %v permission", perm)
		}
		if state != "granted" {
			return true, nil
		}
	}
	return false, nil
}
