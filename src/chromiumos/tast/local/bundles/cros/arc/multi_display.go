// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	// FirstExternalDisplayID represents the display ID for the first external display.
	firstExternalDisplayID = "1"

	// Apk compiled against target SDK 24 (N)
	wmPkgMD = "org.chromium.arc.testapp.windowmanager24"
	wmApkMD = "ArcWMTestApp_24.apk"

	// Different activities used by the subtests.
	nonResizeableUnspecifiedActivityMD = "org.chromium.arc.testapp.windowmanager.NonResizeableUnspecifiedActivity"
	resizeableUnspecifiedActivityMD    = "org.chromium.arc.testapp.windowmanager.ResizeableUnspecifiedActivity"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MultiDisplay,
		Desc:     "Mutli-display ARC window management tests",
		Contacts: []string{"ruanc@chromium.org", "niwa@chromium.org", "arc-framework+tast@google.com"},
		// TODO(ruanc): There is no hardware dependency for multi-display. Remove "disabled" attribute once it is supported.
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Data:         []string{"ArcWMTestApp_24.apk"},
		Pre:          arc.Booted(),
	})
}

// MultiDisplay test requires two connected displays.
func MultiDisplay(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	displayInfos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}

	// TODO(ruanc): This part can be removed if hardware dependency for multi-display is available.
	if len(displayInfos) < 2 {
		s.Fatalf("Not enough connected displays: got %d; want >= 2", len(displayInfos))
	}

	if err := a.Install(ctx, s.DataPath(wmApkMD)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		// Be nice and restore tablet mode to its original state on exit.
		defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)
		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to set tablet mode disabled: ", err)
		}
		// TODO(crbug.com/1002958): Wait for "tablet mode animation is finished" in a reliable way.
		// If an activity is launched while the tablet mode animation is active, the activity
		// will be launched in un undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
		}
	}

	type testFunc func(context.Context, *chrome.Chrome, *arc.ARC) error
	for idx, test := range []struct {
		name string
		fn   testFunc
	}{
		// Based on http://b/129564108
		{"Launch activity on external display", launchActivityOnExternalDisplay},
	} {
		s.Logf("Running test %q", test.name)

		// Log test resut
		if err := test.fn(ctx, cr, a); err != nil {
			for _, info := range displayInfos {
				path := fmt.Sprintf("%s/screenshot-multi-display-failed-test-%d-%q.png", s.OutDir(), idx, info.ID)
				if err := screenshot.CaptureChromeForDisplay(ctx, cr, info.ID, path); err != nil {
					s.Logf("Failed to capture screenshot for display ID %q: %v", info.ID, err)
				}
			}
			s.Errorf("%q test failed: %v", test.name, err)
		}
	}
}

// launchActivityOnExternalDisplay launches the activity directly on the external display.
func launchActivityOnExternalDisplay(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return err
	}
	var externalDisplayID string
	for _, info := range infos {
		if !info.IsInternal {
			externalDisplayID = info.ID
		}
	}

	for _, test := range []struct {
		name    string
		actName string
	}{
		{"Launch resizeable activity on the external display", resizeableUnspecifiedActivityMD},
		{"Launch unresizeable activity on the external display", nonResizeableUnspecifiedActivityMD},
	} {
		if err := func() error {
			testing.ContextLogf(ctx, "Running subtest %q", test.name)
			act, err := arc.NewActivity(a, wmPkgMD, test.actName)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := startActivityOnDisplay(ctx, a, wmPkgMD, test.actName, firstExternalDisplayID); err != nil {
				return err
			}
			defer act.Stop(ctx)

			if err := act.WaitForIdle(ctx, 10*time.Second); err != nil {
				return err
			}
			return ensureWindowOnDisplay(ctx, tconn, wmPkgMD, externalDisplayID)
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
		}
	}
	return nil
}

// ensureWindowOnDisplay checks wheater a window is on a certain display.
func ensureWindowOnDisplay(ctx context.Context, tconn *chrome.Conn, pkgName, dispID string) error {
	windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
	if err != nil {
		return err
	}
	if windowInfo.DisplayID != dispID {
		return errors.Errorf("invalid display ID: got %q; want %q", windowInfo.DisplayID, dispID)
	}
	return nil
}

// startActivityOnDisplay starts an activity by calling "am start --display" on the given display ID.
// TODO(ruanc): Move this function to proper location (activity.go or Ash) once the external displays has better support.
func startActivityOnDisplay(ctx context.Context, a *arc.ARC, pkgName, actName, dispID string) error {
	cmd := a.Command(ctx, "am", "start", "--display", dispID, pkgName+"/"+actName)
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to start activity")
	}
	// "adb shell" doesn't distinguish between a failed/successful run. For that we have to parse the output.
	// Looking for:
	//  Starting: Intent { act=android.intent.action.MAIN cat=[android.intent.category.LAUNCHER] cmp=com.example.name/.ActvityName }
	//  Error type 3
	//  Error: Activity class {com.example.name/com.example.name.ActvityName} does not exist.
	re := regexp.MustCompile(`(?m)^Error:\s*(.*)$`)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) == 2 {
		testing.ContextLog(ctx, "Failed to start activity: ", groups[1])
		return errors.New("failed to start activity")
	}
	return nil
}
