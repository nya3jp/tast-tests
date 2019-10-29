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

	var originalLength int
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// UI component not always stable, especially in first time, so Poll here.
		lines, err := getJSONTextViewContent(ctx, d)
		if err != nil {
			return err
		}
		originalLength = len(lines)
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "could not get JSON message in textview")
	}

	// Read JSON format window caption height infomation.
	if err := d.Object(ui.ID(getCaptionHeightButtonID)).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Get Caption Height button")
	}
	var lines []string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		lines, err = getJSONTextViewContent(ctx, d)
		if err != nil {
			return err
		}
		if len(lines) == originalLength {
			return errors.New("textview still waiting update")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "error get new line in status text view")
	}

	var msg companionLibMessage
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &msg); err != nil {
		return errors.Wrap(err, "parse JSON format message failure")
	} else if msg.CaptionHeightMsg == nil {
		return errors.New("unexpected JSON message format: no CaptionHeightMsg")
	}

	var appWindow *ash.Window
	if windows, err := ash.GetAllWindows(ctx, tconn); err != nil {
		return errors.Wrap(err, "get all windows info")
	} else if len(windows) > 0 {
		for _, w := range windows {
			if w.WindowType == ash.WindowTypeArc && w.ARCPackageName == pkg {
				appWindow = w
			}
		}
	}

	if appWindow == nil {
		return errors.New("can not find corresponding ARC window")
	}
	actualHeight := int(math.Round(float64(appWindow.CaptionHeight) * dispMode.DeviceScaleFactor))
	if actualHeight != msg.CaptionHeightMsg.CaptionHeight {
		return errors.Errorf("wrong caption height: got %v, want %v", msg.CaptionHeightMsg.CaptionHeight, actualHeight)
	}
	return nil

}

func testWorkspaceInsets(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device, s *testing.State) error {
	const getWorkspaceInsetsButtonID = pkg + ":id/get_workspace_insets"

	parseRectText := func(msg string, mode *display.DisplayMode) (ash.Rect, error) {
		// The app message text format is that `Rect(left, top, right, bottom)`.
		// Parse it to rectangle format with native pixel size.
		var left, top, right, bottom int
		if n, err := fmt.Sscanf(msg, "Rect(%d,%d - %d,%d)", &left, &top, &right, &bottom); err != nil {
			return ash.Rect{}, errors.Wrap(err, "Error on parse Rect text")
		} else if n != 4 {
			return ash.Rect{}, errors.Errorf("The format of Rect text is not valid: %q", msg)
		}
		return ash.Rect{
			Left:   left,
			Top:    top,
			Width:  mode.WidthInNativePixels - left - right,
			Height: mode.HeightInNativePixels - top - bottom,
		}, nil
	}

	parseWorkspaceMessage := func(msg string, mode *display.DisplayMode) (ash.Rect, error) {
		const messagePrefix = "Workspace Insets: "
		if !strings.HasPrefix(msg, messagePrefix) {
			return ash.Rect{}, errors.Errorf("invalid message format: got %q; want message with prefix %q", msg, messagePrefix)
		}
		return parseRectText(strings.TrimPrefix(msg, messagePrefix), mode)
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

		var originalLength int
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// UI component not always stable, especially in first time, so Poll here.
			lines, err := getTextViewContent(ctx, d)
			if err != nil {
				return err
			}
			originalLength = len(lines)
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Fatal("Could not get text in textview: ", err)
		}

		// Read window insets size from CompanionLib Demo.
		if err := d.Object(ui.ID(getWorkspaceInsetsButtonID)).Click(ctx); err != nil {
			s.Fatal("Failed to click Get Workspace Insets button: ", err)
		}
		var lines []string
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var err error
			lines, err = getTextViewContent(ctx, d)
			if err != nil {
				return err
			}
			if len(lines) == originalLength {
				return errors.New("textview still waiting update")
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Fatal("Error get new line in status text view: ", err)
		}
		parsedShelfRect, err := parseWorkspaceMessage(lines[len(lines)-1], dispMode)
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

func testDeviceMode(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device, s *testing.State) error {
	const getDeviceModeButtonID = pkg + ":id/get_device_mode_button"

	getDeviceModeString := func() string {
		// Each output in ArcCompanionLib Demo will gengerate a new line in the TextView.
		var originalLength int
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// UI component not always stable, especially in first time, so Poll here.
			lines, err := getTextViewContent(ctx, d)
			if err != nil {
				return err
			}
			originalLength = len(lines)
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Could not get text in textview: ", err)
		}

		if err := d.Object(ui.ID(getDeviceModeButtonID)).Click(ctx); err != nil {
			s.Fatal("Could not click the getDeviceMode button: ", err)
		}

		var lines []string
		err := testing.Poll(ctx, func(ctx context.Context) error {
			var err error
			lines, err = getTextViewContent(ctx, d)
			if err != nil {
				return err
			}
			if len(lines) == originalLength {
				return errors.New("textview still waiting update")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second})
		if err != nil {
			s.Fatal("Error get new line in status text view: ", err)
		}

		// Get the latest message from the last line.
		lastMessage := lines[len(lines)-1]
		if !strings.HasPrefix(lastMessage, "Device mode: ") {
			s.Fatalf("The format of message text is not valid: %q", lastMessage)
		}
		modeString := strings.TrimPrefix(lastMessage, "Device mode: ")
		return modeString
	}

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
		if modeFromAPI := getDeviceModeString(); modeFromAPI != test.modeStatus {
			s.Fatalf("Unexpected getDeviceMode result: got %s; want %s", modeFromAPI, test.modeStatus)
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
