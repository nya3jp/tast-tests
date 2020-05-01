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

func isCaptionInAndroid(ctx context.Context, d *ui.Device, ac *arc.Activity) bool {
	backButtonSelector := d.Object(ui.PackageName(ac.PackageName()), ui.ResourceID(captionBackButtonID))
	closeButtonSelector := d.Object(ui.PackageName(ac.PackageName()), ui.ResourceID(captionCloseButtonID))
	return backButtonSelector.Exists(ctx) != nil || closeButtonSelector.Exists(ctx) != nil
}

func getFrameBounds(ctx context.Context, d *ui.Device, ac *arc.Activity) (coords.Rect, error) {
	frameSelector := d.Object(ui.PackageName(ac.PackageName()),
		ui.ClassName("android.widget.FrameLayout"),
		ui.Instance(0))
	return frameSelector.GetBounds(ctx)
}

/*
    func caption_bounds(self):
        """Gets caption bounds.

        @returns accurate bounds if uiautomator can find caption; or a bounds
                of a pixel height above the frame.
        """
        frame_bounds = self.frame_bounds()
        if not self.is_caption_in_android:
            # Caption is in Chrome
            return rect.Rect(frame_bounds.left + _NOCTURE_FIX,
                             frame_bounds.top - 1,
                             frame_bounds.right, frame_bounds.top)

        # Caption in Android
        back_button_bounds = self._back_button_selector.info['bounds']
        return rect.Rect(frame_bounds.left + _NOCTURE_FIX,
                         back_button_bounds['top'],
                         frame_bounds.right, back_button_bounds['bottom'])

    func shadow_bounds(self) {
        """Gets a list of shadow bounds.

        Shadows are drawn around window. We can easily verify shadows on left,
        bottom and right hand side because we know for sure the boundary, but
        if caption is drawn in Chrome, we don't know the top bound of caption.

        It's enough to only check one pixel outward to the content.

        @return a list of bounds to check shadow colors
        """
        frame_bounds = self.frame_bounds()

        ret = []
        # Left shadow
        if frame_bounds.left > self._device_bounds.left:
            ret.append(rect.Rect(frame_bounds.left - 1 - _NOCTURE_FIX,
                                 frame_bounds.top,
                                 frame_bounds.left - _NOCTURE_FIX,
                                 frame_bounds.bottom))

        # Right shadow
        if frame_bounds.right < self._device_bounds.right:
            ret.append(rect.Rect(frame_bounds.right + _NOCTURE_FIX,
                                 frame_bounds.top,
                                 frame_bounds.right + 1 + _NOCTURE_FIX,
                                 frame_bounds.bottom))

        # Bottom shadow
        if frame_bounds.bottom < self._device_bounds.bottom:
            ret.append(rect.Rect(frame_bounds.left, frame_bounds.bottom,
                                 frame_bounds.right, frame_bounds.bottom + 1))
        return ret
    }

// Gets bounds of the internal content.
func content_bounds(self) {
        frame_bounds = self.frame_bounds()
        frame_bounds.left += _NOCTURE_FIX
        if not self.is_caption_in_android:
            return frame_bounds

        back_button_bounds = self._back_button_selector.info['bounds']
        content_bounds = rect.Rect.from_rect(frame_bounds)
        content_bounds.top = back_button_bounds['bottom']
        return content_bounds
}

*/

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

	contentBounds := coords.NewRect(0, 0, 0, 0)
	appposition.VerifyContent(s, screenshot, contentBounds)

	shadowBounds := coords.NewRect(0, 0, 0, 0)
	appposition.VerifyShadow(s, screenshot, shadowBounds)

	captionBounds := coords.NewRect(0, 0, 0, 0)
	appposition.VerifyCaption(s, screenshot, captionBounds)
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
