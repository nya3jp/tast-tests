// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	resizeLockTestPkgName = "org.chromium.arc.testapp.resizelock"

	resizeLockApkName = "ArcResizeLockTest.apk"
)

var testCases = []wm.TestCase{
	wm.TestCase{
		Name: "O4C Resizability",
		Func: testO4CResizability,
	},
	wm.TestCase{
		Name: "Resize Locked App Resizability",
		Func: testResizeLockedResizability,
	},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeLock,
		Desc:         "Checks that ARC++ Resize Lock works as expected",
		Contacts:     []string{"takise@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Val:               ResizeLock,
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ResizeLock(ctx context.Context, s *testing.State) {
	// Ensure to enable the finch flag.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--enable-features=ArcResizeLock"))

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Could not start ARC: ", err)
	}
	defer a.Close(ctx)

	// ResizeLock is only applied to newly-installed apps, so ensure to reinstall the apk.
	if installed, err := a.PackageInstalled(ctx, resizeLockTestPkgName); err != nil {
		if err != nil {
			s.Fatal("Failed to query installed state: ", err)
		} else if installed {
			if a.Uninstall(ctx, arc.APKPath(resizeLockApkName)); err != nil {
				s.Fatal("Failed to uninstall app: ", err)
			}
		}
	}

	s.Log("Installing ", resizeLockApkName)
	if err := a.Install(ctx, arc.APKPath(resizeLockApkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}
	defer a.Uninstall(ctx, arc.APKPath(resizeLockApkName))

	mainActivity, err := arc.NewActivity(a, resizeLockTestPkgName, ".MainActivity")
	if err != nil {
		s.Fatal("Failed to create main activity: ", err)
	}
	defer mainActivity.Close()

	dev, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer dev.Close(ctx)

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	origShelfAlignment, err := ash.GetShelfAlignment(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf alignment: ", err)
	}
	if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, ash.ShelfAlignmentBottom); err != nil {
		s.Fatal("Failed to set shelf alignment to Bottom: ", err)
	}
	// Be nice and restore shelf alignment to its original state on exit.
	defer ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, origShelfAlignment)

	origShelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Never Auto Hide: ", err)
	}
	// Be nice and restore shelf behavior to its original state on exit.
	defer ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, origShelfBehavior)

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	for _, test := range testCases {
		s.Logf("Running test %q", test.Name)

		if err := test.Func(ctx, tconn, a, dev); err != nil {
			path := fmt.Sprintf("%s/screenshot-resize-lock-failed-test-%s.png", s.OutDir(), test.Name)
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
			s.Errorf("%s test failed: %v", test.Name, err)
		}
	}
}

// testO4CResizability verifies that an O4C app is not resize locked even if it's newly-installed.
func testO4CResizability(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	if installed, err := a.PackageInstalled(ctx, wm.APKNameArcWMTestApp24); err != nil {
		if err != nil {
			return errors.Wrap(err, "failed to query installed state")
		} else if installed {
			if a.Uninstall(ctx, arc.APKPath(wm.APKNameArcWMTestApp24)); err != nil {
				return errors.Wrap(err, "failed to uninstall app")
			}
		}
	}

	if err := a.Install(ctx, arc.APKPath(wm.APKNameArcWMTestApp24)); err != nil {
		return errors.Wrap(err, "failed to install app")
	}
	defer a.Uninstall(ctx, arc.APKPath(wm.APKNameArcWMTestApp24))

	activity, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create resizable unspecified activity")
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start O4C activity")
	}
	defer activity.Stop(ctx, tconn)

	return testing.Poll(ctx, func(ctx context.Context) error {
		window, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return errors.New("failed to Get window of O4C app")
		}
		if !window.CanResize {
			return testing.PollBreak(errors.New("resizable O4C app should never be unresizable"))
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// testResizeLockedResizability verifies that an O4C app is not resize locked even if it's newly-installed.
func testResizeLockedResizability(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	activity, err := arc.NewActivity(a, resizeLockTestPkgName, "org.chromium.arc.testapp.resizelock.MainActivity")
	if err != nil {
		return errors.Wrap(err, "failed to create resizable unspecified activity")
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start the BlackFlashTest activity")
	}
	defer activity.Stop(ctx, tconn)

	return testing.Poll(ctx, func(ctx context.Context) error {

		window, err := ash.GetARCAppWindowInfo(ctx, tconn, resizeLockTestPkgName)
		if err != nil {
			return errors.New("failed to Get window of resize-locked app")
		}
		if window.CanResize {
			return testing.PollBreak(errors.New("resize-locked app shouldn't be resizable"))
			// return testing.PollBreak(errors.New("resize-locked app shouldn't be resizable"))
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
