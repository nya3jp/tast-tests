// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// Default value for arc app window minimize limits (DP).
// See default_minimal_size_resizable_task in //device/google/cheets2/overlay/frameworks/base/core/res/res/values/dimens.xml
const defaultMinimalSizeResizableTask = 412

const (
	apk                     = "ArcCompanionLibDemo.apk"
	companionLibDemoPkg     = "org.chromium.arc.companionlibdemo"
	mainActivity            = ".MainActivity"
	resizeActivity          = ".MoveResizeActivity"
	shadowActivity          = ".ShadowActivity"
	unresizableMainActivity = ".UnresizableMainActivity"
	wallpaper               = "white_wallpaper.jpg"
)

type companionLibTestEntry struct {
	name    string
	actName string
	fn      func(context.Context, *arc.ARC, *chrome.Chrome, *chrome.TestConn, *arc.Activity, *ui.Device) error
}

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
	WindowBoundMsg *struct {
		WindowBound string `json:"window_bound"`
	} `json:"WindowBoundMsg"`
	WindowStateMsg *struct {
		WindowState   string `json:"window_state"`
		AppControlled bool   `json:"app_managed"`
	} `json:"WindowStateMsg"`
}

var generalTests = []companionLibTestEntry{
	{"Window State", mainActivity, testWindowState},
	{"Workspace Insets", mainActivity, testWorkspaceInsets},
	{"Caption Button", mainActivity, testCaptionButton},
	{"Device Mode", mainActivity, testDeviceMode},
	{"Caption Height", mainActivity, testCaptionHeight},
	{"Window Bound", mainActivity, testWindowBounds},
	{"Maximize App-controlled Window", mainActivity, testMaximize},
	{"Always on Top Window State", mainActivity, testAlwaysOnTop},
	{"Move and Resize Window", resizeActivity, testResizeWindow},
}

var arcPOnlyTests = []companionLibTestEntry{
	{"Popup Window", mainActivity, testPopupWindow},
	{"Window shadow", shadowActivity, testWindowShadow},
	// TODO(sstan): Add unresizable activity sub-test for ARC R.
	{"Window Bound for Unresizable Activity", unresizableMainActivity, testWindowBounds},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CompanionLibrary,
		Desc:         "Test all ARC++ companion library",
		Contacts:     []string{"sstan@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ArcCompanionLibDemo.apk", "white_wallpaper.jpg"},
		Fixture:      "arcBooted",
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			// Use the android_p dep for running on android P of the container.
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               append(generalTests, arcPOnlyTests...),
		}, {
			// Use the android_vm dep for running on android P and R of the vm.
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               generalTests,
		}},
	})
}

func CompanionLibrary(ctx context.Context, s *testing.State) {

	cr := s.FixtValue().(*arc.PreData).Chrome

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

	a := s.FixtValue().(*arc.PreData).ARC
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to get device: ", err)
	}
	defer d.Close(ctx)

	// Using HTTP server to provide image for wallpaper setting, because this chrome API don't support local file and gs file.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Change the wallpaper to pure white for counting pixels easiler.
	// The Wallpaper will exist continuous if the Chrome session gets reused.
	if err := setWallpaper(ctx, tconn, server.URL+"/"+wallpaper); err != nil {
		s.Error("Failed to set wallpaper: ", err)
	}

	for _, tc := range s.Param().([]companionLibTestEntry) {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			act, err := arc.NewActivity(a, companionLibDemoPkg, tc.actName)
			if err != nil {
				s.Fatal("Failed to create new activity: ", err)
			}
			defer act.Close()

			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed to start activity: ", err)
			}
			defer func(ctx context.Context) {
				if err := act.Stop(ctx, tconn); err != nil {
					s.Error("Failed to stop activity: ", err)
				}
			}(ctx)

			if err := d.WaitForIdle(ctx, 5*time.Second); err != nil {
				s.Fatal("Failed to wait device idle: ", err)
			}

			if err := tc.fn(ctx, a, cr, tconn, act, d); err != nil {
				fileName := fmt.Sprintf("screenshot-companionlib-failed-test-%s.png", strings.ReplaceAll(tc.name, " ", ""))
				path := filepath.Join(s.OutDir(), fileName)
				if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
					s.Log("Failed to capture screenshot: ", err)
				}
				s.Fatalf("%s test failed: %v", tc.name, err)
			}
		})
	}
}

// testWindowShadow verifies that the enable / disable window shadow function from ChromeOS companion library is correct.
func testWindowShadow(ctx context.Context, _ *arc.ARC, cr *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const (
		toggleButtonID         = companionLibDemoPkg + ":id/toggle_shadow"
		shadowStatusTextViewID = companionLibDemoPkg + ":id/toggle_shadow_status_text_view"
	)

	// Change the window to normal state for display the shadow out of edge.
	if err := setWindowStateSync(ctx, tconn, act, arc.WindowStateNormal); err != nil {
		return errors.Wrap(err, "could not set window state to normal")
	}

	dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	dispBounds := coords.ConvertBoundsFromDPToPX(dispInfo.Bounds, dispMode.DeviceScaleFactor)

	// Check the pixel in a small width rectangle box from each edge.
	const shadowWidth = 5

	// TODO(sstan): Using set bound function replace the simple window bounds check.
	winBounds, err := act.WindowBounds(ctx)
	testing.ContextLogf(ctx, "WindowShadow: window = %v, display = %v", winBounds, dispBounds)

	if err != nil {
		return err
	}
	if winBounds.Width >= dispBounds.Width || winBounds.Height >= dispBounds.Height {
		return errors.New("activity is larger than screen so that shadow can't be visible")
	}
	if winBounds.Left < shadowWidth || winBounds.Left+winBounds.Width+shadowWidth >= dispBounds.Width {
		return errors.New("activity haven't enough space to show shadow")
	}

	imgWithShadow, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to grab screenshot")
	}

	// Push button to hide window shadow.
	if err := d.Object(ui.ID(toggleButtonID)).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click shadow toggle button")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		text, err := d.Object(ui.ID(shadowStatusTextViewID)).GetText(ctx)
		// TODO(sstan): Using obj.WaitForExist() before GetText(), rather than Poll it.
		if err != nil {
			return err
		}
		// The TextView will change after shadow hidden.
		if text != "Hidden" {
			return errors.New("still waiting window shadow change")
		}
		return nil
	}, &testing.PollOptions{Timeout: 4 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to hidden window shadow")
	}

	imgWithoutShadow, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to grab screenshot")
	}

	// Comparing bound outside pixels brightness change after hidden window shadow.
	for _, test := range []struct {
		name           string
		x0, y0, x1, y1 int
	}{
		{"left   edge shadow", winBounds.Left - shadowWidth, winBounds.Top, winBounds.Left, winBounds.Top + winBounds.Height},
		{"right  edge shadow", winBounds.Left + winBounds.Width, winBounds.Top, winBounds.Left + winBounds.Width + shadowWidth, winBounds.Top + winBounds.Height},
		{"bottom edge shadow", winBounds.Left, winBounds.Top + winBounds.Height, winBounds.Left + winBounds.Width, winBounds.Top + winBounds.Height + shadowWidth},
	} {
		subImageWithShadow := imgWithShadow.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(image.Rect(test.x0, test.y0, test.x1, test.y1))

		subImageWithoutShadow := imgWithoutShadow.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(image.Rect(test.x0, test.y0, test.x1, test.y1))

		rect := subImageWithShadow.Bounds()
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		brighterPixelsCount, err := imgcmp.CountBrighterPixels(subImageWithShadow, subImageWithoutShadow)
		testing.ContextLogf(ctx, "WindowShadow: Test %s, screenshot rect: %v, totalPixels: %d, brighterPixels: %d", test.name, rect, totalPixels, brighterPixelsCount)
		if err != nil {
			return errors.Wrap(err, "failed to count brighter pixels by subimg in screenshot")
		}

		// This is a rough estimation.
		// If more than half pixels brighter than before in white background, it can be recogenized that the shadow has been hidden.
		const pixelCountPercentageThreshold = 50
		if brighterPixelsCount*100/totalPixels < pixelCountPercentageThreshold {
			return errors.Errorf("%s has not be hidden", test.name)
		}
	}
	return nil
}

// testCaptionHeight verifies that the caption height length getting from ChromeOS companion library is correct.
func testCaptionHeight(ctx context.Context, _ *arc.ARC, _ *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const getCaptionHeightButtonID = companionLibDemoPkg + ":id/get_caption_height"

	dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
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

	appWindow, err := ash.GetARCAppWindowInfo(ctx, tconn, companionLibDemoPkg)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window")
	}

	actualHeight := int(math.Round(float64(appWindow.CaptionHeight) * dispMode.DeviceScaleFactor))

	// actualHeight is translated from DP value. It may have floating error.
	const epsilon = 1
	if abs(actualHeight-msg.CaptionHeightMsg.CaptionHeight) > epsilon {
		return errors.Errorf("wrong caption height: got %v, want %v", msg.CaptionHeightMsg.CaptionHeight, actualHeight)
	}
	return nil

}

// testResizeWindow verifies that the resize function in ChromeOS companion library works as expected.
// ARC companion library demo provide a activity for resize test, there are four draggable hit-boxes in four sides.
// The test maximizing the window by drag from four side inner hit-boxes. The events will be handled by Companion Library, not Chrome.
func testResizeWindow(ctx context.Context, _ *arc.ARC, _ *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	if err := setWindowStateSync(ctx, tconn, act, arc.WindowStateNormal); err != nil {
		return errors.Wrap(err, "could not set window state to normal")
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, companionLibDemoPkg, ash.WindowStateNormal); err != nil {
		return err
	}

	dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}
	appWindow, err := ash.GetARCAppWindowInfo(ctx, tconn, companionLibDemoPkg)
	if err != nil {
		return errors.Wrap(err, "failed to get arc window info")
	}

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open touchscreen device")
	}
	defer tsw.Close()

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}
	if err = tsw.SetRotation(-orientation.Angle); err != nil {
		return err
	}

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not create TouchEventWriter")
	}
	defer stw.Close()

	// Calculate Pixel (screen display) / Tuxel (touch device) ratio.
	dispBounds := coords.ConvertBoundsFromDPToPX(dispInfo.Bounds, dispMode.DeviceScaleFactor)
	pixelToTuxelX := float64(tsw.Width()) / float64(dispBounds.Width)
	pixelToTuxelY := float64(tsw.Height()) / float64(dispBounds.Height)

	captionHeight := int(math.Round(float64(appWindow.CaptionHeight) * dispMode.DeviceScaleFactor))
	bounds := coords.ConvertBoundsFromDPToPX(appWindow.BoundsInRoot, dispMode.DeviceScaleFactor)
	testing.ContextLogf(ctx, "ResizeWindow: The original window bound is %v, try to maximize it by drag inner hit-boxes", bounds)

	// Waiting for hit-boxes UI ready.
	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for idle")
	}

	innerMargin := 5
	middleX := bounds.Left + bounds.Width/2
	middleY := bounds.Top + bounds.Height/2
	for _, test := range []struct {
		startX, startY, endX, endY int
	}{
		{startX: bounds.Left + innerMargin, startY: middleY, endX: 0, endY: middleY},                                    //left
		{startX: bounds.Left + bounds.Width - innerMargin, startY: middleY, endX: dispBounds.Width - 1, endY: middleY},  //right
		{startX: middleX, startY: bounds.Top + innerMargin + captionHeight, endX: middleX, endY: 0},                     //top
		{startX: middleX, startY: bounds.Top + bounds.Height - innerMargin, endX: middleX, endY: dispBounds.Height - 1}, //bottom
	} {
		// Wait for application's UI ready.
		x0 := input.TouchCoord(float64(test.startX) * pixelToTuxelX)
		y0 := input.TouchCoord(float64(test.startY) * pixelToTuxelY)

		x1 := input.TouchCoord(float64(test.endX) * pixelToTuxelX)
		y1 := input.TouchCoord(float64(test.endY) * pixelToTuxelY)

		testing.ContextLogf(ctx, "ResizeWindow: Running the swipe gesture from {%d,%d} to {%d,%d} to ensure to start drag move", x0, y0, x1, y1)
		if err := stw.Swipe(ctx, x0, y0, x1, y1, 2*time.Second); err != nil {
			return errors.Wrap(err, "failed to execute a swipe gesture")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the swipe gesture")
		}
		// Resize by companion library will take long time waiting for application's UI ready.
		if _, err := d.WaitForWindowUpdate(ctx, companionLibDemoPkg, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait window updated after swipe resize")
		}
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		appWindow, err = ash.GetARCAppWindowInfo(ctx, tconn, companionLibDemoPkg)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get arc window info"))
		}
		const epsilon = 2
		if !isSimilarRect(appWindow.BoundsInRoot, dispInfo.WorkArea, epsilon) {
			return errors.Errorf("resize window doesn't have the expected bounds yet; got %v, want %v", appWindow.BoundsInRoot, dispInfo.WorkArea)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}
	return nil
}

// testWorkspaceInsets verifies that the workspace insets info from ChromeOS companion library is correct.
func testWorkspaceInsets(ctx context.Context, _ *arc.ARC, _ *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const getWorkspaceInsetsButtonID = companionLibDemoPkg + ":id/get_workspace_insets"

	dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	// Ensures workspace running in 0 rotation angle scenario.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		errors.Wrap(err, "failed to get display orientation infomation")
	}
	if orientation.Angle != 0 {
		var rot display.RotationAngle
		switch orientation.Angle {
		case 0:
			rot = display.Rotate0
		case 90:
			rot = display.Rotate90
		case 180:
			rot = display.Rotate180
		case 270:
			rot = display.Rotate270
		default:
			return errors.Errorf("unexpected rotation angle: %v", rot)
		}
		if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, rot); err != nil {
			return errors.Wrap(err, "failed to rotate display")
		}
		defer display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, display.Rotate0)
	}

	dispInfo, err = display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	parseRectString := func(s string) (*coords.Rect, error) {
		// The rectangle short string generated by android /frameworks/base/graphics/java/android/graphics/Rect.java
		// Parse it to rectangle format with pixel size.
		var left, top, right, bottom int
		if n, err := fmt.Sscanf(s, "[%d,%d][%d,%d]", &left, &top, &right, &bottom); err != nil {
			return nil, errors.Wrapf(err, "error on parse, Rect text %q", s)
		} else if n != 4 {
			return nil, errors.Errorf("the format of Rect text %q is not valid", s)
		}

		dispBounds := coords.ConvertBoundsFromDPToPX(dispInfo.Bounds, dispMode.DeviceScaleFactor)
		r := coords.NewRectLTRB(left, top, dispBounds.Width-right, dispBounds.Height-bottom)
		return &r, nil
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
		baseMessage, err := getLastJSONMessage(ctx, d)
		if err != nil {
			return errors.Wrap(err, "failed to get last json message")
		}
		if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, test.shelfBehavior); err != nil {
			return errors.Wrapf(err, "failed to set shelf behavior to %v", test.shelfBehavior)
		}
		if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, test.shelfAlignment); err != nil {
			return errors.Wrapf(err, "failed to set shelf alignment to %v", test.shelfAlignment)
		}
		var callbackMessage *companionLibMessage
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Waiting for workspace insets changed callback message
			var err error
			callbackMessage, err = getLastJSONMessage(ctx, d)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get workspace callback message"))
			}
			if callbackMessage.MessageID == baseMessage.MessageID {
				return errors.New("still waiting for workspace callback message coming")
			}
			if callbackMessage.Type != "callback" || callbackMessage.WorkspaceInsetMsg == nil {
				return testing.PollBreak(errors.New("callback message format error"))
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "could not received the callback message")
		}
		var expectedShelfRect coords.Rect
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Confirm the shelf attribute has changed.
			if actualShelfAlignment, err := ash.GetShelfAlignment(ctx, tconn, dispInfo.ID); err != nil {
				return errors.Wrap(err, "failed to get shelf alignment")
			} else if actualShelfAlignment != test.shelfAlignment {
				return errors.Errorf("shelf alignment has not changed yet: got %v, want %v", actualShelfAlignment, test.shelfAlignment)
			}
			dispInfo, err := display.GetInternalInfo(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get internal display info")
			}
			// The unit of WorkArea is DP.
			expectedShelfRect = dispInfo.WorkArea
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "could not change the system shelf alignment")
		}
		// Convert two rectangle to same unit.
		expectedShelfRectPX := coords.ConvertBoundsFromDPToPX(coords.Rect(expectedShelfRect), dispMode.DeviceScaleFactor)
		parsedShelfRectFromCallback, err := parseRectString(callbackMessage.WorkspaceInsetMsg.InsetBound)
		if err != nil {
			return errors.Wrap(err, "failed to parse message")
		}
		const epsilon = 2
		if !isSimilarRect(expectedShelfRectPX, *parsedShelfRectFromCallback, epsilon) {
			return errors.Errorf("Workspace Inset callback is not as expected: got %v, want %v", parsedShelfRectFromCallback, expectedShelfRectPX)
		}
		// Read JSON format window insets size from CompanionLib Demo.
		if err := d.Object(ui.ID(getWorkspaceInsetsButtonID)).Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Get Workspace Insets button")
		}
		var msg *companionLibMessage
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var err error
			msg, err = getLastJSONMessage(ctx, d)
			if err != nil {
				return testing.PollBreak(err)
			}
			// Waiting for new message coming
			if msg.MessageID == callbackMessage.MessageID || msg.WorkspaceInsetMsg == nil {
				return errors.New("still waiting the new json message")
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to get new message of device mode")
		}
		if msg.WorkspaceInsetMsg == nil {
			return errors.Errorf("unexpected JSON message format: no WorkspaceInsetMsg; got %v", msg)
		}
		parsedShelfRect, err := parseRectString(msg.WorkspaceInsetMsg.InsetBound)
		if err != nil {
			return errors.Wrap(err, "failed to parse message")
		}

		// Workspace insets information computed by window shelf info need several numeric conversion, which easy cause floating errors.
		if !isSimilarRect(expectedShelfRectPX, *parsedShelfRect, epsilon) {
			return errors.Errorf("Workspace Inset is not expected: got %v, want %v", parsedShelfRect, expectedShelfRectPX)
		}
	}
	return nil
}

// testCaptionButton verifies that hidden caption button API works as expected.
func testCaptionButton(ctx context.Context, _ *arc.ARC, _ *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const (
		setCaptionButtonID                      = companionLibDemoPkg + ":id/set_caption_buttons_visibility"
		checkCaptionButtonMinimizeBox           = companionLibDemoPkg + ":id/caption_button_minimize"
		checkCaptionButtonMaximizeAndRestoreBox = companionLibDemoPkg + ":id/caption_button_maximize_and_restore"
		checkCaptionButtonLegacyMenuBox         = companionLibDemoPkg + ":id/caption_button_legacy_menu"
		checkCaptionButtonGoBackBox             = companionLibDemoPkg + ":id/caption_button_go_back"
		checkCaptionButtonCloseBox              = companionLibDemoPkg + ":id/caption_button_close"
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
				testing.ContextLogf(ctx, "CaptionButton: Clean %s checkbox statements", checkboxID)
				if err := d.Object(ui.ID(checkboxID)).Click(ctx); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Make sure each caption button can be hidden as expected.
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
		testing.ContextLogf(ctx, "CaptionButton: Test hiding %v caption button", test.buttonCheckboxID)
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
			return errors.New("error while changing hidden caption button")
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			window, err := ash.GetARCAppWindowInfo(ctx, tconn, companionLibDemoPkg)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "error while get ARC window"))
			}
			if window.CaptionButtonVisibleStatus&test.buttonVisibleStatusMask != 0 {
				return errors.Errorf("Caption Button %v still visible", test.buttonCheckboxID)
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "hidden caption button failure")
		}

	}

	// Make sure maximize window always have minimize and close button.
	testing.ContextLog(ctx, "CaptionButton: Test maximize window with hidden caption bar")
	// Maximize the window.
	if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventMaximize); err != nil {
		return err
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
		return err
	}
	// Enable the app-controlled flag using companion library.
	if err := setWindowState(ctx, d, "", true); err != nil {
		return errors.Wrap(err, "failed to enable app controlled flag")
	}
	if err := d.Object(ui.ID(setCaptionButtonID)).Click(ctx); err != nil {
		return errors.Wrap(err, "could not click the setCaptionButton")
	}

	// Hide all caption buttons to hide caption bar.
	for _, id := range []string{
		checkCaptionButtonCloseBox,
		checkCaptionButtonGoBackBox,
		checkCaptionButtonLegacyMenuBox,
		checkCaptionButtonMaximizeAndRestoreBox,
		checkCaptionButtonMinimizeBox,
	} {
		if checked, err := d.Object(ui.ID(id)).IsChecked(ctx); err != nil {
			return errors.Wrapf(err, "could not get the checkbox %v statement", id)
		} else if !checked {
			if err := d.Object(ui.ID(id)).Click(ctx); err != nil {
				return errors.Wrapf(err, "could not check the checkbox %v", id)
			}
		}
	}
	if err := d.Object(ui.Text("OK")).Click(ctx); err != nil {
		return errors.Wrap(err, "could not click the OK button")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		window, err := ash.GetARCAppWindowInfo(ctx, tconn, companionLibDemoPkg)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "could not get ARC window"))
		}
		// Go back button should be hidden in this scenario. Check it first to make sure button hiding process finished.
		if window.CaptionButtonVisibleStatus&ash.CaptionButtonBack != 0 {
			return errors.New("still waiting for Caption Button Back to be hidden")
		}
		if window.CaptionButtonVisibleStatus&ash.CaptionButtonMinimize == 0 {
			return errors.New("Caption Button Minimize shouldn't be visible in maxmize state")
		}
		if window.CaptionButtonVisibleStatus&ash.CaptionButtonClose == 0 {
			return errors.New("Caption Button Close shouldn't be visible in maxmize state")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "maximized window's caption button behavior check failure")
	}
	// Disable the app-controlled flag using companion library.
	if err := setWindowState(ctx, d, "", false); err != nil {
		return errors.Wrap(err, "failed to disable app controlled flag")
	}
	// Change the window state back to normal.
	if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
		return err
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
		return err
	}
	return nil
}

// testDeviceMode verifies that the device mode info from ChromeOS companion library is correct.
func testDeviceMode(ctx context.Context, _ *arc.ARC, _ *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const getDeviceModeButtonID = companionLibDemoPkg + ":id/get_device_mode_button"

	if err := setWindowStateSync(ctx, tconn, act, arc.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to set window normal state before testing device mode change")
	}
	originalTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the tablet mode status")
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, originalTabletMode)
	for _, test := range []struct {
		// isTabletMode represents current mode of system which is Tablet mode or clamshell mode.
		isTabletMode bool
		// modeStatus represents the expection of device mode string getting from companion library.
		modeStatus string
	}{
		{isTabletMode: true, modeStatus: "TABLET"}, // Default mode is clamshell mode, the test change it to tablet mode first to test the callback message.
		{isTabletMode: false, modeStatus: "CLAMSHELL"},
	} {
		// Read latest message. Each test procedure will cause two messages, a callback and an info messages.
		baseMessage, err := getLastJSONMessage(ctx, d)
		if err != nil {
			return errors.Wrap(err, "failed to get base json message")
		}

		// Force Chrome to be in specific system mode.
		if err := ash.SetTabletModeEnabled(ctx, tconn, test.isTabletMode); err != nil {
			return errors.Wrap(err, "failed to set the system mode")
		}

		// TODO(b/146846841): From M81, the device mode change callback function flaky.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// TODO(crbug.com/1002958): Wait for "tablet mode animation is finished" in a reliable way.
			// If an activity is launched while the tablet mode animation is active, the activity
			// will be launched in un undefined state, making the test flaky.
			tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get whether the device sitting in tablet mode"))
			}
			if tabletModeEnabled != test.isTabletMode {
				return errors.Wrap(err, "failed to switch device mode by tast test library")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait devicde mode changed")
		}

		// The system mode change may generate both mode change callback and workspace change callback.
		var callbackmsg companionLibMessage
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var tempMsg companionLibMessage
			lines, err := getJSONTextViewContent(ctx, d)
			if err != nil {
				return errors.Wrap(err, "failed to get json text")
			}
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &tempMsg); err != nil {
				return errors.Wrap(err, "parse callback message failure")
			}
			// Waiting for new message coming.
			if baseMessage.MessageID == tempMsg.MessageID {
				return errors.New("still waiting the callback json message")
			}
			// If the latest message not the device change callback, check the message before that.
			if tempMsg.Type == "callback" && tempMsg.DeviceModeMsg != nil {
				callbackmsg = tempMsg
			} else {
				if len(lines) < 2 {
					return errors.New("still waiting the callback json message")
				}
				if err := json.Unmarshal([]byte(lines[len(lines)-2]), &callbackmsg); err != nil {
					return errors.Wrap(err, "parse callback message failure")
				}
				if callbackmsg.Type != "callback" || callbackmsg.DeviceModeMsg == nil {
					return errors.New("error on callback message generation")
				}
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to get callback message of device mode changed")
		}
		if callbackmsg.DeviceModeMsg == nil {
			return errors.Errorf("unexpected JSON message format: no DeviceModeMsg; got %v", callbackmsg)
		}
		if callbackmsg.DeviceModeMsg.DeviceMode != test.modeStatus {
			return errors.Errorf("unexpected device mode changed callback message result: got %s; want %s", callbackmsg.DeviceModeMsg.DeviceMode, test.modeStatus)
		}

		if err := d.Object(ui.ID(getDeviceModeButtonID)).Click(ctx); err != nil {
			return errors.Wrap(err, "could not click the getDeviceMode button")
		}

		var msg *companionLibMessage
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var err error
			msg, err = getLastJSONMessage(ctx, d)
			if err != nil {
				return testing.PollBreak(err)
			}
			// Waiting for new message coming
			if callbackmsg.MessageID == msg.MessageID {
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

// testAlwaysOnTop verifies the always on top window work as expected.
func testAlwaysOnTop(ctx context.Context, a *arc.ARC, cr *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const (
		settingPkgName         = "com.android.settings"
		settingActName         = ".Settings"
		getWindowStateButtonID = companionLibDemoPkg + ":id/get_task_window_state_button"
	)
	// Change the window to normal state first, making sure the UI can be touched by tast test library.
	if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
		return err
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
		return err
	}

	captionHeight, err := act.CaptionHeight(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get caption height")
	}
	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get window bounds")
	}

	if err := setWindowState(ctx, d, "Always on top", false); err != nil {
		return errors.Wrap(err, "could not set Always On Top window state")
	}
	defer setWindowState(ctx, d, "Normal", false)

	// Waiting for the window prepared.
	if err := d.Object(ui.ID(getWindowStateButtonID)).WaitForExists(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to touch window state button")
	}

	imageBeforeActive, err := getWindowCaptionScreenshot(ctx, cr, bounds.Top, bounds.Left, captionHeight, bounds.Width)
	if err != nil {
		return errors.Wrap(err, "could not get screen shot before active other window")
	}

	settingAct, err := arc.NewActivity(a, settingPkgName, settingActName)
	if err != nil {
		return errors.Wrap(err, "could not create Settings Activity")
	}
	defer settingAct.Close()

	if err := settingAct.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "could not start Settings Activity")
	}
	defer settingAct.Stop(ctx, tconn)

	// Make sure the setting window will have an initial maximized state.
	if err := setWindowStateSync(ctx, tconn, settingAct, arc.WindowStateMaximized); err != nil {
		return errors.Wrap(err, "failed to set window state of Settings Activity to maximized")
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, settingPkgName, ash.WindowStateMaximized); err != nil {
		return errors.Wrap(err, "setting window was not maximized")
	}

	imageAfterActive, err := getWindowCaptionScreenshot(ctx, cr, bounds.Top, bounds.Left, captionHeight, bounds.Width)
	if err != nil {
		return errors.Wrap(err, "could not get screen shot after active other window")
	}

	const roundingErrorThreshold = 1
	diffPixelNum, err := imgcmp.CountDiffPixels(imageBeforeActive, imageAfterActive, roundingErrorThreshold)
	if err != nil {
		return errors.Wrap(err, "error on count match pixels")
	}
	percent := diffPixelNum * 100 / (bounds.Width * captionHeight)

	// When the window lost focus, the color of caption button image will change.
	const lostFocusThreshold = 1
	if percent < lostFocusThreshold {
		return errors.New("always on top window still keep focused")
	}
	// In this case the window will be covered by setting window.
	const notOnTopThreshold = 10
	if percent > notOnTopThreshold {
		return errors.New("always on top window not on top")
	}
	return nil
}

// testPopupWindow verifies that popup window's behaviors works as expected.
func testPopupWindow(ctx context.Context, a *arc.ARC, cr *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const (
		showPopupWindowButtonID = companionLibDemoPkg + ":id/popup_window_button"
		clipToTaskCheckboxID    = companionLibDemoPkg + ":id/clip_to_task_bounds"
		dismissButtonID         = companionLibDemoPkg + ":id/dismiss"
		popupWindowString       = "Popup Window"
	)

	countPopupWindowPixelPercentage := func(captionImage image.Image) float64 {
		// https://developer.android.com/reference/android/R.color#holo_blue_light
		holoBlueLight := color.RGBA{0x33, 0xb5, 0xe5, 0xff}
		rect := captionImage.Bounds()
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		popupWindowPixelsCount := imgcmp.CountPixels(captionImage, holoBlueLight)
		return float64(popupWindowPixelsCount) * 100.0 / float64(totalPixels)
	}

	dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	// Set window on the top of the workspace insets. Make sure the framework can ignore the caption bar size of popup window layer. See b/147783396.
	dispBounds := coords.ConvertBoundsFromDPToPX(dispInfo.Bounds, dispMode.DeviceScaleFactor)
	setWindowBounds(ctx, d, coords.NewRect(0, 0, dispBounds.Width, dispBounds.Height))

	captionHeight, err := act.CaptionHeight(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get caption height")
	}
	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get window bounds")
	}

	if err := d.Object(ui.ID(showPopupWindowButtonID)).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click popup window button")
	}

	// Check the popup window has poped.
	if err := d.Object(ui.Text(popupWindowString)).WaitForExists(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to popup window")
	}
	clipWindowCaption, err := getWindowCaptionScreenshot(ctx, cr, bounds.Top, bounds.Left, captionHeight, bounds.Width)
	if err != nil {
		return errors.Wrap(err, "failed to get clip window caption screenshot")
	}

	// In initial state, the popup window should be cliped to the task window bounds, which means it should be covered by window caption.
	clipWindowCoverPercentage := countPopupWindowPixelPercentage(clipWindowCaption)
	if clipWindowCoverPercentage > 0 {
		testing.ContextLog(ctx, "PopupWindow: Clip popup window cover percentage: ", clipWindowCoverPercentage)
		return errors.New("unexpected popup window bound: got uncliped; want cliped")
	}

	if err := d.Object(ui.ID(clipToTaskCheckboxID)).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click the checkbox to disable clip bound")
	}
	if err := d.Object(ui.ID(dismissButtonID)).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click dismiss button")
	}
	if err := d.Object(ui.ID(showPopupWindowButtonID)).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click popup window button to show unclip window")
	}
	// Check the popup window has poped.
	if err := d.Object(ui.Text(popupWindowString)).WaitForExists(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to popup window")
	}

	// After disable the clip, the popup window should be cover the window caption.
	unclipWindowCaption, err := getWindowCaptionScreenshot(ctx, cr, bounds.Top, bounds.Left, captionHeight, bounds.Width)
	if err != nil {
		return errors.Wrap(err, "failed to get unclip window caption screenshot")
	}
	upclipWindowCoverPercentage := countPopupWindowPixelPercentage(unclipWindowCaption)
	if upclipWindowCoverPercentage == 0 {
		testing.ContextLog(ctx, "PopupWindow: Unclip popup window cover percentage: ", upclipWindowCoverPercentage)
		return errors.New("unexpected popup window bound: got cliped; want uncliped")
	}

	if err := d.Object(ui.ID(dismissButtonID)).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click dismiss button on unclip window")
	}
	return nil
}

// testWindowState verifies that change window state by ChromeOS companion library works as expected.
func testWindowState(ctx context.Context, _ *arc.ARC, _ *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const (
		setWindowStateButtonID = companionLibDemoPkg + ":id/set_task_window_state_button"
		getWindowStateButtonID = companionLibDemoPkg + ":id/get_task_window_state_button"
	)

	for _, test := range []struct {
		windowStateStr string
		windowStateExp ash.WindowStateType
		isAppManaged   bool
	}{
		{windowStateStr: "Maximize", windowStateExp: ash.WindowStateMaximized, isAppManaged: false},
		{windowStateStr: "Normal", windowStateExp: ash.WindowStateNormal, isAppManaged: false},
		{windowStateStr: "Minimize", windowStateExp: ash.WindowStateMinimized, isAppManaged: false},
	} {
		testing.ContextLogf(ctx, "WindowState: Testing windowState=%v, appManaged=%t", test.windowStateStr, test.isAppManaged)

		if err := setWindowState(ctx, d, test.windowStateStr, test.isAppManaged); err != nil {
			return errors.Wrap(err, "error while setting window state")
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), test.windowStateExp); err != nil {
			return errors.Wrapf(err, "error while waiting %v setting up", test.windowStateExp)
		}
	}
	return nil
}

// testMaximize verifies that the app-controlled window cannot be maximized by double click caption after the maximize button has been hidden.
func testMaximize(ctx context.Context, _ *arc.ARC, _ *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const (
		setCaptionButtonID                      = companionLibDemoPkg + ":id/set_caption_buttons_visibility"
		checkCaptionButtonMaximizeAndRestoreBox = companionLibDemoPkg + ":id/caption_button_maximize_and_restore"
	)

	// Click hidden maximize button checkbox in dialog.
	clickMaximizeCheckbox := func() error {
		if err := d.Object(ui.ID(setCaptionButtonID)).WaitForExists(ctx, 5*time.Second); err != nil {
			return errors.New("failed to find set window caption button")
		}
		if err := d.Object(ui.ID(setCaptionButtonID)).Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click set window caption button")
		}
		if err := d.Object(ui.Text("OK")).WaitForExists(ctx, 5*time.Second); err != nil {
			return errors.Wrap(err, "failed to open set window caption button dialog")
		}
		if err := d.Object(ui.ID(checkCaptionButtonMaximizeAndRestoreBox)).Click(ctx); err != nil {
			return errors.Wrap(err, "failed to check the maximize checkbox")
		}
		if err := d.Object(ui.Text("OK")).Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click OK button")
		}
		return nil
	}

	dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	// Change the window to normal state to make sure the UI can be touched by the Tast test library.
	if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
		return err
	}
	// Check the maximize & restore button checkbox in Hidden Caption Button Dialog.
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
		return err
	}

	// Enable the app-controlled flag using the companion library.
	if err := setWindowState(ctx, d, "", true); err != nil {
		return errors.Wrap(err, "failed to enabled app controlled flag")
	}

	// Hide maximize and restore caption button.
	if err := clickMaximizeCheckbox(); err != nil {
		return errors.Wrap(err, "failed to click the hidden maximize checkbox")
	}

	// Double click the caption.
	const doubleClickGap = 100 * time.Millisecond
	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get activity bounds")
	}
	middleCaptionLoc := coords.NewPoint(
		int(float64(bounds.Top+5)/dispMode.DeviceScaleFactor),
		int(float64(bounds.Left+bounds.Width/2)/dispMode.DeviceScaleFactor))
	if err := mouse.Click(ctx, tconn, middleCaptionLoc, mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click window caption the first time")
	}

	if err := testing.Sleep(ctx, doubleClickGap); err != nil {
		return errors.Wrap(err, "failed to wait for the gap between the double click")
	}
	if err := mouse.Click(ctx, tconn, middleCaptionLoc, mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click window caption the second time")
	}

	if state, err := act.GetWindowState(ctx); err != nil {
		return errors.Wrap(err, "could not get the window state")
	} else if state == arc.WindowStateMaximized {
		return errors.New("window shouldn't be maximized by double click")
	}

	// Disable the app-controlled flag using companion library.
	if err := setWindowState(ctx, d, "", false); err != nil {
		return errors.Wrap(err, "failed to disable app controlled flag")
	}

	// Restore maximize caption button.
	if err := clickMaximizeCheckbox(); err != nil {
		return errors.Wrap(err, "failed to click the maximize checkbox")
	}
	return nil
}

// testWindowBounds verifies that the window bounds related API works as expected in ChromeOS Companion Lib.
func testWindowBounds(ctx context.Context, _ *arc.ARC, _ *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const getWindowBoundsButtonID = companionLibDemoPkg + ":id/get_window_bounds_button"

	physicalDisplayDensity, err := act.DisplayDensity(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get physical display density")
	}

	dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	// Each ARC app window has limitation of window bounds. The CompanionLib Demo use default window bound size.
	minimizeSize := int(math.Round(defaultMinimalSizeResizableTask * physicalDisplayDensity))

	// In clamshell mode, set window bound cannot set the window higher than caption.
	// Get caption height for calculate expected window bound.
	captionHeight, err := act.CaptionHeight(ctx)
	if err != nil {
		return err
	}

	dispBoundsPX := coords.ConvertBoundsFromDPToPX(dispInfo.Bounds, dispMode.DeviceScaleFactor)
	shelfHeightPX := dispBoundsPX.Height - int(math.Round(float64(dispInfo.WorkArea.Height)*dispMode.DeviceScaleFactor))

	// Change the window to normal state for make sure the bounds of window can be set.
	if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
		return err
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
		return err
	}

	initAshWindow, err := ash.GetARCAppWindowInfo(ctx, tconn, companionLibDemoPkg)
	if err != nil {
		return err
	}

	initBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get window bounds")
	}
	testing.ContextLogf(ctx, "WindowBounds: original bounds rect: %v, minimize length: %v, caption height: %v", initBounds, minimizeSize, captionHeight)

	originalShelfAlignment, err := ash.GetShelfAlignment(ctx, tconn, dispInfo.ID)
	if err != nil {
		return errors.Wrap(err, "failed to get shelf alignmnet")
	}
	defer ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, originalShelfAlignment)

	// It is possible that some TextView be set outside window, which would cause tast library cannot read messages.
	// Should avoid this case in test.
	for _, test := range []struct {
		name string
		// Format of coords.Rect is {top, left, width, height}, and it's not input for SetBounds function.
		settingBound  coords.Rect
		expectedBound coords.Rect
	}{
		{"trigger min size limit", coords.NewRect(0, 0, 0, 0), coords.NewRect(0, captionHeight, minimizeSize, minimizeSize)},
		{"trigger min size limit again", coords.NewRect(0, captionHeight/2, minimizeSize/2, minimizeSize/2), coords.NewRect(0, captionHeight, minimizeSize, minimizeSize)},
		{"fullscreen size", coords.NewRect(0, 0, dispBoundsPX.Width, dispBoundsPX.Height), coords.NewRect(0, captionHeight, dispBoundsPX.Width, dispBoundsPX.Height-captionHeight-shelfHeightPX)}, // Auto maximize. It means the edge will not over the shelf
	} {
		// The expected window bound depends on setting window bound and can be
		// calculated directly, according to the window bound behavior.
		if err := setWindowBounds(ctx, d, test.settingBound); err != nil {
			return errors.Wrap(err, "failed to setting window bound")
		}

		// Because the conversion of DP to PX, we should be lenient with the epsilon.
		const epsilon = 2

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			w, err := ash.GetARCAppWindowInfo(ctx, tconn, companionLibDemoPkg)
			if err != nil {
				return err
			}

			chromeBounds := coords.ConvertBoundsFromDPToPX(w.BoundsInRoot, dispMode.DeviceScaleFactor)
			chromeBounds.Top += captionHeight
			chromeBounds.Height -= captionHeight
			if !isSimilarRect(chromeBounds, test.expectedBound, epsilon) {
				return errors.Errorf("Chrome bounds are different on subtest %v: got %v, want %v", test.name, chromeBounds, test.expectedBound)
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for window bound change")
		}

		bound, err := windowBounds(ctx, d)
		if err != nil {
			return errors.Wrap(err, "failed to get window bound from UI message")
		}
		if !isSimilarRect(coords.Rect(bound), coords.Rect(test.expectedBound), epsilon) {
			return errors.Errorf("wrong window bound on subtest %v, set %v: got %v, want %v", test.name, test.settingBound, bound, test.expectedBound)
		}

		if ashWindow, err := ash.GetARCAppWindowInfo(ctx, tconn, companionLibDemoPkg); err != nil {
			return errors.Wrap(err, "failed to get window info")
		} else if ashWindow.CanResize != initAshWindow.CanResize {
			return errors.Errorf("unexpectedly changed window resizeability on subtest %v: got %t, want %t", test.name, ashWindow.CanResize, initAshWindow.CanResize)
		}
	}

	// Check that app-controlled state is not modified by bounds change.
	if appControlled, err := isAppControlled(ctx, d); err != nil {
		return err
	} else if appControlled == true {
		return errors.New("unexpectedly changed app controlled state to true")
	}

	// Set app-controlled and resize back to the initial bounds.
	// This should not change app-controlled state as well.
	if err := setWindowState(ctx, d, "", true); err != nil {
		return errors.Wrap(err, "failed to enable app controlled flag")
	}
	if err := setWindowBounds(ctx, d, initBounds); err != nil {
		return errors.Wrap(err, "failed to setting window bound")
	}

	if appControlled, err := isAppControlled(ctx, d); err != nil {
		return err
	} else if appControlled == false {
		return errors.New("unexpectedly changed app controlled state to true")
	}

	return nil
}

// setWindowBounds uses CompanionLib Demo UI operation to setting the window bounds.
// Only works on the window which has Normal State.
func setWindowBounds(ctx context.Context, d *ui.Device, bound coords.Rect) error {
	const (
		setWindowBoundsButtonID = companionLibDemoPkg + ":id/set_window_bounds_button"
		topNumberTextID         = companionLibDemoPkg + ":id/top_number_text"
		bottomNumberTextID      = companionLibDemoPkg + ":id/bottom_number_text"
		rightNumberTextID       = companionLibDemoPkg + ":id/right_number_text"
		leftNumberTextID        = companionLibDemoPkg + ":id/left_number_text"
	)

	if err := d.Object(ui.ID(setWindowBoundsButtonID)).WaitForExists(ctx, 5*time.Second); err != nil {
		return errors.New("failed to find set window bounds button")
	}
	if err := d.Object(ui.ID(setWindowBoundsButtonID)).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click set window bounds button")
	}
	if err := d.Object(ui.Text("OK")).WaitForExists(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to open set window bounds dialog")
	}

	if err := d.Object(ui.ID(leftNumberTextID)).SetText(ctx, strconv.Itoa(bound.Left)); err != nil {
		return errors.Wrap(err, "failed to set left number")
	}
	if err := d.Object(ui.ID(topNumberTextID)).SetText(ctx, strconv.Itoa(bound.Top)); err != nil {
		return errors.Wrap(err, "failed to set top number")
	}
	if err := d.Object(ui.ID(rightNumberTextID)).SetText(ctx, strconv.Itoa(bound.Left+bound.Width)); err != nil {
		return errors.Wrap(err, "failed to set right number")
	}
	if err := d.Object(ui.ID(bottomNumberTextID)).SetText(ctx, strconv.Itoa(bound.Top+bound.Height)); err != nil {
		return errors.Wrap(err, "failed to set bottom number")
	}
	if err := d.Object(ui.Text("OK")).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click OK button")
	}
	return nil
}

// windowBounds uses CompanionLib Demo UI operation to getting the window bounds.
func windowBounds(ctx context.Context, d *ui.Device) (coords.Rect, error) {
	const getWindowBoundsButtonID = companionLibDemoPkg + ":id/get_window_bounds_button"

	parseBoundFromMsg := func(msg *companionLibMessage) (coords.Rect, error) {
		// Parse Rect short string to rectangle format with built-in pixel size.
		var left, top, right, bottom int
		if msg.WindowBoundMsg == nil {
			return coords.Rect{}, errors.New("not a window bound message")
		}
		if n, err := fmt.Sscanf(msg.WindowBoundMsg.WindowBound, "[%d,%d][%d,%d]", &left, &top, &right, &bottom); err != nil {
			return coords.Rect{}, errors.Wrap(err, "error on parse Rect text")
		} else if n != 4 {
			return coords.Rect{}, errors.Errorf("the format of Rect text is not valid: %q", msg.WindowBoundMsg.WindowBound)
		}
		return coords.NewRectLTRB(left, top, right, bottom), nil
	}

	lastMsg, err := getLastJSONMessage(ctx, d)
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "error on get last JSON message")
	}
	// Get window bound message in JSON format TextView.
	if err := d.Object(ui.ID(getWindowBoundsButtonID)).Click(ctx); err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to click get window bound button")
	}
	// Waiting for window bound changed and check it work as expected.
	var msg *companionLibMessage
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		msg, err = getLastJSONMessage(ctx, d)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "error on get new JSON message"))
		}
		if msg.MessageID == lastMsg.MessageID {
			return errors.New("still waiting new window bound message")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to get window bound")
	}
	return parseBoundFromMsg(msg)
}

// setWindowState uses CompanionLib Demo UI operation to set the window state.
// About app controlled, see go/arc++-support-library.
func setWindowState(ctx context.Context, d *ui.Device, windowStateStr string, isAppControlled bool) error {
	const setWindowStateButtonID = companionLibDemoPkg + ":id/set_task_window_state_button"
	const appControlledCheckboxText = "App Managed"

	if err := d.Object(ui.ID(setWindowStateButtonID)).WaitForExists(ctx, 5*time.Second); err != nil {
		return errors.New("failed to find set window state button")
	}
	if err := d.Object(ui.ID(setWindowStateButtonID)).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click set window state button")
	}
	if err := d.Object(ui.Text("OK")).WaitForExists(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to open set window state dialog")
	}

	// Set empty windowStateStr can avoid specify a window state in dialog.
	if windowStateStr != "" {
		if isClickable, err := d.Object(ui.Text(windowStateStr)).IsClickable(ctx); err != nil {
			return errors.Wrap(err, "failed check the radio clickable")
		} else if isClickable {
			// If isClickable = false, it will do nothing because the test application logic will automatically check the current window state radio. It can't be clicked if the state radio has been clicked.
			if err := d.Object(ui.Text(windowStateStr)).Click(ctx); err != nil {
				return errors.Wrapf(err, "failed to click %v", windowStateStr)
			}
		}
	}

	// Change the app controlled checkbox.
	checked, err := d.Object(ui.Text(appControlledCheckboxText)).IsChecked(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check App Controlled checkbox")
	}
	if checked != isAppControlled {
		if err := d.Object(ui.Text(appControlledCheckboxText)).Click(ctx); err != nil {
			return errors.Wrap(err, "failed to change the App Controlled checkbox")
		}
	}

	if err := d.Object(ui.Text("OK")).Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click OK button")
	}
	return nil
}

func isAppControlled(ctx context.Context, d *ui.Device) (bool, error) {
	const getWindowStateButtonID = companionLibDemoPkg + ":id/get_task_window_state_button"

	lastMsg, err := getLastJSONMessage(ctx, d)
	if err != nil {
		return false, errors.Wrap(err, "error on get last JSON message")
	}
	if err := d.Object(ui.ID(getWindowStateButtonID)).Click(ctx); err != nil {
		return false, errors.Wrap(err, "failed to click get window bound button")
	}

	var msg *companionLibMessage
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		msg, err = getLastJSONMessage(ctx, d)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "error on get new JSON message"))
		}
		if msg.MessageID == lastMsg.MessageID {
			return errors.New("still waiting for a new window state message")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return false, errors.Wrap(err, "failed to get window state")
	}

	if msg.WindowStateMsg == nil {
		return false, errors.Errorf("unexpected JSON message format: no WindowStateMsg; got %v", msg)
	}
	return msg.WindowStateMsg.AppControlled, nil
}

// setWindowStateSync returns after the window state changed as expected.
func setWindowStateSync(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, state arc.WindowState) error {
	if err := act.SetWindowState(ctx, tconn, state); err != nil {
		return errors.Wrap(err, "could not set window state")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if currentState, err := act.GetWindowState(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "could not get the window state"))
		} else if currentState != state {
			return errors.Errorf("window state has not changed yet: got %s; want %s", currentState, state)
		}
		return nil
	}, &testing.PollOptions{Timeout: 4 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to waiting for window state change")
	}
	return nil
}

// getJSONTextViewContent returns all text in JSON textview.
func getJSONTextViewContent(ctx context.Context, d *ui.Device) ([]string, error) {
	const JSONTextViewID = companionLibDemoPkg + ":id/status_jsontext_view"
	if err := d.Object(ui.ID(JSONTextViewID)).WaitForExists(ctx, 5*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to wait JSON textview ready")
	}
	text, err := d.Object(ui.ID(JSONTextViewID)).GetText(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "JSONStatusTextView not ready yet")
	}
	return strings.Split(text, "\n"), nil
}

// getLastJSONMessage returns last JSON format output message of ChromeOS Companion Library Demo
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

// setWallpaper setting given URL as ChromeOS wallpaper.
func setWallpaper(ctx context.Context, tconn *chrome.TestConn, wallpaperURL string) error {
	return tconn.Call(ctx, nil, `(url) => tast.promisify(chrome.wallpaper.setWallpaper)({
		  url: url,
		  layout: 'STRETCH',
		  filename: 'test_wallpaper'
		})`, wallpaperURL)
}

// getWindowCaptionScreenshot returns a screenshot image of window caption bar.
func getWindowCaptionScreenshot(ctx context.Context, cr *chrome.Chrome, captionTopPX, captionLeftPX, captionHeightPX, captionWidthPX int) (image.Image, error) {
	img, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to grab screenshot")
	}
	captionImage := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(captionLeftPX, captionTopPX, captionLeftPX+captionWidthPX, captionTopPX+captionHeightPX))
	return captionImage, nil
}

// abs returns absolute value of integer parameter
func abs(num int) int {
	if num >= 0 {
		return num
	}
	return -num
}

// isSimilarRect compares two rectangle whether their similar by epsilon.
func isSimilarRect(lhs, rhs coords.Rect, epsilon int) bool {
	return abs(lhs.Left-rhs.Left) <= epsilon && abs(lhs.Width-rhs.Width) <= epsilon && abs(lhs.Top-rhs.Top) <= epsilon && abs(lhs.Height-rhs.Height) <= epsilon
}
