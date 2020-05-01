// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	// "bytes"
	"context"
	// "fmt"
	// "io/ioutil"
	// "os"
	// "path/filepath"
	// "strconv"
	// "strings"
	// "time"

	// "golang.org/x/sys/unix"

	// "chromiumos/tast/local/screenshot"
	// "chromiumos/tast/errors"
	// "chromiumos/tast/local/coords"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	// "chromiumos/tast/local/sysutil"
	// "chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const apk = "ArcAppPositionTestApp.apk"

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

/*
// Directories to dump information when test fails
const _SCREENSHOT_DIR = '/var/log/arc-screenshots/cheets_AppPositionTest'
*/

// AppPosition test checks that Android framework and compositors all put activities at proper location.
func AppPosition(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to make the device: ", err)
	}
	defer d.Close()

	const pkg = "org.chromium.arc.testapp.appposition"
	const cls = ".BackgroundActivity"
	ac, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to make an activity: ", err)
	}
	defer ac.Close()

	verifyLaunchBounds(ctx, s, d, ac)

	// TODO
	// with _start_verify_screenshot(activity) as screenshot:
	//     verifier.verify_content(screenshot, activity.content_bounds())
	//     verifier.verify_shadow(screenshot, activity.shadow_bounds())
	//     verifier.verify_caption(screenshot, activity.caption_bounds())
}

/*

// Resource ID for caption widgets. Only retrievable if they're still drawn on
// Android side.
const (
	_CAPTION_BACK_BUTTON_ID = 'android:id/decor_go_back_button'
	_CAPTION_MINIMIZE_BUTTON_ID = 'android:id/decor_minimize_button'
	_CAPTION_MAXIMIZE_BUTTON_ID = 'android:id/decor_maximize_button'
	_CAPTION_CLOSE_BUTTON_ID = 'android:id/decor_close_button'
)


// Hack for nocture. This is a temporal hack to fix the off-by-one bug in
// screenshots taken in Nocture: http://b/117897276
const _NOCTURE_FIX = 1

func getDeviceBounds(device) {
   return rect.Rect(0, 0, device.info['displayWidth'],
                     device.info['displayHeight'])
}


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

    func _is_caption_in_android(self):
        return (self._back_button_selector.exists
                or self._close_button_selector.exists)

    func frame_bounds(self):
        """Gets the bounds of this activity.

        This bounds include caption if it's drawn on Android side.

        @return the bounds of the activity.
        """
        return rect.Rect.from_rect(self._frame_selector.info['bounds'])

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

func is_caption_in_android(self) {
       return self._caption_in_android
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
	const pkg = "org.chromium.arc.testapp.appposition"

	/*
		display_size, err := ac.disp.Size()
		if err != nil {
			s.Fatal("Failed to get the size of the display of the activity:", err)
		}
		device_bounds := coords.NewRect(0, 0, display_size.Width, display_size.Height)
		launch_bounds := coords.NewRect(
			int(float64(device_bounds.Width) * MAIN_ACTIVITY_LAUNCH_BOUNDS_LEFT),
			int(float64(device_bounds.Height) * MAIN_ACTIVITY_LAUNCH_BOUNDS_TOP),
			int(float64(device_bounds.Width) * (MAIN_ACTIVITY_LAUNCH_BOUNDS_RIGHT - MAIN_ACTIVITY_LAUNCH_BOUNDS_LEFT)),
			int(float64(device_bounds.Height) * (MAIN_ACTIVITY_LAUNCH_BOUNDS_BOTTOM - MAIN_ACTIVITY_LAUNCH_BOUNDS_TOP)))
	*/
	launchBounds, err := ac.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get the size of the window of the activity: ", err)
	}

	frameSelector := d.Object(ui.PackageName(ac.PackageName()),
		ui.ClassName("android.widget.FrameLayout"),
		ui.Instance(0))
	frameBounds, err := frameSelector.GetBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get the bounds of the frame: ", err)
	}
	/*
		if ac.is_caption_in_android {
			// We can't assert that the top of frame bounds will be as expected
			// because it may be offset by the caption in Chrome.
			// TODO(b/79587124): Check if this is needed when caption is in Chrome.
			launchBounds.top = frameBounds.top
		}
	*/

	if launchBounds != frameBounds {
		s.Fatalf("Unexpected launch bounds %s. Expecting: %s", frameBounds, launchBounds)
	}
}

/*
@contextlib.contextmanager
func _start_verify_screenshot(activity) {
    image = screenshot.capture_screenshot()
    try:
        yield image
    except Exception as e:
        if not os.path.exists(_SCREENSHOT_DIR) {
            os.makedirs(_SCREENSHOT_DIR)
        }
        screenshot_path = os.path.join(_SCREENSHOT_DIR, '%s' % activity) + '.png'
        image.save(screenshot_path)
        raise e
}
*/
