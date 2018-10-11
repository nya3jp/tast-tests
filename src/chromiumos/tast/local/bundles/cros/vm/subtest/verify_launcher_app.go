// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"fmt"
	"image/color"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// VerifyLauncherApp verifies that an installed application properly works with
// the Chrome launcher. It check that icons are present, it can be launched, renders
// when launched and has its shelf item appear as well. After that it closes the
// app with a keypress and verifies it has disappeared from the shelf.
func VerifyLauncherApp(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	tconn *chrome.Conn, ownerId, appName, appId string, expectedColor color.Color) {
	s.Log("Verifying launcher integration for ", appName)
	// There's a delay with apps being installed in Crostini and them appearing
	// in the launcher as well as having their icons loaded. The icons are only
	// loaded after they appear in the launcher, so if we check that first we know
	// it is in the launcher afterwards.
	s.Log("Checking that app icons exist for ", appName)
	checkIconExistence(ctx, s, ownerId, appName, appId)

	s.Log("Launching application ", appName)
	launchApplication(ctx, s, tconn, appName, appId)

	s.Log("Verifying screenshot after launching ", appName)
	verifyScreenshot(ctx, s, cr, appName, expectedColor)

	s.Log("Checking shelf visibility after launching ", appName)
	if !getShelfVisbility(ctx, s, tconn, appName, appId) {
		s.Errorf("App %v was not shown in shelf", appName)
	}

	s.Log("Closing %v with keypress", appName)
	if err := sendEnterKey(ctx, s); err != nil {
		// Device doesn't support a keyboard most likely, so don't check if the
		// shelf item went away.
		s.Log("Failed to send keypress; ignoring (no internal keyboard?): ", err)
		return
	}

	s.Log("Checking shelf visibility after closing ", appName)
	// This may not happen instantaneously, so poll for it.
	stillVisibleErr := errors.Errorf("app %v was visible in shelf after closing", appName)
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if getShelfVisbility(ctx, s, tconn, appName, appId) {
			return stillVisibleErr
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
	if err != nil {
		s.Error(err)
	}
}

// checkIconExistence verifies that the Crostini icon folder for the specified
// application exists in the filesystem and contains at least one file.
func checkIconExistence(ctx context.Context, s *testing.State, ownerId, appName, appId string) {
	iconDir := filepath.Join("/home/user", ownerId, "crostini.icons", appId)
	err := testing.Poll(ctx, func(ctx context.Context) error {
		fileInfo, err := os.Stat(iconDir)
		if err != nil {
			return err
		}
		if !fileInfo.IsDir() {
			return errors.Errorf("icon path %v is not a directory", iconDir)
		}
		entries, err := ioutil.ReadDir(iconDir)
		if err != nil {
			return errors.Wrapf(err, "failed reading dir %v", iconDir)
		}
		if len(entries) == 0 {
			return errors.Errorf("no icons exist in %v", iconDir)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
	if err != nil {
		s.Errorf("Failed checking %v icons in %v: %v", appName, iconDir, err)
	}
}

// launchApplication launches the specified application via an autotest API call.
func launchApplication(ctx context.Context, s *testing.State, tconn *chrome.Conn, appName, appId string) {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.launchApp('%v', () => {
				if (chrome.runtime.lastError === undefined) {
					resolve();
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, appId)
	if err := tconn.EvalPromise(ctx, expr, nil); err != nil {
		s.Errorf("Running autotestPrivate.launchApp failed for %v: %v", appName, err)
		return
	}
}

// verifyScreenshot takes a screenshot and then checks that the majority of the
// pixels in it match the passed in expected color.
func verifyScreenshot(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	appName string, expectedColor color.Color) {
	screenshotName := "screenshot_launcher_" + appName + ".png"
	path := filepath.Join(s.OutDir(), screenshotName)

	// Largest differing color known to date, we will be changing this over time
	// based on testing results.
	const maxKnownColorDiff = 0x1

	// Allow up to 10 seconds for the target screen to render.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			s.Fatalf("Failed opening the screenshot image %v: %v", path, err)
		}
		defer f.Close()
		im, err := png.Decode(f)
		if err != nil {
			s.Fatalf("Failed decoding the screenshot image %v: %v", path, err)
		}
		color, ratio := colorcmp.DominantColor(im)
		if ratio >= 0.5 && colorcmp.ColorsMatch(color, expectedColor, maxKnownColorDiff) {
			return nil
		}
		return errors.Errorf("screenshot did not have matching dominant color, expected %v but got %v at ratio %0.2f",
			colorcmp.ColorStr(expectedColor), colorcmp.ColorStr(color), ratio)
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	if err != nil {
		s.Errorf("Failure in screenshot comparison for %v from launcher: %v", appName, err)
	}
}

// getShelfVisbility makes an autotest API call to determine if the specified
// application has a shelf icon that is in the running state and returns true
// if so, false otherwise.
func getShelfVisbility(ctx context.Context, s *testing.State, tconn *chrome.Conn, appName, appId string) bool {
	var appShown bool
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
			chrome.autotestPrivate.isAppShown('%v', function(appShown) {
				if (chrome.runtime.lastError === undefined) {
					resolve(appShown);
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, appId)
	if err := tconn.EvalPromise(ctx, expr, &appShown); err != nil {
		s.Errorf("Running autotestPrivate.isAppShown failed for %v: %v", appName, err)
		return false
	}
	return appShown
}

// sendEnterKey simulates pressing and releasing the enter key on the keyboard.
func sendEnterKey(ctx context.Context, s *testing.State) error {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		// This can happen on devices that don't support a keyboard.
		return err
	}
	defer ew.Close()

	// TODO(derat): Replace all of this once the input package exposes friendly
	// methods for injecting sequences of events.
	if err := ew.Event(input.EV_KEY, input.KEY_ENTER, 1); err != nil {
		s.Fatal("Failed to write key down event: ", err)
	}
	if err := ew.Sync(); err != nil {
		s.Fatal("Failed to write key down sync:", err)
	}
	if err := ew.Event(input.EV_KEY, input.KEY_ENTER, 0); err != nil {
		s.Fatal("Failed to write key up event:", err)
	}
	if err := ew.Sync(); err != nil {
		s.Fatal("Failed to write key up sync:", err)
	}
	return nil
}
