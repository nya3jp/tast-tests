// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"fmt"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// AppDisplayDensityThroughLauncher launches the X11 or Wayland demo app twice through the Chrome OS Launcher calls
// after setting the display density to high denstiy and low density respectively and measures the sizes of the windows
// and verifies that low density windows are no smaller than the high density ones.
func AppDisplayDensityThroughLauncher(ctx context.Context, s *testing.State, tconn *chrome.Conn,
	ownerID, appName, appID string) {
	sizeHighDensity, err := launchAppAndMeasureWindowSize(ctx, s, tconn, ownerID, appName, appID, false)
	if err != nil {
		s.Errorf("Failed getting window %q size: %v", appName, err)
		return
	}
	s.Logf("window %q size is %v when scaled is false", appName, sizeHighDensity)
	sizeLowDensity, err := launchAppAndMeasureWindowSize(ctx, s, tconn, ownerID, appName, appID, true)
	if err != nil {
		s.Errorf("Failed getting window %q size: %v", appName, err)
		return
	}
	s.Logf("window %q size is %v when scaled is true", appName, sizeLowDensity)

	if sizeHighDensity.W > sizeLowDensity.W || sizeHighDensity.H > sizeLowDensity.H {
		s.Errorf("App %q has high density size %v greater than low density size %v", appName, sizeHighDensity, sizeLowDensity)
		return
	}

	factor, err := getPrimaryDisplayScaleFactor(ctx, tconn)
	if err != nil {
		s.Error("Failed getting primary display scale factor: ", err)
		return
	}
	s.Log("Primary display scale factor is ", factor)
	if math.Abs(factor-1.0) > 0.000001 && (sizeHighDensity.W == sizeLowDensity.W || sizeHighDensity.H == sizeLowDensity.H) {
		s.Errorf("App %q has high density and low density windows with the same size of %v while the scale factor is %v", appName, sizeHighDensity, factor)
		return
	}
}

func launchAppAndMeasureWindowSize(ctx context.Context, s *testing.State, tconn *chrome.Conn,
	ownerID, appName, appID string, scaled bool) (sz size, err error) {
	s.Log("Verifying launcher integration for ", appName)
	// There's a delay with apps being installed in Crostini and them appearing
	// in the launcher as well as having their icons loaded. The icons are only
	// loaded after they appear in the launcher, so if we check that first we know
	// it is in the launcher afterwards.
	s.Log("Checking that app icons exist for ", appName)
	checkIconExistence(ctx, s, ownerID, appName, appID)

	s.Logf("Setting application %q property scaled to %v", appName, scaled)
	setAppScaled(ctx, s, tconn, appName, appID, scaled)

	s.Log("Launching application ", appName)
	launchApplication(ctx, s, tconn, appName, appID)

	s.Log("Getting app window size after launching ", appName)
	sz, err = getWindowSizeWithPoll(ctx, tconn, appName)
	if err != nil {
		s.Errorf("Failed getting window %q size: %v", appName, err)
		return sz, err
	}
	s.Logf("window %q size is %v", appName, sz)

	s.Log("Checking shelf visibility after launching ", appName)
	if !getShelfVisibility(ctx, s, tconn, appName, appID) {
		s.Errorf("App %v was not shown in shelf", appName)
	}

	s.Logf("Closing %v with keypress", appName)
	ew, err := input.Keyboard(ctx)
	if err != nil {
		// Device doesn't have an internal keyboard most likely, so don't check if the
		// shelf item went away.
		s.Log("Failed to find keyboard device; ignoring: ", err)
		return sz, err
	}
	defer ew.Close()

	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}

	s.Log("Checking shelf visibility after closing ", appName)
	// This may not happen instantaneously, so poll for it.
	stillVisibleErr := errors.Errorf("app %v was visible in shelf after closing", appName)
	err = testing.Poll(ctx, func(ctx context.Context) error {
		if getShelfVisibility(ctx, s, tconn, appName, appID) {
			return stillVisibleErr
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
	if err != nil {
		s.Error("Failed to get shelf visibility: ", err)
		return sz, err
	}
	return sz, nil
}

// setAppScaled sets the specified application to be scaled or not via an autotest API call.
func setAppScaled(ctx context.Context, s *testing.State, tconn *chrome.Conn, appName, appID string, scaled bool) {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.setCrostiniAppScaled(%q, %v, () => {
				if (chrome.runtime.lastError === undefined) {
					resolve();
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, appID, scaled)
	if err := tconn.EvalPromise(ctx, expr, nil); err != nil {
		s.Errorf("Running autotestPrivate.setCrostiniAppScaled failed for %q %v: %v", appName, scaled, err)
		return
	}
}
