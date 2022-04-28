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
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/bond"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
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

// meetTest specifies the setting of a Hangouts Meet journey. More info at go/cros-meet-tests.
type meetTest struct {
	num         int                  // Number of bots in the meeting.
	layout      meetLayoutType       // Type of the layout in the meeting.
	present     bool                 // Whether it is presenting the Google Docs or not. It can not be true if docs is false.
	docs        bool                 // Whether it is running with a Google Docs window.
	jamboard    bool                 // Whether it is running with a Jamboard window.
	split       bool                 // Whether it is in split screen mode. It can not be true if docs is false.
	cam         bool                 // Whether the camera is on or not.
	duration    time.Duration        // Duration of the meet call. Must be less than test timeout.
	useLacros   bool                 // Whether to use lacros browser.
	tracing     bool                 // Whether to turn on tracing.
	validation  bool                 // Whether to add extra cpu loads before collecting metrics.
	botsOptions []bond.AddBotsOption // Customizes the meeting participant bots.
}

// videoCodecReport is used to report a video codec to a performance metric so that it is easy to find in places like TPS Dashboard.
type videoCodecReport float64

// Bigger values should represent "better" codecs in some sense, because these are reported with perf.BiggerIsBetter.
// That is silly, of course, but every metric must specify either perf.SmallerIsBetter or perf.BiggerIsBetter.
const (
	vp8 videoCodecReport = 0
	vp9 videoCodecReport = 1
)

const defaultTestTimeout = 25 * time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func:         MeetCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of critical user journey for Google Meet",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		SoftwareDeps: []string{"chrome", "arc", caps.BuiltinOrVividCamera},
		Vars: []string{
			"mute",
			"record",
			"meeting_code",
			"ui.MeetCUJ.doc",
		},
		VarDeps: []string{
			"ui.MeetCUJ.bond_credentials",
		},
		Params: []testing.Param{{
			Name:      "2p",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:    1,
				layout: meetLayoutTiled,
				cam:    true,
			},
			Fixture: "loggedInToCUJUser",
		}, {
			Name:      "lacros_2p",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:       1,
				layout:    meetLayoutTiled,
				cam:       true,
				useLacros: true,
			},
			Fixture:           "loggedInToCUJUserLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name:      "4p",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:    3,
				layout: meetLayoutTiled,
				cam:    true,
			},
			Fixture: "loggedInToCUJUser",
		}, {
			// Small meeting.
			Name:      "4p_present_notes_split",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:     3,
				layout:  meetLayoutTiled,
				present: true,
				docs:    true,
				split:   true,
				cam:     true,
			},
			Fixture: "loggedInToCUJUser",
		}, {
			// Big meeting.
			Name:      "16p",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:    15,
				layout: meetLayoutTiled,
				cam:    true,
			},
			Fixture: "loggedInToCUJUser",
		}, {
			// Even bigger meeting.
			Name:    "49p",
			Timeout: defaultTestTimeout,
			Val: meetTest{
				num:    48,
				layout: meetLayoutTiled,
				cam:    true,
			},
			Fixture: "loggedInToCUJUser",
		}, {
			// Big meeting with tracing.
			Name:      "16p_trace",
			Timeout:   defaultTestTimeout + 10*time.Minute,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:     15,
				layout:  meetLayoutTiled,
				cam:     true,
				tracing: true,
			},
			Fixture: "loggedInToCUJUser",
		}, {
			// Validation test for big meeting.
			Name:    "16p_validation",
			Timeout: defaultTestTimeout + 10*time.Minute,
			Val: meetTest{
				num:        15,
				layout:     meetLayoutTiled,
				cam:        true,
				validation: true,
			},
			Fixture: "loggedInToCUJUser",
		}, {
			// Big meeting with notes.
			Name:    "16p_notes",
			Timeout: defaultTestTimeout,
			Val: meetTest{
				num:    15,
				layout: meetLayoutTiled,
				docs:   true,
				split:  true,
				cam:    true,
			},
			Fixture: "loggedInToCUJUser",
		}, {
			// 16p with jamboard test.
			Name:    "16p_jamboard",
			Timeout: defaultTestTimeout,
			Val: meetTest{
				num:      15,
				layout:   meetLayoutTiled,
				jamboard: true,
				split:    true,
				cam:      true,
			},
			Fixture: "loggedInToCUJUser",
		}, {
			// Lacros 4p
			Name:      "lacros_4p",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:       3,
				layout:    meetLayoutTiled,
				cam:       true,
				useLacros: true,
			},
			Fixture:           "loggedInToCUJUserLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			// 49p with vp8 video codec.
			Name:    "49p_vp8",
			Timeout: defaultTestTimeout,
			Val: meetTest{
				num:         48,
				layout:      meetLayoutTiled,
				cam:         true,
				botsOptions: []bond.AddBotsOption{bond.WithVP9(false, false)},
			},
			Fixture: "loggedInToCUJUser",
		}, {
			// Lacros variation of 16p test
			Name:      "lacros_16p",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:       15,
				layout:    meetLayoutTiled,
				cam:       true,
				useLacros: true,
			},
			Fixture:           "loggedInToCUJUserLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			// Lacros variation of 16p trace test
			Name:    "lacros_16p_trace",
			Timeout: defaultTestTimeout + 10*time.Minute,
			Val: meetTest{
				num:       15,
				layout:    meetLayoutTiled,
				cam:       true,
				useLacros: true,
				tracing:   true,
			},
			Fixture:           "loggedInToCUJUserLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			// Long meeting to catch slow performance degradation.
			Name:    "2p_30m",
			Timeout: defaultTestTimeout + 30*time.Minute,
			Val: meetTest{
				num:      1,
				layout:   meetLayoutTiled,
				cam:      true,
				duration: 30 * time.Minute,
			},
			Fixture: "loggedInToCUJUser",
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
		timeout                 = 10 * time.Second
		defaultDocsURL          = "https://docs.new/"
		jamboardURL             = "https://jamboard.google.com"
		notes                   = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."
		newTabTitle             = "New Tab"
		decodingCodecMetricName = "meetcuj_decoding_codec"
		encodingCodecMetricName = "meetcuj_encoding_codec"
	)

	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: timeout}

	meet := s.Param().(meetTest)
	if meet.docs && meet.jamboard {
		s.Fatal("Tried to open both Google Docs and Jamboard at the same time")
	}

	// Determines the meet call duration. Use the meet duration specified in
	// test param if there is one. Otherwise, default to 10 minutes.
	meetTimeout := 10 * time.Minute
	if meet.duration != 0 {
		meetTimeout = meet.duration
	}
	s.Log("Run meeting for ", meetTimeout)

	// Shorten context to allow for cleanup. Reserve one minute in case of power
	// test.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	var cs ash.ConnSource
	var bTconn *chrome.TestConn
	if meet.useLacros {
		// Launch lacros.
		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		defer l.Close(ctx)
		cs = l

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get lacros TestAPIConn: ", err)
		}
	} else {
		cs = cr
		bTconn = tconn
	}

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

	sctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	// Add 15 minutes to the bot duration, to ensure that the bots stay long enough
	// for the test to detect the video codecs used for encoding and decoding.
	if !codeOk {
		defer func(ctx context.Context) {
			s.Log("Removing all bots from the call")
			if _, _, err := bc.RemoveAllBots(ctx, meetingCode); err != nil {
				s.Fatal("Failed to remove all bots: ", err)
			}
		}(closeCtx)
		addBotsCount := meet.num
		wait := 100 * time.Millisecond
		for i := 0; i < 3; i++ {
			// Exponential backoff. The wait time is 0.1s, 1s and 10s before each retry.
			if err := testing.Sleep(ctx, wait); err != nil {
				s.Errorf("Failed to sleep for %v: %v", wait, err)
			}
			botList, numFailures, err := bc.AddBots(sctx, meetingCode, addBotsCount, meetTimeout+15*time.Minute, meet.botsOptions...)
			if err != nil {
				s.Fatalf("Failed to create %d bots: ", addBotsCount)
			}
			s.Logf("%d bots started, %d bots failed", len(botList), numFailures)
			if numFailures == 0 {
				break
			}
			addBotsCount -= len(botList)
			wait *= 10
		}
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
	// Jank criteria for input event latencies. The 1st number is the
	// threshold to be marked as jank and the 2nd one is to be marked
	// very jank.
	jankCriteria := []int64{80000, 400000}
	if meet.docs {
		configs = append(configs, cuj.NewCustomMetricConfigWithTestConn(
			"Event.Latency.EndToEnd.KeyPress", "microsecond", perf.SmallerIsBetter,
			jankCriteria, bTconn))
	} else if meet.jamboard {
		configs = append(configs, cuj.NewCustomMetricConfigWithTestConn(
			"Event.Latency.EndToEnd.Mouse", "microsecond", perf.SmallerIsBetter,
			jankCriteria, bTconn))
	}

	configs = append(configs, cuj.NewCustomMetricConfig(
		"Cras.FetchDelayMilliSeconds", "millisecond", perf.SmallerIsBetter,
		[]int64{1, 20}))
	configs = append(configs, cuj.NewCustomMetricConfig(
		"Cras.MissedCallbackFrequencyInput", "millisecond", perf.SmallerIsBetter,
		[]int64{1, 20}))
	configs = append(configs, cuj.NewCustomMetricConfig(
		"Cras.MissedCallbackFrequencyOutput", "millisecond", perf.SmallerIsBetter,
		[]int64{1, 20}))

	recorder, err := cuj.NewRecorder(ctx, cr, nil, cuj.RecorderOptions{}, configs...)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	if meet.tracing {
		recorder.EnableTracing(s.OutDir())
	}
	defer func() {
		if err := recorder.Close(closeCtx); err != nil {
			s.Error("Failed to stop recorder: ", err)
		}
	}()

	if meet.validation {
		validationHelper := cuj.NewTPSValidationHelper(closeCtx)
		if err := validationHelper.Stress(); err != nil {
			s.Fatal("Failed to stress: ", err)
		}
		defer func() {
			if err := validationHelper.Release(); err != nil {
				s.Fatal("Failed to release validationHelper: ", err)
			}
		}()
	}

	meetConn, err := cs.NewConn(ctx, "https://meet.google.com/"+meetingCode, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open the hangout meet website: ", err)
	}
	defer meetConn.Close()

	closedMeet := false
	defer func() {
		if closedMeet {
			return
		}
		// Close the Meet window to finish meeting.
		if err := meetConn.CloseTarget(closeCtx); err != nil {
			s.Error("Failed to close the meeting: ", err)
		}
	}()

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Lacros specific setup.
	if meet.useLacros {
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

	// Sets the display zoom factor to minimum, to ensure that all
	// meeting participants' video can be shown simultaneously.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}
	zoomInitial := info.DisplayZoomFactor
	zoomMin := info.AvailableDisplayZoomFactors[0]
	if err := display.SetDisplayProperties(ctx, tconn, info.ID, display.DisplayProperties{DisplayZoomFactor: &zoomMin}); err != nil {
		s.Fatalf("Failed to set display zoom factor to minimum %f: %v", zoomMin, err)
	}
	defer display.SetDisplayProperties(closeCtx, tconn, info.ID, display.DisplayProperties{DisplayZoomFactor: &zoomInitial})

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

	if meet.docs {
		docsURL := defaultDocsURL
		if docsURLOverride, ok := s.Var("ui.MeetCUJ.doc"); ok {
			docsURL = docsURLOverride
		}

		// Create another browser window and open a Google Docs file.
		docsConn, err := cs.NewConn(ctx, docsURL, browser.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open the google docs website: ", err)
		}
		defer docsConn.Close()
		s.Log("Creating a Google Docs window")
	} else if meet.jamboard {
		// Create another browser window and open a new Jamboard file.
		jamboardConn, err := cs.NewConn(ctx, jamboardURL, browser.WithNewWindow())
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
		defer ash.SetOverviewModeAndWait(closeCtx, tconn, false)
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

		if err := meetConn.WaitForExpr(ctx, "hrTelemetryApi.isInMeeting()"); err != nil {
			return errors.Wrap(err, "failed to wait for entering meeting")
		}

		if err := meetConn.Eval(ctx, "hrTelemetryApi.setMicMuted(false)", nil); err != nil {
			return errors.Wrap(err, "failed to turn on mic")
		}
		if !meet.cam {
			if err := meetConn.Eval(ctx, "hrTelemetryApi.setCameraMuted(true)", nil); err != nil {
				return errors.Wrap(err, "failed to turn off camera")
			}
		}

		var participantCount int
		if err := meetConn.Eval(ctx, "hrTelemetryApi.getParticipantCount()", &participantCount); err != nil {
			return errors.Wrap(err, "failed to get participant count")
		}
		if expectedParticipantCount := meet.num + 1; participantCount != expectedParticipantCount {
			return errors.Errorf("got %d participants, expected %d", participantCount, expectedParticipantCount)
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
			return errors.Wrap(err, "failed to start the profiler")
		}
		if prof != nil {
			defer func() {
				if err := prof.End(closeCtx); err != nil {
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

	webrtcInternals, err := cs.NewConn(ctx, "chrome://webrtc-internals")
	if err != nil {
		s.Fatal("Failed to open chrome://webrtc-internals: ", err)
	}
	defer webrtcInternals.Close()

	// Report what video codecs were used for decoding and encoding.
	webRTCReportWaiter := ui.WithTimeout(10 * time.Minute)
	videoStream := nodewith.NameContaining("VideoStream").First()
	if err := webRTCReportWaiter.WaitUntilExists(videoStream)(ctx); err != nil {
		s.Error("Failed to wait for video stream info to appear: ", err)
	}
	if err := reportCodec(ctx, ui, pv, decodingCodecMetricName, "(inbound-rtp, VP8)", "(inbound-rtp, VP9)"); err != nil {
		s.Errorf("Failed to report %s: %v", decodingCodecMetricName, err)
	}
	if err := reportCodec(ctx, ui, pv, encodingCodecMetricName, "(outbound-rtp, VP8)", "(outbound-rtp, VP9)"); err != nil {
		s.Errorf("Failed to report %s: %v", encodingCodecMetricName, err)
	}

	// Report WebRTC metrics for video streams.
	type histInfo struct {
		unit      string
		direction perf.Direction
		outbound  bool
	}
	infoByName := map[string]histInfo{
		"WebRTC.Video.BandwidthLimitedResolutionInPercent":             histInfo{"percent", perf.SmallerIsBetter, true},
		"WebRTC.Video.BandwidthLimitedResolutionsDisabled":             histInfo{"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.CpuLimitedResolutionInPercent":                   histInfo{"percent", perf.SmallerIsBetter, true},
		"WebRTC.Video.DecodedFramesPerSecond":                          histInfo{"fps", perf.BiggerIsBetter, false},
		"WebRTC.Video.DroppedFrames.Capturer":                          histInfo{"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.DroppedFrames.Encoder":                           histInfo{"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.DroppedFrames.EncoderQueue":                      histInfo{"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.DroppedFrames.Ratelimiter":                       histInfo{"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.DroppedFrames.Receiver":                          histInfo{"count", perf.SmallerIsBetter, false},
		"WebRTC.Video.InputFramesPerSecond":                            histInfo{"fps", perf.BiggerIsBetter, true},
		"WebRTC.Video.NumberResolutionDownswitchesPerMinute":           histInfo{"count_per_minute", perf.SmallerIsBetter, false},
		"WebRTC.Video.QualityLimitedResolutionDownscales":              histInfo{"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.QualityLimitedResolutionInPercent":               histInfo{"percent", perf.SmallerIsBetter, true},
		"WebRTC.Video.RenderFramesPerSecond":                           histInfo{"fps", perf.BiggerIsBetter, false},
		"WebRTC.Video.Screenshare.BandwidthLimitedResolutionInPercent": histInfo{"percent", perf.SmallerIsBetter, true},
		"WebRTC.Video.Screenshare.BandwidthLimitedResolutionsDisabled": histInfo{"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.Screenshare.InputFramesPerSecond":                histInfo{"fps", perf.BiggerIsBetter, true},
		"WebRTC.Video.Screenshare.QualityLimitedResolutionDownscales":  histInfo{"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.Screenshare.QualityLimitedResolutionInPercent":   histInfo{"percent", perf.SmallerIsBetter, true},
		"WebRTC.Video.Screenshare.SentFramesPerSecond":                 histInfo{"fps", perf.BiggerIsBetter, true},
		"WebRTC.Video.Screenshare.SentToInputFpsRatioPercent":          histInfo{"percent", perf.BiggerIsBetter, true},
		"WebRTC.Video.SentFramesPerSecond":                             histInfo{"fps", perf.BiggerIsBetter, true},
		"WebRTC.Video.SentToInputFpsRatioPercent":                      histInfo{"percent", perf.BiggerIsBetter, true},
		"WebRTC.Video.TimeInHdPercentage":                              histInfo{"percent", perf.BiggerIsBetter, false},
	}
	var names []string
	for name := range infoByName {
		names = append(names, name)
	}
	if hists, err := metrics.Run(ctx, bTconn, func(ctx context.Context) error {
		// The histograms are recorded when video streams are removed.
		closedMeet = true
		if err := meetConn.CloseTarget(closeCtx); err != nil {
			return errors.Wrap(err, "failed to close the meeting")
		}
		if err := webRTCReportWaiter.WaitUntilGone(videoStream)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for video stream info to disappear")
		}
		return nil
	}, names...); err != nil {
		s.Error("Failed to gather WebRTC metrics for video streams: ", err)
	} else {
		for _, hist := range hists {
			count := hist.TotalCount()
			if count == 0 {
				continue
			}

			info := infoByName[hist.Name]
			var expectedCount int64
			if info.outbound {
				expectedCount = 1
			} else {
				expectedCount = int64(meet.num)
			}
			if count != expectedCount {
				s.Errorf("Unexpected sample count on %s: got %d; expected %d", hist.Name, count, expectedCount)
				continue
			}

			total := float64(hist.Sum)
			if info.outbound {
				pv.Set(perf.Metric{
					Name:      hist.Name,
					Unit:      info.unit,
					Direction: info.direction,
				}, total)
				continue
			}

			var bucketMinima []float64
			var bucketMaxima []float64
			for _, bucket := range hist.Buckets {
				for i := int64(0); i < bucket.Count; i++ {
					bucketMinima = append(bucketMinima, float64(bucket.Min))
					bucketMaxima = append(bucketMaxima, float64(bucket.Max))
				}
			}
			pv.Set(perf.Metric{
				Name:      hist.Name,
				Variant:   "bucket_minima",
				Unit:      info.unit,
				Direction: info.direction,
				Multiple:  true,
			}, bucketMinima...)
			pv.Set(perf.Metric{
				Name:      hist.Name,
				Variant:   "bucket_maxima",
				Unit:      info.unit,
				Direction: info.direction,
				Multiple:  true,
			}, bucketMaxima...)
			pv.Set(perf.Metric{
				Name:      hist.Name,
				Variant:   "total",
				Unit:      info.unit,
				Direction: info.direction,
			}, total)
			pv.Set(perf.Metric{
				Name:      hist.Name,
				Variant:   "mean",
				Unit:      info.unit,
				Direction: info.direction,
			}, total/float64(count))
		}
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}

// reportCodec looks for node names containing given descriptions of a vp8 video stream
// and a vp9 video stream, and reports the detected video codec to a performance metric.
func reportCodec(ctx context.Context, ui *uiauto.Context, pv *perf.Values, metricName, vp8Description, vp9Description string) error {
	foundVP8, err := ui.IsNodeFound(ctx, nodewith.NameContaining(vp8Description).First())
	if err != nil {
		return errors.Wrap(err, "failed to check for vp8 video stream")
	}

	foundVP9, err := ui.IsNodeFound(ctx, nodewith.NameContaining(vp9Description).First())
	if err != nil {
		return errors.Wrap(err, "failed to check for vp9 video stream")
	}

	if !foundVP8 && !foundVP9 {
		return errors.New("found neither a vp8 video stream nor a vp9 video stream")
	}

	if foundVP8 && foundVP9 {
		return errors.New("found both a vp8 video stream and a vp9 video stream")
	}

	var codec videoCodecReport
	if foundVP8 {
		codec = vp8
	} else if foundVP9 {
		codec = vp9
	}

	pv.Set(perf.Metric{
		Name:      metricName,
		Unit:      "unitless",
		Direction: perf.BiggerIsBetter,
	}, float64(codec))

	return nil
}
