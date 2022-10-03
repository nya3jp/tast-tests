// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/bond"
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
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/loginstatus"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/local/webrtcinternals"
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
	num         int                  // Number of bots in the meeting.
	layout      meetLayoutType       // Type of the layout in the meeting.
	present     bool                 // Whether it is presenting the Google Docs/Jamboard window.
	docs        bool                 // Whether it is running with a Google Docs window.
	jamboard    bool                 // Whether it is running with a Jamboard window.
	split       bool                 // Whether it is in split screen mode. It can not be true if docs is false.
	cam         bool                 // Whether the camera is on or not.
	duration    time.Duration        // Duration of the meet call. Must be less than test timeout.
	browserType browser.Type         // Ash Chrome browser or Lacros.
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

const defaultTestTimeout = 30 * time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func:         MeetCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of critical user journey for Google Meet",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		HardwareDeps: hwdep.D(
			hwdep.SkipOnModel("kaisa"),
			hwdep.SkipOnModel("kench"),
		),
		SoftwareDeps: []string{"chrome", "arc"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Vars: []string{
			"mute",
			"record",
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
				num:         1,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserWithWebRTCEventLogging",
		}, {
			Name:      "2p_enterprise",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:         1,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserEnterpriseWithWebRTCEventLogging",
		}, {
			Name:      "lacros_2p",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:         1,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeLacros,
			},
			Fixture:           "loggedInToCUJUserWithWebRTCEventLoggingLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name:      "4p",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:         3,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserWithWebRTCEventLogging",
		}, {
			Name:      "4p_enterprise",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:         3,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserEnterpriseWithWebRTCEventLogging",
		}, {
			// Small meeting.
			Name:      "4p_present_notes_split",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:         3,
				layout:      meetLayoutTiled,
				present:     true,
				docs:        true,
				split:       true,
				cam:         true,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserWithWebRTCEventLogging",
		}, {
			Name:      "4p_present_notes_split_enterprise",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:         3,
				layout:      meetLayoutTiled,
				present:     true,
				docs:        true,
				split:       true,
				cam:         true,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserEnterpriseWithWebRTCEventLogging",
		}, {
			// Big meeting.
			Name:      "16p",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:         15,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserWithWebRTCEventLogging",
		}, {
			Name:      "16p_enterprise",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:         15,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserEnterpriseWithWebRTCEventLogging",
		}, {
			// Even bigger meeting.
			Name:      "49p",
			Timeout:   defaultTestTimeout,
			Val: meetTest{
				num:         48,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserWithWebRTCEventLogging",
		}, {
			Name:      "lacros_49p",
			Timeout:   defaultTestTimeout,
			Val: meetTest{
				num:         48,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeLacros,
			},
			Fixture:           "loggedInToCUJUserWithWebRTCEventLoggingLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			// Big meeting with notes.
			Name:    "16p_notes",
			Timeout: defaultTestTimeout,
			Val: meetTest{
				num:         15,
				layout:      meetLayoutTiled,
				docs:        true,
				split:       true,
				cam:         true,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserWithWebRTCEventLogging",
		}, {
			// 16p with jamboard test.
			Name:    "16p_jamboard",
			Timeout: defaultTestTimeout + 15*time.Minute,
			Val: meetTest{
				num:         15,
				layout:      meetLayoutTiled,
				jamboard:    true,
				split:       true,
				cam:         true,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserWithWebRTCEventLogging",
		}, {
			// Lacros 4p
			Name:      "lacros_4p",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:         3,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeLacros,
			},
			Fixture:           "loggedInToCUJUserWithWebRTCEventLoggingLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			// 49p with vp8 video codec.
			Name:      "49p_vp8",
			Timeout:   defaultTestTimeout,
			Val: meetTest{
				num:         48,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeAsh,
				botsOptions: []bond.AddBotsOption{bond.WithVP9(false, false)},
			},
			Fixture: "loggedInToCUJUserWithWebRTCEventLogging",
		}, {
			// Lacros variation of 16p test
			Name:      "lacros_16p",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:         15,
				layout:      meetLayoutTiled,
				cam:         true,
				browserType: browser.TypeLacros,
			},
			Fixture:           "loggedInToCUJUserWithWebRTCEventLoggingLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			// Long meeting to catch slow performance degradation.
			Name:    "2p_30m",
			Timeout: defaultTestTimeout + 30*time.Minute,
			Val: meetTest{
				num:         1,
				layout:      meetLayoutTiled,
				cam:         true,
				duration:    30 * time.Minute,
				browserType: browser.TypeAsh,
			},
			Fixture: "loggedInToCUJUserWithWebRTCEventLogging",
		}, {
			// Lacros 4p with presenting and notes split
			Name:      "lacros_4p_present_notes_split",
			Timeout:   defaultTestTimeout,
			ExtraAttr: []string{"group:cuj"},
			Val: meetTest{
				num:         3,
				layout:      meetLayoutTiled,
				present:     true,
				docs:        true,
				split:       true,
				cam:         true,
				browserType: browser.TypeLacros,
			},
			Fixture:           "loggedInToCUJUserWithWebRTCEventLoggingLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

// MeetCUJ measures the performance of critical user journeys for Google Meet.
// Journeys for Google Meet are specified by testing parameters.
//
// Pre-preparation:
//   - Open a Meet window.
//   - Create and enter the meeting code.
//   - Open a Google Docs/Jamboard window (if necessary).
//   - Enter split mode (if necessary).
//   - Turn off camera (if necessary).
//
// During recording:
//   - Join the meeting.
//   - Add participants(bots) to the meeting.
//   - Set up the layout.
//   - Max out the number of the maximum tiles (if necessary).
//   - Start to present (if necessary).
//   - Input notes to Google Docs file or draw on Jamboard (if necessary).
//   - Wait for 30 seconds before ending the meeting.
//
// After recording:
//   - Record and save metrics.
func MeetCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout        = 10 * time.Second
		defaultDocsURL = "https://docs.new/"
		jamboardURL    = "https://jamboard.google.com"
		notes          = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."
		newTabTitle    = "New Tab"
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

	// Sets the display zoom factor to minimum, to ensure that all
	// meeting participants' video can be shown simultaneously.
	// The display zoom should be done before the Meet window is
	// opened, to prevent visual oddities on some devices. We also
	// zoom out on the browser after the Meet window is opened,
	// because on some boards the display zoom is not enough to
	// show all of the participants.
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

	var cs ash.ConnSource
	var bTconn *chrome.TestConn
	switch meet.browserType {
	case browser.TypeLacros:
		// Launch lacros.
		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		defer l.Close(closeCtx)
		cs = l

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get lacros TestAPIConn: ", err)
		}
	case browser.TypeAsh:
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
	func() {
		sctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		meetingCode, err = bc.CreateConference(sctx)
		if err != nil {
			s.Fatal("Failed to create a conference room: ", err)
		}
	}()
	s.Log("Created a room with the code ", meetingCode)

	sctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		s.Log("Removing all bots from the call")
		if _, _, err := bc.RemoveAllBots(ctx, meetingCode); err != nil {
			s.Log("Failed to remove all bots: ", err)
		}
	}(closeCtx)
	// Create a bot with spotlight layout to request HD video.
	spotlightBotList, _, err := bc.AddBots(sctx, meetingCode, 1, meetTimeout+30*time.Minute, append(meet.botsOptions, bond.WithLayout("SPOTLIGHT"))...)
	if err != nil {
		s.Fatal("Failed to create bot with spotlight layout: ", err)
	}
	if len(spotlightBotList) != 1 {
		s.Fatalf("Unexpected number of bots with spotlight layout successfully started; got %d, expected 1", len(spotlightBotList))
	}
	// After the spotlight bot, attempt to add meet.num - 1 more bots, for a total of meet.num.
	addBotsCount := meet.num - 1
	if addBotsCount > 0 {
		wait := 100 * time.Millisecond
		for i := 0; i < 3; i++ {
			if err := testing.Sleep(ctx, wait); err != nil {
				s.Errorf("Failed to sleep for %v: %v", wait, err)
			}
			// Exponential backoff. The wait time is 0.1s, 1s and 10s before each retry.
			wait *= 10
			// Add 30 minutes to the bot duration, to ensure that the bots stay long
			// enough for the test to get info from chrome://webrtc-internals.
			botList, numFailures, err := bc.AddBots(sctx, meetingCode, addBotsCount, meetTimeout+30*time.Minute, meet.botsOptions...)
			if err != nil {
				s.Fatalf("Failed to create %d bots: %v", addBotsCount, err)
			}
			s.Logf("%d bots started, %d bots failed", len(botList), numFailures)
			if numFailures == 0 {
				break
			}
			addBotsCount -= len(botList)
		}
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}

	if err := recorder.AddCollectedMetrics(bTconn, meet.browserType,
		cujrecorder.NewCustomMetricConfig("Cras.MissedCallbackFrequencyInput", "millisecond", perf.SmallerIsBetter),
		cujrecorder.NewCustomMetricConfig("Cras.MissedCallbackFrequencyOutput", "millisecond", perf.SmallerIsBetter)); err != nil {
		s.Fatal("Failed to add metrics to recorder: ", err)
	}

	if err := recorder.AddCommonMetrics(tconn, bTconn); err != nil {
		s.Fatal("Failed to add common metrics to recorder: ", err)
	}

	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))

	if _, ok := s.Var("record"); ok {
		if err := recorder.AddScreenRecorder(ctx, tconn, s.TestName()); err != nil {
			s.Fatal("Failed to add screen recorder: ", err)
		}
	}

	defer func() {
		if err := recorder.Close(closeCtx); err != nil {
			s.Error("Failed to stop recorder: ", err)
		}
	}()

	// Open chrome://webrtc-internals now so it will collect data on the meeting's streams.
	webrtcInternals, err := cs.NewConn(ctx, "chrome://webrtc-internals", browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open chrome://webrtc-internals: ", err)
	}
	defer webrtcInternals.Close()

	// Lacros specific setup.
	if meet.browserType == browser.TypeLacros {
		// Close "New Tab" window after creating the chrome://webrtc-internals window.
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

	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, tconn)

	// Expand the Create Dump section of chrome://webrtc-internals. We will not need it
	// until after the meeting, but we can expand the section much faster now while
	// chrome://webrtc-internals does not have much data to show.
	ui := uiauto.New(tconn)
	createDumpSection := nodewith.Name("Create Dump").Role(role.DisclosureTriangle)
	if err := uiauto.Combine("expand",
		ui.DoDefault(createDumpSection.Collapsed()),
		ui.WaitUntilExists(createDumpSection.Expanded()),
	)(ctx); err != nil {
		s.Fatal("Failed to expand Create Dump section of chrome://webrtc-internals: ", err)
	}

	meetConn, err := cs.NewConn(ctx, "https://meet.google.com/"+meetingCode, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open the hangout meet website: ", err)
	}
	defer meetConn.Close()

	// Match window titles `Google Meet` and `meet.google.com`.
	meetRE := regexp.MustCompile(`\bMeet\b|\bmeet\.\b`)
	meetWindow, err := ash.FindOnlyWindow(ctx, tconn, func(w *ash.Window) bool { return meetRE.MatchString(w.Title) })
	if err != nil {
		s.Fatal("Failed to find the Meet window: ", err)
	}

	closedMeet := false
	defer func() {
		if closedMeet {
			return
		}
		// Close the Meet window to finish meeting.
		if err := meetWindow.CloseWindow(closeCtx, tconn); err != nil {
			s.Error("Failed to close the meeting: ", err)
		}
	}()

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

	s.Log("Resetting browser zoom to 100%")
	zoomNode := nodewith.HasClass("ZoomView")
	if err := uiauto.Combine(
		"reset zoom and wait for zoom indicator to be absent",
		kw.AccelAction("Ctrl+0"),
		ui.WaitUntilGone(zoomNode),
	)(ctx); err != nil {
		s.Fatal("Failed to press Ctrl+0 to reset the zoom: ", err)
	}

	// Zoom out on the browser to maximize the number of visible video
	// feeds. This needs to be done before the final layout mode has been set,
	// so that Meet can properly recalculate how many inbound videos should
	// be visible. Pressing Ctrl+Minus 3 times results in the zoom going from
	// 100% -> 90% -> 80% -> 75%.
	if err := inputsimulations.RepeatKeyPress(ctx, kw, "Ctrl+-", 3*time.Second, 3); err != nil {
		s.Fatal("Failed to repeatedly press Ctrl+Minus to zoom out: ", err)
	}

	// Verify that we zoomed correctly.
	zoomInfo, err := ui.Info(ctx, zoomNode)
	if err != nil {
		s.Fatal("Failed to find the current browser zoom: ", err)
	}
	if zoomInfo.Name != "Zoom: 75%" {
		s.Fatalf(`Unexpected zoom value: got %s; want "Zoom: 75%%"`, zoomInfo.Name)
	}
	s.Log("Zoomed browser window to 75%")

	var collaborationRE *regexp.Regexp
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
		collaborationRE = regexp.MustCompile(`\bDocs\b`)
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
		collaborationRE = regexp.MustCompile(`\bJamboard\b`)
	}

	if meet.split {
		if collaborationRE == nil {
			s.Fatal("Need a collaboration window for split view")
		}
		collaborationWindow, err := ash.FindOnlyWindow(ctx, tconn, func(w *ash.Window) bool { return collaborationRE.MatchString(w.Title) })
		if err != nil {
			s.Fatal("Failed to find the collaboration window: ", err)
		}

		if err := ash.SetWindowStateAndWait(ctx, tconn, collaborationWindow.ID, ash.WindowStateLeftSnapped); err != nil {
			s.Fatal("Failed to snap the collaboration window to the left: ", err)
		}
		if err := ash.SetWindowStateAndWait(ctx, tconn, meetWindow.ID, ash.WindowStateRightSnapped); err != nil {
			s.Fatal("Failed to snap the Meet window to the right: ", err)
		}
	} else {
		if err := meetWindow.ActivateWindow(ctx, tconn); err != nil {
			s.Fatal("Failed to activate the Meet window: ", err)
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

		if err := meetConn.Eval(ctx, fmt.Sprintf("hrTelemetryApi.setCameraMuted(%t)", !meet.cam), nil); err != nil {
			return errors.Wrapf(err, "failed to set camera off-status to %t", !meet.cam)
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

		if err := meetConn.Eval(ctx, "hrTelemetryApi.streamQuality.send720p()", nil); err != nil {
			return errors.Wrap(err, "failed to request sending 720p")
		}
		if err := meetConn.Eval(ctx, "hrTelemetryApi.streamQuality.receive720p()", nil); err != nil {
			return errors.Wrap(err, "failed to request receiving 720p")
		}

		// Direct the spotlight bot to pin the test user so
		// that the test user will have to provide HD video.
		login, err := loginstatus.GetLoginStatus(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get login status: ", err)
		}
		if !login.IsLoggedIn {
			s.Fatal("Expect to see a user is logged in in login status")
		}
		if err := bc.ExecuteScript(ctx, fmt.Sprintf("@b%d pin_participant_by_name %q", spotlightBotList[0], *login.DisplayName), meetingCode); err != nil {
			s.Fatal("Failed to direct the spotlight bot to pin the test user: ", err)
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

			presentTabTitle := "Untitled document"
			if meet.jamboard {
				presentTabTitle = "Untitled Jam"
			}

			// Select the tab to present.
			if err := action.Combine(
				"select tab to screenshare",
				pc.Click(nodewith.NameStartingWith(presentTabTitle).HasClass("AXVirtualView")),
				kw.AccelAction("Enter"),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to select the tab to share")
			}
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
		if err := inputsimulations.MoveMouseFor(ctx, tconn, meetTimeout); err != nil {
			return errors.Wrap(err, "failed to simulate mouse movement")
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

	// Report info from chrome://webrtc-internals.
	webRTCUI := ui.WithTimeout(10 * time.Minute)
	videoStream := nodewith.NameContaining("VideoStream").First()
	if path, err := dumpWebRTCInternals(ctx, tconn, webRTCUI, cr.NormalizedUser()); err != nil {
		s.Error("Failed to download dump from chrome://webrtc-internals: ", err)
	} else {
		dump, readErr := os.ReadFile(path)
		if readErr != nil {
			s.Error("Failed to read WebRTC internals dump from Downloads folder: ", readErr)
		}
		if err := os.Remove(path); err != nil {
			s.Error("Failed to remove WebRTC internals dump from Downloads folder: ", err)
		}
		if readErr == nil {
			if err := os.WriteFile(filepath.Join(s.OutDir(), "webrtc-internals.json"), dump, 0644); err != nil {
				s.Error("Failed to write WebRTC internals dump to test results folder: ", err)
			}
			webRTCInternalsPV, err := reportWebRTCInternals(dump, meet.num, meet.present)
			if err != nil {
				s.Error("Failed to report info from WebRTC internals dump to performance metrics: ", err)
			}
			pv.Merge(webRTCInternalsPV)
		}
	}

	// Take a screenshot prior to closing Meet, to facilitate debugging.
	screenshotFile := filepath.Join(s.OutDir(), "meet_screenshot.png")
	if err := screenshot.CaptureChrome(ctx, cr, screenshotFile); err != nil {
		s.Log("Failed to take screenshot: ", err)
	}

	// Reset the browser zoom, because the browser retains the zoom
	// across test variants.
	if err := kw.Accel(ctx, "Ctrl+0"); err != nil {
		s.Log("Failed to reset browser zoom to 100%")
	}

	// Report WebRTC metrics for video streams.
	infoByName := map[string]struct {
		unit      string
		direction perf.Direction
		outbound  bool
	}{
		"WebRTC.Video.BandwidthLimitedResolutionInPercent":             {"percent", perf.SmallerIsBetter, true},
		"WebRTC.Video.BandwidthLimitedResolutionsDisabled":             {"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.CpuLimitedResolutionInPercent":                   {"percent", perf.SmallerIsBetter, true},
		"WebRTC.Video.DecodedFramesPerSecond":                          {"fps", perf.BiggerIsBetter, false},
		"WebRTC.Video.DroppedFrames.Capturer":                          {"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.DroppedFrames.Encoder":                           {"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.DroppedFrames.EncoderQueue":                      {"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.DroppedFrames.Ratelimiter":                       {"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.DroppedFrames.Receiver":                          {"count", perf.SmallerIsBetter, false},
		"WebRTC.Video.InputFramesPerSecond":                            {"fps", perf.BiggerIsBetter, true},
		"WebRTC.Video.NumberResolutionDownswitchesPerMinute":           {"count_per_minute", perf.SmallerIsBetter, false},
		"WebRTC.Video.QualityLimitedResolutionDownscales":              {"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.QualityLimitedResolutionInPercent":               {"percent", perf.SmallerIsBetter, true},
		"WebRTC.Video.RenderFramesPerSecond":                           {"fps", perf.BiggerIsBetter, false},
		"WebRTC.Video.Screenshare.BandwidthLimitedResolutionInPercent": {"percent", perf.SmallerIsBetter, true},
		"WebRTC.Video.Screenshare.BandwidthLimitedResolutionsDisabled": {"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.Screenshare.InputFramesPerSecond":                {"fps", perf.BiggerIsBetter, true},
		"WebRTC.Video.Screenshare.QualityLimitedResolutionDownscales":  {"count", perf.SmallerIsBetter, true},
		"WebRTC.Video.Screenshare.QualityLimitedResolutionInPercent":   {"percent", perf.SmallerIsBetter, true},
		"WebRTC.Video.Screenshare.SentFramesPerSecond":                 {"fps", perf.BiggerIsBetter, true},
		"WebRTC.Video.Screenshare.SentToInputFpsRatioPercent":          {"percent", perf.BiggerIsBetter, true},
		"WebRTC.Video.SentFramesPerSecond":                             {"fps", perf.BiggerIsBetter, true},
		"WebRTC.Video.SentToInputFpsRatioPercent":                      {"percent", perf.BiggerIsBetter, true},
		"WebRTC.Video.TimeInHdPercentage":                              {"percent", perf.BiggerIsBetter, false},
	}
	var names []string
	for name := range infoByName {
		names = append(names, name)
	}
	if hists, err := metrics.Run(ctx, bTconn, func(ctx context.Context) error {
		// The histograms are recorded when video streams are removed.
		closedMeet = true
		if err := meetWindow.CloseWindow(closeCtx, tconn); err != nil {
			return errors.Wrap(err, "failed to close the meeting")
		}
		if err := webRTCUI.WaitUntilGone(videoStream)(ctx); err != nil {
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

// dumpWebRTCInternals downloads a dump from chrome://webrtc-internals and
// returns the file path. This function assumes that chrome://webrtc-internals
// is already shown, with the Create Dump section expanded.
func dumpWebRTCInternals(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, username string) (string, error) {
	downloadsPath, err := cryptohome.DownloadsPath(ctx, username)
	if err != nil {
		return "", errors.Wrap(err, "failed to get Downloads path")
	}

	button := nodewith.Name("Download the PeerConnection updates and stats data").Role(role.Button)
	if err := uiauto.Combine("invoke the button for the dump download",
		ui.WaitUntilExists(button),
		ui.DoDefault(button),
	)(ctx); err != nil {
		return "", err
	}

	notification, err := ash.WaitForNotification(ctx, tconn, 10*time.Minute, ash.WaitTitle("Download complete"))
	if err != nil {
		return "", errors.Wrap(err, "failed to wait for download notification")
	}

	return filepath.Join(downloadsPath, notification.Message), nil
}

// reportWebRTCInternals reports info from a WebRTC internals dump to performance metrics.
func reportWebRTCInternals(dump []byte, numBots int, present bool) (*perf.Values, error) {
	var webRTC webrtcinternals.Dump
	if err := json.Unmarshal(dump, &webRTC); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal WebRTC internals dump")
	}

	expectedConns := 1
	expectedScreenshareConns := 0
	if present {
		expectedConns = 2
		expectedScreenshareConns = 1
	}

	if numConns := len(webRTC.PeerConnections); numConns != expectedConns {
		return nil, errors.Errorf("unexpected number of peer connections: got %d; want %d", numConns, expectedConns)
	}

	numScreenshareConns := 0
	pv := perf.NewValues()
	for connID, peerConn := range webRTC.PeerConnections {
		byType := peerConn.Stats.BuildIndex()
		inTotalCount, inScreenshareCount, err := reportVideoStreams(pv, byType["inbound-rtp"], "framesReceived", ".Inbound", "bot%02d")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to report inbound-rtp video streams in peer connection %v", connID)
		}
		outTotalCount, outScreenshareCount, err := reportVideoStreams(pv, byType["outbound-rtp"], "framesSent", ".Outbound", "stream%d")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to report outbound-rtp video streams in peer connection %v", connID)
		}

		if inScreenshareCount != 0 {
			return nil, errors.Errorf("unexpected number of inbound-rtp screenshare video streams in peer connection %v; got %d, want 0", connID, inScreenshareCount)
		}
		if outTotalCount == 0 {
			return nil, errors.Errorf("found no outbound-rtp video streams in peer connection %v", connID)
		}
		expectedInTotalCount := 0
		switch outScreenshareCount {
		case 0:
			expectedInTotalCount = numBots
		case outTotalCount:
			numScreenshareConns++
		default:
			return nil, errors.Errorf("found %d screenshare(s) among %d outbound-rtp video streams in peer connection %v, expected all or none", outScreenshareCount, outTotalCount, connID)
		}
		if inTotalCount != expectedInTotalCount {
			return nil, errors.Errorf("unexpected number of inbound-rtp video streams in peer connection %v; got %d, want %d", connID, inTotalCount, expectedInTotalCount)
		}
	}

	if numScreenshareConns != expectedScreenshareConns {
		return nil, errors.Errorf("unexpected number of screenshare peer connections; got %d, want %d", numScreenshareConns, expectedScreenshareConns)
	}

	return pv, nil
}

// reportVideoStreams reports info from a webrtcinternals.StatsIndexByStatsID to performance
// metrics. Returns the number of active video streams, and how many of them are screenshares.
func reportVideoStreams(pv *perf.Values, byID webrtcinternals.StatsIndexByStatsID, framesTransmittedAttribute, directionSuffix, variantFormat string) (int, int, error) {
	totalCount := 0
	screenshareCount := 0
	for id, byAttribute := range byID {
		kindTimeline, ok := byAttribute["kind"]
		if !ok {
			return 0, 0, errors.Errorf("no kind attribute for %q", id)
		}
		kind, err := kindTimeline.Collapse()
		if err != nil {
			return 0, 0, errors.Errorf("failed to collapse timeline of kind attribute for %q", id)
		}
		if kind != "video" {
			continue
		}

		framesTransmittedTimeline, ok := byAttribute[framesTransmittedAttribute]
		if !ok {
			return 0, 0, errors.Errorf("no %s attribute for %q", framesTransmittedAttribute, id)
		}
		if len(framesTransmittedTimeline) == 0 {
			return 0, 0, errors.Errorf("no values for %s attribute for %q", framesTransmittedAttribute, id)
		}
		if framesTransmittedTimeline[len(framesTransmittedTimeline)-1] == 0 {
			continue
		}

		screenShareSuffix := ""
		if contentTypeTimeline, ok := byAttribute["contentType"]; ok {
			contentType, err := contentTypeTimeline.Collapse()
			if err != nil {
				return 0, 0, errors.Errorf("failed to collapse timeline of contentType attribute for %q", id)
			}
			if contentType == "screenshare" {
				screenShareSuffix = ".Screenshare"
				screenshareCount++
			}
		}

		for _, config := range []struct {
			attribute       string
			reporter        func(interface{}) (float64, error)
			attributeSuffix string
			unit            string
		}{
			{"frameWidth", reportFloat64, ".frameWidth", "px"},
			{"frameHeight", reportFloat64, ".frameHeight", "px"},
			{"framesPerSecond", reportFloat64, ".framesPerSecond", "fps"},
			{"[codec]", reportVideoCodec, ".codec", "unitless"},
		} {
			timeline, ok := byAttribute[config.attribute]
			if !ok {
				continue
			}

			var report []float64
			for _, value := range timeline {
				metric, err := config.reporter(value)
				if err != nil {
					return 0, 0, errors.Wrapf(err, "failed to represent %s attribute for %q as performance metric", config.attribute, id)
				}
				report = append(report, metric)
			}

			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("WebRTCInternals.Video%s%s%s", screenShareSuffix, directionSuffix, config.attributeSuffix),
				Variant:   fmt.Sprintf(variantFormat, totalCount),
				Unit:      config.unit,
				Direction: perf.BiggerIsBetter,
				Multiple:  true,
			}, report...)
		}
		totalCount++
	}
	return totalCount, screenshareCount, nil
}

// reportFloat64 simply typecasts from interface{} to float64.
func reportFloat64(value interface{}) (float64, error) {
	report, ok := value.(float64)
	if !ok {
		return 0, errors.Errorf("%v is not of type float64", value)
	}
	return report, nil
}

// reportVideoCodec parses a video codec description from a WebRTC internals dump, and
// represents the video codec as float64 so it can be reported to a performance metric.
func reportVideoCodec(value interface{}) (float64, error) {
	description, ok := value.(string)
	if !ok {
		return 0, errors.Errorf("%v is not of type string", value)
	}

	if strings.HasPrefix(description, "VP8") {
		return float64(vp8), nil
	}
	if strings.HasPrefix(description, "VP9") {
		return float64(vp9), nil
	}
	return 0, errors.Errorf("unrecognized video stream codec: %q", description)
}
