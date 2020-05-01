// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	// "bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	// "io/ioutil"
	"os"
	"path/filepath"
	// "strconv"
	// "strings"
	// "time"

	// "golang.org/x/sys/unix"


	"chromiumos/tast/local/bundles/cros/arc/appposition"
	"chromiumos/tast/local/screenshot"
	// "chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/chrome"
	// "chromiumos/tast/local/sysutil"
	// "chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const apk = "ArcAppPositionTest.apk"

func init() {
	testing.AddTest(&testing.Test{
		Func: AppPosition,
		Desc: "Checks that content is drawn at proper location for launched apps",
		Contacts: []string{
			"xutan@google.com", // original author.
			"arc-eng@google.com",
			"kimiyuki@google.org", // Tast port.
		},
		SoftwareDeps: []string{"android_p", "chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{apk},
		Pre:          arc.Booted(),
	})
}

// Resource ID for caption widgets. Only retrievable if they're still drawn on
// Android side.
const (
	captionBackButtonID = "android:id/decor_go_back_button"
	captionMinimizeButtonID = "android:id/decor_minimize_button"
	captionMaximizeButtonID = "android:id/decor_maximize_button"
	captionCloseButtonID = "android:id/decor_close_button"
)


// Hack for nocture. This is a temporal hack to fix the off-by-one bug in
// screenshots taken in Nocture: http://b/117897276
const noctureFix = 1

func getDeviceBounds(ctx context.Context, d *ui.Device) (coords.Rect, error) {
	deviceInfo, err := d.GetInfo(ctx)
	if err != nil {
		return coords.NewRect(0, 0, 0, 0), err
	}
	return coords.NewRect(0, 0, deviceInfo.DisplayWidth, deviceInfo.DisplayHeight), nil
}


/*
class AppPositionActivity(activity.Activity):
    """Wrapper of AppPosition test activity running on Chromebooks.

    Used to capture bounds for the activity, its content and its caption.
    """
    func __init__(self, device, package, activity):
        super(AppPositionActivity, self).__init__(device, package, activity)

        self._frame_selector = self._device(
            className='android.widget.FrameLayout', instance=0)
        self._back_button_selector = self._device(
            resourceId=_CAPTION_BACK_BUTTON_ID)
        self._minimize_button_selector = self._device(
            resourceId=_CAPTION_MINIMIZE_BUTTON_ID)
        self._maximize_button_selector = self._device(
            resourceId=_CAPTION_MAXIMIZE_BUTTON_ID)
        self._close_button_selector = self._device(
            resourceId=_CAPTION_CLOSE_BUTTON_ID)

        self._device_bounds = getDeviceBounds(self._device)

    func _start(self):
        super(AppPositionActivity, self)._start()
        self._caption_in_android = self._is_caption_in_android()
*/

func getBackButtonSelector(d *ui.Device, ac *arc.Activity) *ui.Object {
	return d.Object(ui.PackageName(ac.PackageName()), ui.ResourceID(captionBackButtonID))
}

func getCloseButtonSelector(d *ui.Device, ac *arc.Activity) *ui.Object {
	return d.Object(ui.PackageName(ac.PackageName()), ui.ResourceID(captionCloseButtonID))
}

func isCaptionInAndroid(ctx context.Context, d *ui.Device, ac *arc.Activity) bool {
	backButtonSelector := getBackButtonSelector(d, ac)
	closeButtonSelector := getCloseButtonSelector(d, ac)
	return backButtonSelector.Exists(ctx) == nil || closeButtonSelector.Exists(ctx) == nil
}

func getFrameBounds(ctx context.Context, d *ui.Device, ac *arc.Activity) (coords.Rect, error) {
	frameSelector := d.Object(ui.PackageName(ac.PackageName()),
		ui.ClassName("android.widget.FrameLayout"),
		ui.Instance(0))
	return frameSelector.GetBounds(ctx)
}

// Gets caption bounds.
//
// @returns accurate bounds if uiautomator can find caption; or a bounds
// 	of a pixel height above the frame.
func getCaptionBounds(ctx context.Context, d *ui.Device, ac *arc.Activity) (coords.Rect, error) {
	frameBounds, err := getFrameBounds(ctx, d, ac)
	if err != nil {
		return frameBounds, err
	}
	if ! isCaptionInAndroid(ctx, d, ac) {
		// Caption is in Chrome
		return coords.NewRectLTRB(frameBounds.Left + noctureFix,
			frameBounds.Top - 1,
			frameBounds.Left + frameBounds.Width, frameBounds.Top), nil
	}

	// Caption in Android
	backButtonBounds, err := getBackButtonSelector(d, ac).GetBounds(ctx)
	if err != nil {
		return backButtonBounds, err
	}
	return coords.NewRectLTRB(frameBounds.Left + noctureFix,
		backButtonBounds.Top,
		frameBounds.Left + frameBounds.Width, backButtonBounds.Top + backButtonBounds.Height), nil
}

// Gets a list of shadow bounds.
//
// Shadows are drawn around window. We can easily verify shadows on left,
// bottom and right hand side because we know for sure the boundary, but
// if caption is drawn in Chrome, we don't know the top bound of caption.
//
// It's enough to only check one pixel outward to the content.
//
// @return a list of bounds to check shadow colors
func getShadowBoundsList(ctx context.Context, d *ui.Device, ac *arc.Activity) ([]coords.Rect, error) {
	frameBounds, err := getFrameBounds(ctx, d, ac)
	if err != nil {
		return []coords.Rect{}, err
	}
	deviceBounds, err := getDeviceBounds(ctx, d)
	if err != nil {
		return []coords.Rect{}, err
	}

	ret := []coords.Rect{}
	// Left shadow
	if frameBounds.Left > deviceBounds.Left {
		ret = append(ret, coords.NewRectLTRB(frameBounds.Left - 1 - noctureFix,
		frameBounds.Top,
		frameBounds.Left - noctureFix,
		frameBounds.Top + frameBounds.Height))
	}

	// Right shadow
	if frameBounds.Left + frameBounds.Width < deviceBounds.Left + deviceBounds.Width {
		ret = append(ret, coords.NewRectLTRB(frameBounds.Left + frameBounds.Width + noctureFix,
		frameBounds.Top,
		frameBounds.Left + frameBounds.Width + 1 + noctureFix,
		frameBounds.Top + frameBounds.Height))
	}

	// Bottom shadow
	if frameBounds.Top + frameBounds.Height < deviceBounds.Top + deviceBounds.Height {
		ret = append(ret, coords.NewRectLTRB(frameBounds.Left, frameBounds.Top + frameBounds.Height,
		frameBounds.Left + frameBounds.Width, frameBounds.Top + frameBounds.Height + 1))
	}
	return ret, nil
}

// Gets bounds of the internal content.
func getContentBounds(ctx context.Context, d *ui.Device, ac *arc.Activity) (coords.Rect, error) {
	frameBounds, err := getFrameBounds(ctx, d, ac)
	if err != nil {
		return frameBounds, err
	}
        frameBounds.Left += noctureFix
        frameBounds.Width -= noctureFix
        if ! isCaptionInAndroid(ctx, d, ac) {
		return frameBounds, nil
	}

	backButtonBounds, err := getBackButtonSelector(d, ac).GetBounds(ctx)
	if err != nil {
		return backButtonBounds, err
	}
	contentBounds := frameBounds
        contentBounds.Top = backButtonBounds.Top + backButtonBounds.Height
        return contentBounds, nil
}

// The ratio between launch bounds and screen size
const (
	mainActivityLaunchBoundsLeft   float64 = 0.5
	mainActivityLaunchBoundsTop    float64 = 0.2
	mainActivityLaunchBoundsRight  float64 = 0.8
	mainActivityLaunchBoundsBottom float64 = 0.8
)

func verifyLaunchBounds(ctx context.Context, s *testing.State, d *ui.Device, ac *arc.Activity) {

	deviceBounds, err := getDeviceBounds(ctx, d)
	if err != nil {
		s.Fatal("Failed to get the device bounds: ", err)
	}
	launchBounds := coords.NewRect(
		int(float64(deviceBounds.Width)*mainActivityLaunchBoundsLeft),
		int(float64(deviceBounds.Height)*mainActivityLaunchBoundsTop),
		int(float64(deviceBounds.Width)*(mainActivityLaunchBoundsRight-mainActivityLaunchBoundsLeft)),
		int(float64(deviceBounds.Height)*(mainActivityLaunchBoundsBottom-mainActivityLaunchBoundsTop)))

	frameBounds, err := getFrameBounds(ctx, d, ac)
	if err != nil {
		s.Fatal("Failed to get the frame bounds: ", err)
	}
	if isCaptionInAndroid(ctx, d, ac) {
		// We can't assert that the top of frame bounds will be as expected
		// because it may be offset by the caption in Chrome.
		// TODO(b/79587124): Check if this is needed when caption is in Chrome.
		launchBounds.Top = frameBounds.Top
	}

	if launchBounds != frameBounds {
		s.Errorf("Unexpected launch bounds %s. Expecting: %s", frameBounds, launchBounds)
	}
}

func takeScreenshot(ctx context.Context, s *testing.State, cr *chrome.Chrome, d *ui.Device, ac *arc.Activity) (image.Image, error) {
	// Directories to dump information
	const screenshotDir = "/var/log/arc-screenshots/AppPositionTest"
	if err := os.MkdirAll(screenshotDir, os.ModePerm); err != nil {
		return nil, err
	}

	screenshotPath := filepath.Join(screenshotDir, fmt.Sprintf("%p.png", ac))
	if err := screenshot.CaptureChrome(ctx, cr, screenshotPath); err != nil {
		return nil, err
	}

	fh, err := os.Open(screenshotPath)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	screenshot, err := png.Decode(fh)
	if err != nil {
		return nil, err
	}
	return screenshot, nil
}

func testLaunchActivity(ctx context.Context, s *testing.State, cr *chrome.Chrome, d *ui.Device, ac *arc.Activity) {
	verifyLaunchBounds(ctx, s, d, ac)

	screenshot, err := takeScreenshot(ctx, s, cr, d, ac)
	if err != nil {
		s.Fatal("Failed to take a screenshot: ", err)
	}

	contentBounds, err := getContentBounds(ctx, d, ac)
	if err != nil {
		s.Error("Failed to get the content bounds: ", err)
	} else {
		appposition.VerifyContent(s, screenshot, contentBounds)
	}

	shadowBoundsList, err := getShadowBoundsList(ctx, d, ac)
	if err != nil {
		s.Error("Failed to get the content bounds: ", err)
	} else {
		appposition.VerifyShadow(s, screenshot, shadowBoundsList)
	}

	captionBounds, err := getCaptionBounds(ctx, d, ac)
	if err != nil {
		s.Error("Failed to get the caption bounds: ", err)
	} else {
		appposition.VerifyCaption(s, screenshot, captionBounds)
	}
}

// AppPosition test checks that Android framework and compositors all put activities at proper location.
func AppPosition(ctx context.Context, s *testing.State) {
	const pkg = "org.chromium.arc.testapp.appposition"
	const cls = ".BackgroundActivity"
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	ac, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer ac.Close()

	s.Log("Launching the activity")
	if err := ac.Start(ctx, tconn); err != nil {
		s.Fatal("Failed start activity: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to make the device: ", err)
	}
	defer d.Close()

	s.Log("Verifying bounds")
	testLaunchActivity(ctx, s, cr, d, ac)
}
