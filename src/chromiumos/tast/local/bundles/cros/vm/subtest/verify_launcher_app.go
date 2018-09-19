// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"errors"
	"fmt"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// VerifyLauncherApp verifies that an installed application properly works with
// the Chrome launcher. It check that icons are present, it can be launched, renders
// when launched and has its shelf item appear as well. After that it closes the
// app with a keypress and verifies it has disappeared from the shelf.
func VerifyLauncherApp(s *testing.State, tconn *chrome.Conn, ownerId, appName, appId string,
	expectedColor screenshot.Color) {
	s.Logf("Verifying launcher integration for: %v", appName)
	ctx := s.Context()
	// There's a delay with apps being installed in Crostini and them appearing
	// in the launcher as well as having their icons loaded. The icons are only
	// loaded after they appear in the launcher, so if we check that first we know
	// it is in the launcher afterwards.
	s.Logf("Checking that app icons exist for: %v", appName)
	iconPath := filepath.Join("/home/user", ownerId, "crostini.icons", appId)
	err := testing.Poll(ctx, func(ctx context.Context) error {
		fileInfo, err := os.Stat(iconPath)
		if err != nil {
			return fmt.Errorf("Failed checking for existence of path: %v", err)
		}
		if !fileInfo.IsDir() {
			return errors.New("Icon path is not a directory")
		}
		entries, err := ioutil.ReadDir(iconPath)
		if err != nil {
			return fmt.Errorf("Failed getting directory listing of: %v", err)
		}
		if len(entries) == 0 {
			return errors.New("No icons existed in directory")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
	if err != nil {
		s.Errorf("Failed checking for icons in %v for %v of %v", iconPath, appName, err)
	}

	s.Logf("Launching application: %v", appName)
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
	if err = tconn.EvalPromise(ctx, expr, nil); err != nil {
		s.Errorf("Running autotestPrivate.launchApp failed for %v with %v", appName, err)
		return
	}

	s.Logf("Verifying screenshot for launch: %v", appName)
	screenshotName := "screenshot_launcher_" + appName + ".png"
	path := filepath.Join(s.OutDir(), screenshotName)

	// Largest differing color known to date, we will be changing this over time
	// based on testing results.
	const maxKnownColorDiff = 0x0100

	// Allow up to 10 seconds for the target screen to render.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		if err := screenshot.Capture(ctx, path); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			s.Fatal("Failed opening the screenshot image: ", err)
		}
		defer f.Close()
		im, err := png.Decode(f)
		if err != nil {
			s.Fatal("Failed decoding the screenshot image: ", err)
		}
		color, ratio := screenshot.DominantColor(im)
		if ratio >= 0.5 && screenshot.ColorsMatch(color, expectedColor, maxKnownColorDiff) {
			return nil
		} else {
			return fmt.Errorf("screenshot did not have matching dominant color, expected "+
				"%v but got %v at ratio %v", expectedColor, color, ratio)
		}
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	if err != nil {
		s.Errorf("Failure in screenshot comparison for %v from terminal: ", appName, err)
	}

	s.Logf("Checking shelf visibility: %v", appName)
	var appShown bool
	expr = fmt.Sprintf(
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
		s.Errorf("Running autotestPrivate.isAppShown failed for %v with %v", appName, err)
	} else if !appShown {
		s.Errorf("App was not shown in shelf: %v", appName)
	}

	// TODO(jkardatzke): Close the application with a keypress once we have that
	// capability in tast-tests. Then verify the app no longer exists in the shelf
	// after being closed.
}
