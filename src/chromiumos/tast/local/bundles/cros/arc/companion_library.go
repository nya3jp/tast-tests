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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/screenshot"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	screenshotCR "chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CompanionLibrary,
		Desc:     "Test all ARC++ companion library",
		Contacts: []string{"sstan@google.com", "arc-framework+tast@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// TODO(sstan): Remove display_backlight once crbug.com/950346 support hardware dependency.
		SoftwareDeps: []string{"android_p", "chrome", "display_backlight"},
		Data:         []string{"ArcCompanionLibDemo.apk", "white_wallpaper.jpg"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

const pkg = "org.chromium.arc.companionlibdemo"

// Default value for arc app window minimize limits (DP).
// See default_minimal_size_resizable_task in //device/google/cheets2/overlay/frameworks/base/core/res/res/values/dimens.xml
const defaultMinimalSizeResizableTask = 412

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
}

func CompanionLibrary(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcCompanionLibDemo.apk"

		mainActivity     = ".MainActivity"
		resizeActivityID = ".MoveResizeActivity"
		shadowActivityID = ".ShadowActivity"
		wallpaper        = "white_wallpaper.jpg"
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

	// Using HTTP server to provide image for wallpaper setting, because this chrome API don't support local file and gs file.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Change the wallpaper to pure white for counting pixels easiler.
	// The Wallpaper will exist continuous if the Chrome session gets reused.
	if err := setWallpaper(ctx, tconn, server.URL+"/"+wallpaper); err != nil {
		s.Error("Failed to set wallpaper: ", err)
	}

	// All of tests in this block running on MainActivity.
	type testFunc func(context.Context, *chrome.TestConn, *arc.Activity, *ui.Device) error
	for idx, test := range []struct {
		name string
		fn   testFunc
	}{
		{"Window State", testWindowState},
		{"Workspace Insets", testWorkspaceInsets},
		{"Caption Button", testCaptionButton},
		{"Device Mode", testDeviceMode},
		{"Caption Height", testCaptionHeight},
		{"Window Bound", testWindowBounds},
		{"Maximize App-controlled Window", testMaximize},
	} {
		s.Log("Running ", test.name)
		if err := act.Start(ctx); err != nil {
			s.Fatal("Failed to start context: ", err)
		}
		if err := act.WaitForResumed(ctx, time.Second); err != nil {
			s.Fatal("Failed to wait for activity to resuyme: ", err)
		}
		if err := test.fn(ctx, tconn, act, d); err != nil {
			path := fmt.Sprintf("%s/screenshot-companionlib-failed-test-%d.png", s.OutDir(), idx)
			if err := screenshotCR.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
			s.Errorf("%s test failed: %v", test.name, err)
		}
		if err := act.Stop(ctx); err != nil {
			s.Fatal("Failed to stop context: ", err)
		}
	}

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start context: ", err)
	}
	s.Log("Running Popup Window")
	if err := testPopupWindow(ctx, cr, tconn, act, d); err != nil {
		path := filepath.Join(s.OutDir(), "screenshot-companionlib-failed-test-popupwindow.png")
		if err := screenshotCR.CaptureChrome(ctx, cr, path); err != nil {
			s.Log("Failed to capture screenshot: ", err)
		}
		s.Error("Popup window test failed: ", err)
	}
	if err := act.Stop(ctx); err != nil {
		s.Fatal("Failed to stop context: ", err)
	}

	// These test running on specific activity.
	shadowAct, err := arc.NewActivity(a, pkg, shadowActivityID)
	if err != nil {
		s.Fatal("Could not create ResizeActivity: ", err)
	}
	if err := shadowAct.Start(ctx); err != nil {
		s.Fatal("Could not start ResizeActivity: ", err)
	}
	s.Log("Running Window shadow")
	if err := setWindowStateSync(ctx, shadowAct, arc.WindowStateNormal); err != nil {
		s.Fatal("Could not set window normal state: ", err)
	}
	if err := testWindowShadow(ctx, cr, tconn, shadowAct, d); err != nil {
		path := filepath.Join(s.OutDir(), "screenshot-companionlib-failed-test-windowshadow.png")
		if err := screenshotCR.CaptureChrome(ctx, cr, path); err != nil {
			s.Log("Failed to capture screenshot: ", err)
		}
		s.Error("Window shadow test failed: ", err)
	}
	if err := shadowAct.Stop(ctx); err != nil {
		s.Fatal("Could not stop resize activity: ", err)
	}

	resizeAct, err := arc.NewActivity(a, pkg, resizeActivityID)
	if err != nil {
		s.Fatal("Could not create ResizeActivity: ", err)
	}
	s.Log("Running Window Resize")
	if err := resizeAct.Start(ctx); err != nil {
		s.Fatal("Could not start ResizeActivity: ", err)
	}
	defer func() {
		if err := resizeAct.Stop(ctx); err != nil {
			s.Fatal("Could not stop resize activity: ", err)
		}
	}()
	if err := setWindowStateSync(ctx, resizeAct, arc.WindowStateNormal); err != nil {
		s.Fatal("Could not set window normal state: ", err)
	}
	if err := testResizeWindow(ctx, tconn, resizeAct, d); err != nil {
		s.Error("Move & Resize Window test failed: ", err)
	}
}

// testWindowShadow verifies that the enable / disable window shadow function from ChromeOS companion library is correct.
func testWindowShadow(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const (
		toggleButtonID         = pkg + ":id/toggle_shadow"
		shadowStatusTextViewID = pkg + ":id/toggle_shadow_status_text_view"
	)

	// Change the window to normal state for display the shadow out of edge.
	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		return errors.Wrap(err, "could not set window state to normal")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if state, err := act.GetWindowState(ctx); err != nil {
			return err
		} else if state != arc.WindowStateNormal {
			return errors.Errorf("window state has not changed yet: got %s; want %s", state, arc.WindowStateNormal)
		}
		return nil
	}, &testing.PollOptions{Timeout: 4 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to waiting for change to normal window state")
	}

	dispMode, err := ash.InternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}

	// Check the pixel in a small width rectangle box from each edge.
	const shadowWidth = 5

	// TODO(sstan): Using set bound function replace the simple window bounds check.
	bounds, err := act.WindowBounds(ctx)
	testing.ContextLogf(ctx, "WindowShadow: bound %v,%v; dispMode W:%v, H:%v", bounds.Width, bounds.Height, dispMode.WidthInNativePixels, dispMode.HeightInNativePixels)

	if err != nil {
		return err
	}
	if bounds.Width >= dispMode.WidthInNativePixels || bounds.Height >= dispMode.HeightInNativePixels {
		return errors.New("activity is larger than screen so that shadow can't be visible")
	}
	if bounds.Left < shadowWidth || bounds.Left+bounds.Width+shadowWidth >= dispMode.WidthInNativePixels {
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
		{"left   edge shadow", bounds.Left - shadowWidth, bounds.Top, bounds.Left, bounds.Top + bounds.Height},
		{"right  edge shadow", bounds.Left + bounds.Width, bounds.Top, bounds.Left + bounds.Width + shadowWidth, bounds.Top + bounds.Height},
		{"bottom edge shadow", bounds.Left, bounds.Top + bounds.Height, bounds.Left + bounds.Width, bounds.Top + bounds.Height + shadowWidth},
	} {
		subImageWithShadow := imgWithShadow.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(image.Rect(test.x0, test.y0, test.x1, test.y1))

		subImageWithoutShadow := imgWithoutShadow.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(image.Rect(test.x0, test.y0, test.x1, test.y1))

		rect := subImageWithShadow.Bounds()
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		brighterPixelsCount, err := screenshot.CountBrighterPixels(subImageWithShadow, subImageWithoutShadow)
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
func testCaptionHeight(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
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

	appWindow, err := ash.GetARCAppWindowInfo(ctx, tconn, pkg)
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
func testResizeWindow(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	dispMode, err := ash.InternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}
	appWindow, err := ash.GetARCAppWindowInfo(ctx, tconn, pkg)
	if err != nil {
		return errors.Wrap(err, "failed to get arc window info")
	}

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open touchscreen device")
	}
	defer tsw.Close()

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not create TouchEventWriter")
	}
	defer stw.Close()

	// Calculate Pixel (screen display) / Tuxel (touch device) ratio.
	dispW := dispMode.WidthInNativePixels
	dispH := dispMode.HeightInNativePixels
	pixelToTuxelX := float64(tsw.Width()) / float64(dispW)
	pixelToTuxelY := float64(tsw.Height()) / float64(dispH)

	captionHeight := int(math.Round(float64(appWindow.CaptionHeight) * dispMode.DeviceScaleFactor))
	bounds := ash.ConvertBoundsFromDpToPx(appWindow.BoundsInRoot, dispMode.DeviceScaleFactor)
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
		{startX: bounds.Left + innerMargin, startY: middleY, endX: 0, endY: middleY},                        //left
		{startX: bounds.Left + bounds.Width - innerMargin, startY: middleY, endX: dispW - 1, endY: middleY}, //right
		{startX: middleX, startY: bounds.Top + innerMargin + captionHeight, endX: middleX, endY: 0},         //top
		{startX: middleX, startY: bounds.Top + bounds.Height - innerMargin, endX: middleX, endY: dispH - 1}, //bottom
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
		if _, err := d.WaitForWindowUpdate(ctx, pkg, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait window updated after swipe resize")
		}
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		appWindow, err = ash.GetARCAppWindowInfo(ctx, tconn, pkg)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get arc window info"))
		}
		const epsilon = 2
		if !isSimilarRect(appWindow.BoundsInRoot, ash.Rect(*dispInfo.WorkArea), epsilon) {
			return errors.Errorf("resize window doesn't have the expected bounds yet; got %v, want %v", appWindow.BoundsInRoot, dispInfo.WorkArea)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}
	return nil
}

// testWorkspaceInsets verifies that the workspace insets info from ChromeOS companion library is correct.
func testWorkspaceInsets(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
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
				return errors.Wrap(err, "failed to get internal display info")
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
			return errors.Wrap(err, "could not change the system shelf alignment")
		}
		// Convert two rectangle to same unit.
		expectedShelfRectPX := ash.ConvertBoundsFromDpToPx(ash.Rect(expectedShelfRect), dispMode.DeviceScaleFactor)
		parsedShelfRectFromCallback, err := parseRectString(callbackMessage.WorkspaceInsetMsg.InsetBound, dispMode)
		if err != nil {
			return errors.Wrap(err, "failed to parse message")
		}
		const epsilon = 2
		if !isSimilarRect(expectedShelfRectPX, parsedShelfRectFromCallback, epsilon) {
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
		parsedShelfRect, err := parseRectString(msg.WorkspaceInsetMsg.InsetBound, dispMode)
		if err != nil {
			return errors.Wrap(err, "failed to parse message")
		}

		// Workspace insets infomation computed by window shelf info need several numeric conversion, which easy cause floating errors.
		if !isSimilarRect(expectedShelfRectPX, parsedShelfRect, epsilon) {
			return errors.Errorf("Workspace Inset is not expected: got %v, want %v", parsedShelfRect, expectedShelfRectPX)
		}
	}
	return nil
}

// testCaptionButton verifies that hidden caption button API works as expected.
func testCaptionButton(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
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
			return errors.New("Error while changing hidden caption button")
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			window, err := ash.GetARCAppWindowInfo(ctx, tconn, pkg)
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
		window, err := ash.GetARCAppWindowInfo(ctx, tconn, pkg)
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
func testDeviceMode(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const getDeviceModeButtonID = pkg + ":id/get_device_mode_button"

	if err := setWindowStateSync(ctx, act, arc.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to set window normal state before testing device mode change")
	}
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
				return testing.PollBreak(errors.Wrap(err, "failed to get json text"))
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
					return testing.PollBreak(errors.New("error on callback message generation"))
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

// testPopupWindow verifies that popup window's behaviors works as expected.
func testPopupWindow(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const (
		showPopupWindowButtonID = pkg + ":id/popup_window_button"
		clipToTaskCheckboxID    = pkg + ":id/clip_to_task_bounds"
		dismissButtonID         = pkg + ":id/dismiss"
		popupWindowString       = "Popup Window"
	)

	countPopupWindowPixelPercentage := func(captionImage image.Image) float64 {
		// https://developer.android.com/reference/android/R.color#holo_blue_light
		holoBlueLight := color.RGBA{0x33, 0xb5, 0xe5, 0xff}
		rect := captionImage.Bounds()
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		popupWindowPixelsCount := screenshot.CountPixels(captionImage, holoBlueLight)
		return float64(popupWindowPixelsCount) * 100.0 / float64(totalPixels)
	}

	dispMode, err := ash.InternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}

	// Set window on the top of the workspace insets. Make sure the framework can ignore the caption bar size of popup window layer. See b/147783396.
	setWindowBounds(ctx, d, arc.Rect{Left: 0, Top: 0, Width: 0, Height: dispMode.HeightInNativePixels})

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
func testWindowState(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const (
		setWindowStateButtonID = pkg + ":id/set_task_window_state_button"
		getWindowStateButtonID = pkg + ":id/get_task_window_state_button"
	)
	// TODO(sstan): Add testcase of "Always on top" setting
	for _, test := range []struct {
		windowStateStr string
		windowStateExp ash.WindowStateType
		isAppManaged   bool
	}{
		{windowStateStr: "Minimize", windowStateExp: ash.WindowStateMinimized, isAppManaged: false},
		{windowStateStr: "Maximize", windowStateExp: ash.WindowStateMaximized, isAppManaged: false},
		{windowStateStr: "Normal", windowStateExp: ash.WindowStateNormal, isAppManaged: false},
	} {
		testing.ContextLogf(ctx, "WindowState: Testing windowState=%v, appManaged=%t", test.windowStateStr, test.isAppManaged)
		// Change the window to normal state first, making sure the UI can be touched by tast test library.
		if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
			return err
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
			return err
		}

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
func testMaximize(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const (
		setCaptionButtonID                      = pkg + ":id/set_caption_buttons_visibility"
		checkCaptionButtonMaximizeAndRestoreBox = pkg + ":id/caption_button_maximize_and_restore"
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

	dispMode, err := ash.InternalDisplayMode(ctx, tconn)
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
	middleCaptionLoc := ash.Location{
		Y: int(float64(bounds.Top+5) / dispMode.DeviceScaleFactor),
		X: int(float64(bounds.Left+bounds.Width/2) / dispMode.DeviceScaleFactor),
	}
	if err := ash.MouseClick(ctx, tconn, middleCaptionLoc, ash.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click window caption the first time")
	}

	if err := testing.Sleep(ctx, doubleClickGap); err != nil {
		return errors.Wrap(err, "failed to wait for the gap between the double click")
	}
	if err := ash.MouseClick(ctx, tconn, middleCaptionLoc, ash.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click window caption the second time")
	}

	if err := act.WaitForResumed(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait until activity resumed")
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
func testWindowBounds(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	const getWindowBoundsButtonID = pkg + ":id/get_window_bounds_button"

	physicalDisplayDensity, err := act.DisplayDensity(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get physical display density")
	}

	dispMode, err := ash.InternalDisplayMode(ctx, tconn)
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

	shelfHeightPX := dispMode.HeightInNativePixels - int(math.Round(float64(dispInfo.WorkArea.Height)*dispMode.DeviceScaleFactor))

	// Change the window to normal state for make sure the bounds of window can be set.
	if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
		return err
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
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
		// Format of arc.Rect is {top, left, width, height}, and it's not input for SetBounds function.
		settingBound  arc.Rect
		expectedBound arc.Rect
	}{
		{"trigger min size limit", arc.Rect{Left: 0, Top: 0, Width: 0, Height: 0}, arc.Rect{Left: 0, Top: captionHeight, Width: minimizeSize, Height: minimizeSize}},
		{"trigger min size limit again", arc.Rect{Left: 0, Top: captionHeight / 2, Width: minimizeSize / 2, Height: minimizeSize / 2}, arc.Rect{Left: 0, Top: captionHeight, Width: minimizeSize, Height: minimizeSize}},
		{"fullscreen size", arc.Rect{Left: 0, Top: 0, Width: dispMode.WidthInNativePixels, Height: dispMode.HeightInNativePixels}, arc.Rect{Left: 0, Top: captionHeight, Width: dispMode.WidthInNativePixels, Height: dispMode.HeightInNativePixels - captionHeight - shelfHeightPX}}, // Auto maximize. It means the edge will not over the shelf
	} {
		// The expected window bound depends on setting window bound and can be
		// calculated directly, according to the window bound behavior.
		if err := setWindowBounds(ctx, d, test.settingBound); err != nil {
			return errors.Wrap(err, "failed to setting window bound")
		}

		bound, err := windowBounds(ctx, d)
		if err != nil {
			return errors.Wrap(err, "failed to get window bound from UI message")
		}
		// Because the conversion of DP to PX, we should be lenient with the epsilon.
		const epsilon = 2
		if !isSimilarRect(ash.Rect(bound), ash.Rect(test.expectedBound), epsilon) {
			return errors.Errorf("wrong window bound, set %v: got %v, want %v", test.settingBound, bound, test.expectedBound)
		}
	}
	return nil
}

// setWindowBounds uses CompanionLib Demo UI operation to setting the window bounds.
// Only works on the window which has Normal State.
func setWindowBounds(ctx context.Context, d *ui.Device, bound arc.Rect) error {
	const (
		setWindowBoundsButtonID = pkg + ":id/set_window_bounds_button"
		topNumberTextID         = pkg + ":id/top_number_text"
		bottomNumberTextID      = pkg + ":id/bottom_number_text"
		rightNumberTextID       = pkg + ":id/right_number_text"
		leftNumberTextID        = pkg + ":id/left_number_text"
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
func windowBounds(ctx context.Context, d *ui.Device) (arc.Rect, error) {
	const getWindowBoundsButtonID = pkg + ":id/get_window_bounds_button"

	parseBoundFromMsg := func(msg *companionLibMessage) (arc.Rect, error) {
		// Parse Rect short string to rectangle format with native pixel size.
		var left, top, right, bottom int
		if msg.WindowBoundMsg == nil {
			return arc.Rect{}, errors.New("not a window bound message")
		}
		if n, err := fmt.Sscanf(msg.WindowBoundMsg.WindowBound, "[%d,%d][%d,%d]", &left, &top, &right, &bottom); err != nil {
			return arc.Rect{}, errors.Wrap(err, "error on parse Rect text")
		} else if n != 4 {
			return arc.Rect{}, errors.Errorf("the format of Rect text is not valid: %q", msg.WindowBoundMsg.WindowBound)
		}
		return arc.Rect{Left: left, Top: top, Width: right - left, Height: bottom - top}, nil
	}

	lastMsg, err := getLastJSONMessage(ctx, d)
	if err != nil {
		return arc.Rect{}, errors.Wrap(err, "error on get last JSON message")
	}
	// Get window bound message in JSON format TextView.
	if err := d.Object(ui.ID(getWindowBoundsButtonID)).Click(ctx); err != nil {
		return arc.Rect{}, errors.Wrap(err, "failed to click get window bound button")
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
		return arc.Rect{}, errors.Wrap(err, "failed to get window bound")
	}
	return parseBoundFromMsg(msg)
}

// setWindowState uses CompanionLib Demo UI operation to set the window state.
// About app controlled, see go/arc++-support-library.
func setWindowState(ctx context.Context, d *ui.Device, windowStateStr string, isAppControlled bool) error {
	const setWindowStateButtonID = pkg + ":id/set_task_window_state_button"
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

// setWindowStateSync returns after the window state changed as expected.
func setWindowStateSync(ctx context.Context, act *arc.Activity, state arc.WindowState) error {
	if err := act.SetWindowState(ctx, state); err != nil {
		return errors.Wrap(err, "could not set window state to normal")
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

// getTextViewContent returns all text in status textview.
func getTextViewContent(ctx context.Context, d *ui.Device) ([]string, error) {
	const statusTextViewID = pkg + ":id/status_text_view"
	if err := d.Object(ui.ID(statusTextViewID)).WaitForExists(ctx, 5*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to wait status textview ready")
	}
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
	expr := fmt.Sprintf(
		`tast.promisify(chrome.wallpaper.setWallpaper)({
			url: '%s',
			layout: 'STRETCH',
			filename: 'test_wallpaper'
		})`, wallpaperURL)
	err := tconn.EvalPromise(ctx, expr, nil)
	return err
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
func isSimilarRect(lhs ash.Rect, rhs ash.Rect, epsilon int) bool {
	return abs(lhs.Left-rhs.Left) <= epsilon && abs(lhs.Width-rhs.Width) <= epsilon && abs(lhs.Top-rhs.Top) <= epsilon && abs(lhs.Height-rhs.Height) <= epsilon
}
