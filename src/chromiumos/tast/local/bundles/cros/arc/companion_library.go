// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CompanionLibrary,
		Desc:         "Test all ARC++ companion library",
		Contacts:     []string{"sstan@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcCompanionLibDemo.apk"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

const pkg = "org.chromium.arc.companionlibdemo"

type companionLibMessage struct {
	MessageID int    `json:"mid"`
	Type      string `json:"type"`
	API       string `json:"api"`
	LogMsg    *struct {
		Msg string `json:"msg"`
	} `json:"LogMsg"`
	CaptionHeightMsg *struct {
		CaptionHeight int `json:"caption_height"`
	} `json:"CaptionHeightMsg"`
	DeviceModeMsg *struct {
		DeviceMode string `json:"device_mode"`
	} `json:"DeviceModeMsg"`
	WorkspaceInsetMsg *struct {
		InsetBound string `json:"inset_bound"`
	} `json:"WorkspaceInsetMsg"`
}

func CompanionLibrary(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcCompanionLibDemo.apk"

		mainActivity = ".MainActivity"
	)

	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	// Force Chrome to be in clamshell mode, where windows are resizable.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to disable tablet mode: ", err)
	}

	a := s.PreValue().(arc.PreData).ARC
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, mainActivity)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to get device: ", err)
	}
	defer d.Close()

	if err := act.WaitForResumed(ctx, time.Second); err != nil {
		s.Fatal("Failed to wait for activity to resume: ", err)
	}

	type testFunc func(context.Context, *chrome.Conn, *arc.Activity, *ui.Device, *testing.State) error
	for _, test := range []struct {
		name string
		fn   testFunc
	}{
		{"Window State", testWindowState},
		{"Get Workspace Insets", testWorkspaceInsets},
		{"Caption Button", testCaptionButton},
		{"Get Device Mode", testDeviceMode},
		{"Get Caption Height", testCaptionHeight},
	} {
		s.Logf("Running %q", test.name)
		if err := act.Start(ctx); err != nil {
			s.Fatal("Failed to start context: ", err)
		}
		if err := act.WaitForResumed(ctx, time.Second); err != nil {
			s.Fatal("Failed to wait for activity to resuyme: ", err)
		}
		if err := test.fn(ctx, tconn, act, d, s); err != nil {
			s.Errorf("%s test failed: %v", test.name, err)
		}
		if err := act.Stop(ctx); err != nil {
			s.Fatal("Failed to stop context: ", err)
		}
	}

}

// testCaptionHeight verifies that the caption height length getting from ChromeOS companion library is correct.
func testCaptionHeight(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device, s *testing.State) error {
	const getCaptionHeightButtonID = pkg + ":id/get_caption_height"

	dispMode, err := ash.InternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}

	// Read JSON format window caption height infomation.
	baseMessage, err := getLastJSONMessage(ctx, d)
	if err != nil {
		return errors.Wrap(err, "failed to get base json message")
	}
	if err := d.Object(ui.ID(getCaptionHeightButtonID)).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Get Caption Height button")
	}
	var msg *companionLibMessage
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		msg, err = getLastJSONMessage(ctx, d)
		if err != nil {
			return testing.PollBreak(err)
		}
		// Waiting for new message coming
		if baseMessage.MessageID == msg.MessageID {
			return errors.New("still waiting the new json message")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to get new message of caption height")
	}
	if msg.CaptionHeightMsg == nil {
		return errors.Errorf("unexpected JSON message format: no CaptionHeightMsg; got %v", msg)
	}

	appWindow, err := getArcAppWindowInfo(ctx, tconn, pkg)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window")
	}

	actualHeight := int(math.Round(float64(appWindow.CaptionHeight) * dispMode.DeviceScaleFactor))
	if actualHeight != msg.CaptionHeightMsg.CaptionHeight {
		return errors.Errorf("wrong caption height: got %v, want %v", msg.CaptionHeightMsg.CaptionHeight, actualHeight)
	}
	return nil

}

func testWorkspaceInsets(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device, s *testing.State) error {
	const getWorkspaceInsetsButtonID = pkg + ":id/get_workspace_insets"

	parseRectString := func(rectShortString string, mode *display.DisplayMode) (ash.Rect, error) {
		// The rectangle short string generated by android /frameworks/base/graphics/java/android/graphics/Rect.java
		// Parse it to rectangle format with native pixel size.
		var left, top, right, bottom int
		if n, err := fmt.Sscanf(rectShortString, "[%d,%d][%d,%d]", &left, &top, &right, &bottom); err != nil {
			return ash.Rect{}, errors.Wrap(err, "Error on parse Rect text")
		} else if n != 4 {
			return ash.Rect{}, errors.Errorf("The format of Rect text is not valid: %q", rectShortString)
		}
		return ash.Rect{
			Left:   left,
			Top:    top,
			Width:  mode.WidthInNativePixels - left - right,
			Height: mode.HeightInNativePixels - top - bottom,
		}, nil
	}

	dispMode, err := ash.InternalDisplayMode(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display mode: ", err)
	}
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	for _, test := range []struct {
		shelfAlignment ash.ShelfAlignment
		shelfBehavior  ash.ShelfBehavior
	}{
		{ash.ShelfAlignmentLeft, ash.ShelfBehaviorAlwaysAutoHide},
		{ash.ShelfAlignmentLeft, ash.ShelfBehaviorNeverAutoHide},
		{ash.ShelfAlignmentRight, ash.ShelfBehaviorAlwaysAutoHide},
		{ash.ShelfAlignmentRight, ash.ShelfBehaviorNeverAutoHide},
		{ash.ShelfAlignmentBottom, ash.ShelfBehaviorAlwaysAutoHide},
		{ash.ShelfAlignmentBottom, ash.ShelfBehaviorNeverAutoHide},
	} {
		if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, test.shelfBehavior); err != nil {
			s.Fatalf("Failed to set shelf behavior to %v: %v", test.shelfBehavior, err)
		}
		if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, test.shelfAlignment); err != nil {
			s.Fatalf("Failed to set shelf alignment to %v: %v", test.shelfAlignment, err)
		}
		var expectedShelfRect arc.Rect
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Confirm the shelf attribute has changed.
			if actualShelfAlignment, err := ash.GetShelfAlignment(ctx, tconn, dispInfo.ID); err != nil {
				return errors.Wrap(err, "failed to get shelf alignment")
			} else if actualShelfAlignment != test.shelfAlignment {
				return errors.Errorf("shelf alignment has not changed yet: got %v, want %v", actualShelfAlignment, test.shelfAlignment)
			}
			dispInfo, err := display.GetInternalInfo(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get internal display info: ", err)
			}
			// The unit of WorkArea is DP.
			expectedShelfRect = arc.Rect{
				Left:   dispInfo.WorkArea.Left,
				Top:    dispInfo.WorkArea.Top,
				Width:  dispInfo.WorkArea.Width,
				Height: dispInfo.WorkArea.Height,
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Fatal("Could not change the system shelf alignment: ", err)
		}

		// Read JSON format window insets size from CompanionLib Demo.
		baseMessage, err := getLastJSONMessage(ctx, d)
		if err != nil {
			return errors.Wrap(err, "failed to get basement json message")
		}
		if err := d.Object(ui.ID(getWorkspaceInsetsButtonID)).Click(ctx); err != nil {
			s.Fatal("Failed to click Get Workspace Insets button: ", err)
		}
		var msg *companionLibMessage
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var err error
			msg, err = getLastJSONMessage(ctx, d)
			if err != nil {
				return testing.PollBreak(err)
			}
			// Waiting for new message coming
			if baseMessage.MessageID == msg.MessageID {
				return errors.New("still waiting the new json message")
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to get new message of device mode")
		}
		if msg.WorkspaceInsetMsg == nil {
			return errors.Errorf("unexpected JSON message format: no WorkspaceInsetMsg; got %v", msg)
		}
		parsedShelfRect, err := parseRectString(msg.WorkspaceInsetMsg.InsetBound, dispMode)
		if err != nil {
			s.Fatal("Failed to parse message: ", err)
		}
		// Convert two rectangle to same unit.
		expectedShelfRectPX := ash.ConvertBoundsFromDpToPx(ash.Rect(expectedShelfRect), dispMode.DeviceScaleFactor)

		if expectedShelfRectPX != parsedShelfRect {
			s.Fatalf("Workspace Inset is not expected: got %v, want %v", parsedShelfRect, expectedShelfRectPX)
		}
	}
	return nil
}

func testCaptionButton(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device, s *testing.State) error {
	const (
		setCaptionButtonID                      = pkg + ":id/set_caption_buttons_visibility"
		checkCaptionButtonMinimizeBox           = pkg + ":id/caption_button_minimize"
		checkCaptionButtonMaximizeAndRestoreBox = pkg + ":id/caption_button_maximize_and_restore"
		checkCaptionButtonLegacyMenuBox         = pkg + ":id/caption_button_legacy_menu"
		checkCaptionButtonGoBackBox             = pkg + ":id/caption_button_go_back"
		checkCaptionButtonCloseBox              = pkg + ":id/caption_button_close"
	)

	resetCaptionCheckboxes := func() error {
		for _, checkboxID := range []string{
			checkCaptionButtonMinimizeBox,
			checkCaptionButtonMaximizeAndRestoreBox,
			checkCaptionButtonLegacyMenuBox,
			checkCaptionButtonGoBackBox,
			checkCaptionButtonCloseBox,
		} {
			checked, err := d.Object(ui.ID(checkboxID)).IsChecked(ctx)
			if err != nil {
				return errors.Wrap(err, "could not get the checkbox statement")
			}
			if checked != false {
				s.Logf("Clean %s checkbox statements", checkboxID)
				if err := d.Object(ui.ID(checkboxID)).Click(ctx); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, test := range []struct {
		buttonCheckboxID        string
		buttonVisibleStatusMask ash.CaptionButtonStatus
	}{
		{checkCaptionButtonMinimizeBox, ash.CaptionButtonMinimize},
		{checkCaptionButtonMaximizeAndRestoreBox, ash.CaptionButtonMaximizeAndRestore},
		{checkCaptionButtonLegacyMenuBox, ash.CaptionButtonMenu},
		{checkCaptionButtonGoBackBox, ash.CaptionButtonBack},
		{checkCaptionButtonCloseBox, ash.CaptionButtonClose},
	} {
		s.Logf("Test hiding %v caption button", test.buttonCheckboxID)

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := d.Object(ui.ID(setCaptionButtonID)).Click(ctx); err != nil {
				return errors.Wrap(err, "could not click the setCaptionButton")
			}
			if err := resetCaptionCheckboxes(); err != nil {
				return errors.Wrap(err, "could not clean the button checkboxes setting")
			}
			if err := d.Object(ui.ID(test.buttonCheckboxID)).Click(ctx); err != nil {
				return errors.Wrap(err, "could not check the checkbox")
			}
			if err := d.Object(ui.Text("OK")).Click(ctx); err != nil {
				return errors.Wrap(err, "could not click the OK button")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.New("Error while changing hidden caption button")
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			window, err := getArcAppWindowInfo(ctx, tconn, pkg)
			if err != nil {
				return errors.Wrap(err, "error while get ARC window")
			}
			if window.CaptionButtonVisibleStatus&int(test.buttonVisibleStatusMask) != 0 {
				return errors.Errorf("Caption Button %v still visible", test.buttonCheckboxID)
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to waiting caption button changed")
		}

	}
	return nil
}

func testDeviceMode(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device, s *testing.State) error {
	const getDeviceModeButtonID = pkg + ":id/get_device_mode_button"

	for _, test := range []struct {
		// isTabletMode represents current mode of system which is Tablet mode or clamshell mode.
		isTabletMode bool
		// modeStatus represents the expection of device mode string getting from companion library.
		modeStatus string
	}{
		{isTabletMode: true, modeStatus: "TABLET"},
		{isTabletMode: false, modeStatus: "CLAMSHELL"},
	} {
		// Force Chrome to be in specific system mode.
		if err := ash.SetTabletModeEnabled(ctx, tconn, test.isTabletMode); err != nil {
			s.Fatal("Failed to set the system mode: ", err)
		}

		// Read JSON format window caption height infomation.
		baseMessage, err := getLastJSONMessage(ctx, d)
		if err != nil {
			return errors.Wrap(err, "failed to get basement json message")
		}
		if err := d.Object(ui.ID(getDeviceModeButtonID)).Click(ctx); err != nil {
			s.Fatal("Could not click the getDeviceMode button: ", err)
		}
		var msg *companionLibMessage
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var err error
			msg, err = getLastJSONMessage(ctx, d)
			if err != nil {
				return testing.PollBreak(err)
			}
			// Waiting for new message coming
			if baseMessage.MessageID == msg.MessageID {
				return errors.New("still waiting the new json message")
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to get new message of device mode")
		}
		if msg.DeviceModeMsg == nil {
			return errors.Errorf("unexpected JSON message format: no DeviceModeMsg; got %v", msg)
		}
		if msg.DeviceModeMsg.DeviceMode != test.modeStatus {
			return errors.Errorf("unexpected getDeviceMode result: got %s; want %s", msg.DeviceModeMsg.DeviceMode, test.modeStatus)
		}
	}
	return nil
}

func testWindowState(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device, s *testing.State) error {
	const (
		setWindowStateButtonID = pkg + ":id/set_task_window_state_button"
		getWindowStateButtonID = pkg + ":id/get_task_window_state_button"
	)
	// TODO(sstan): Add testcase of "Always on top" setting
	for _, test := range []struct {
		windowStateStr string
		windowStateExp arc.WindowState
		isAppManaged   bool
	}{
		{windowStateStr: "Minimize", windowStateExp: arc.WindowStateMinimized, isAppManaged: false},
		{windowStateStr: "Maximize", windowStateExp: arc.WindowStateMaximized, isAppManaged: false},
		{windowStateStr: "Normal", windowStateExp: arc.WindowStateNormal, isAppManaged: false},
	} {
		s.Logf("Testing windowState=%v, appManaged=%t", test.windowStateStr, test.isAppManaged)
		if err := act.Start(ctx); err != nil {
			s.Fatal("Failed to start context: ", err)
		}
		if err := act.WaitForResumed(ctx, time.Second); err != nil {
			s.Fatal("Failed to wait for Resumed: ", err)
		}
		if err := d.Object(ui.ID(setWindowStateButtonID)).Click(ctx); err != nil {
			s.Fatal("Failed to click Set Task Window State button: ", err)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if isClickable, err := d.Object(ui.Text(test.windowStateStr)).IsClickable(ctx); err != nil {
				return errors.Wrap(err, "failed check the radio clickable")
			} else if isClickable {
				// If isClickable = false, it will do nothing because the test application logic will automatically check the current window state radio. It can't be clicked if the state radio has been clicked.
				if err := d.Object(ui.Text(test.windowStateStr)).Click(ctx); err != nil {
					s.Fatalf("Failed to click %v radio: %v", test.windowStateStr, err)
				}
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Failed to waiting click radio: ", err)
		}

		if err := d.Object(ui.Text("OK")).Click(ctx); err != nil {
			s.Fatal("Failed to click OK button: ", err)
		}
		err := testing.Poll(ctx, func(ctx context.Context) error {
			actualWindowState, err := act.GetWindowState(ctx)
			if err != nil {
				return errors.Wrap(err, "could not get window state")
			}
			if actualWindowState != test.windowStateExp {
				return errors.Errorf("unexpected window state: got %v; want %v", actualWindowState, test.windowStateExp)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second})
		if err != nil {
			s.Fatal("Error while waiting window state setting up: ", err)
		}
		if err := act.Stop(ctx); err != nil {
			s.Fatal("Failed to stop context: ", err)
		}
	}
	return nil
}

// getArcAppWindowInfo returns corresponding arc window infomation.
func getArcAppWindowInfo(ctx context.Context, tconn *chrome.Conn, pkgName string) (*ash.Window, error) {
	var appWindow *ash.Window
	if windows, err := ash.GetAllWindows(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "get all windows info")
	} else if len(windows) > 0 {
		for _, w := range windows {
			if w.WindowType == ash.WindowTypeArc && w.ARCPackageName == pkgName {
				appWindow = w
			}
		}
	}
	if appWindow == nil {
		return nil, errors.New("can not find corresponding ARC window")
	}
	return appWindow, nil
}

// getTextViewContent returns all text in status textview.
func getTextViewContent(ctx context.Context, d *ui.Device) ([]string, error) {
	const statusTextViewID = pkg + ":id/status_text_view"
	text, err := d.Object(ui.ID(statusTextViewID)).GetText(ctx)
	if err != nil {
		// It not always success when get object, poll is necessary.
		return nil, errors.Wrap(err, "StatusTextView not ready yet")
	}
	return strings.Split(text, "\n"), nil
}

// getJSONTextViewContent returns all text in JSON textview.
func getJSONTextViewContent(ctx context.Context, d *ui.Device) ([]string, error) {
	const JSONTextViewID = pkg + ":id/status_jsontext_view"
	text, err := d.Object(ui.ID(JSONTextViewID)).GetText(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "JSONStatusTextView not ready yet")
	}
	return strings.Split(text, "\n"), nil
}

// getLastJSONMessage return last JSON format output message of ChromeOS Companion Library Demo
func getLastJSONMessage(ctx context.Context, d *ui.Device) (*companionLibMessage, error) {
	var lines []string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		lines, err = getJSONTextViewContent(ctx, d)
		// Using poll here to avoid get text failure because UI compontent isn't stable.
		if err != nil {
			return errors.Wrap(err, "failed to get JSON message text")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to get a new line in status text view")
	}
	var msg companionLibMessage
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &msg); err != nil {
		return nil, errors.Wrap(err, "parse JSON format message failure")
	}
	return &msg, nil
}
