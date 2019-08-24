// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     HWOverlayTablet,
		Desc:     "Checks that hardware overlay works with ARC applications in tablet mode",
		Contacts: []string{"takise@chromium.org", "arc-framework+tast@google.com"},
		// TODO(ricardoq): enable test once the bug that fixes hardware overlay gets fixed. See: http://b/120557146
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"drm_atomic", "tablet_mode", "android_p", "chrome"},
		Pre:          arc.Booted(),
	})
}

func HWOverlayTablet(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get a Test API connection: ", err)
	}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	// TODO(ricardoq): Add clamshell mode tests.
	// Force Chrome to be in tablet mode.
	if err := ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to disable tablet mode: ", err)
	}

	a := s.PreValue().(arc.PreData).ARC

	// Any ARC++ activity could be used for this test. Using one that is already installed.
	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create Settings activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start Settings activity: ", err)
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	stateFile, err := driStateFile()
	// Should not fail, since it is guaranteed by "hw_overlay" SoftwareDeps.
	if err != nil {
		s.Fatal("Hardware overlay not supported. Perhaps 'hw_overlay' USE property added to the incorrect board: ", err)
	}

	// Leave Chromebook in reasonable state.
	rot0 := 0
	p := display.DisplayProperties{Rotation: &rot0}
	defer display.SetDisplayProperties(ctx, tconn, dispInfo.ID, p)

	// These are the only 4 valid rotation values.
	for _, rot := range []int{0, 90, 180, 270} {
		s.Logf("Setting display rotation to %d", rot)

		p = display.DisplayProperties{Rotation: &rot}
		if err := display.SetDisplayProperties(ctx, tconn, dispInfo.ID, p); err != nil {
			s.Error("Failed to set rotation: ", err)
		}

		// While rotating, HW overlay is disabled. It might take a few seconds to get active again.
		err := testing.Poll(ctx, func(ctx context.Context) error {
			return verifyHWOverlay(ctx, act, stateFile)
		}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second})
		if err != nil {
			s.Errorf("verifyHWOverlay failed for rotation %d: %s", rot, err)
		}
	}
}

// verifyHWOverlay verifies that hardware overlay is being used by comparing the surface size with the
// different CRTC buffer sizes. If there is a match, it means that HW overlays is being used for ARC++ applications.
// See https://crbug.com/932778 for alternative ways to verify whether HW overlays is being used.
// path is the full path to the DRI state file.
func verifyHWOverlay(ctx context.Context, a *arc.Activity, path string) error {
	bounds, err := a.SurfaceBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get activity surface bounds")
	}
	w := bounds.Width
	h := bounds.Height

	// Might be possible that while rotating, the surface bounds contains invalid values.
	if w <= 0 || h <= 0 {
		return errors.Errorf("invalid surface size: %dx%d", w, h)
	}

	dat, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	// A typical CRTC entry looks like the following:
	// plane[27]: plane 1A
	//     crtc=(null)
	//     fb=0
	//     crtc-pos=3000x2000+0+0
	//     src-pos=3000.000000x2000.000000+0.000000+0.000000
	//     rotation=1

	// A non-zero value in crtc-pos indicates that a CRTC buffer is being used.
	// The format of crtc-pos is size + offset. If there is a CRTC buffer with the same size as the
	// activity frame, we confirm that HW overlay is being used for ARC++.
	regStr := `(?m)` + // Enable multiline.
		`crtc-pos=(\d+)x(\d+)\+(\d+)\+(\d+)` // Match CRTC size + offset

	re := regexp.MustCompile(regStr)
	for _, groups := range re.FindAllStringSubmatch(string(dat), -1) {
		// For the comparison, CRTC buffer size is good enough. Ignoring the CRTC offset.
		crtcW, err := strconv.Atoi(groups[1])
		if err != nil {
			return errors.Wrap(err, "could not parse CRTC width")
		}
		crtcH, err := strconv.Atoi(groups[2])
		if err != nil {
			return errors.Wrap(err, "could not parse CRTC height")
		}

		// CRTC size is always in native size (non-rotated). Frame size could be rotated or not.
		if (crtcW == w && crtcH == h) || (crtcW == h && crtcH == w) {
			return nil
		}
	}
	return errors.Errorf("could not find CRTC buffer of size: %d,%d", w, h)
}

// driStateFile returns the path to the DRI state file, which is the file that contains the hardware overlay information.
// Returns error if file cannot be found.
func driStateFile() (driFile string, err error) {
	// The 'state' file is the one that has the HW overlay state. Depending on the device, it
	// could be either in the .../dri/0/ or .../dri/1/ directories.
	driStateFiles := []string{"/sys/kernel/debug/dri/0/state", "/sys/kernel/debug/dri/1/state"}
	for _, p := range driStateFiles {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("dri state file not found")
}
