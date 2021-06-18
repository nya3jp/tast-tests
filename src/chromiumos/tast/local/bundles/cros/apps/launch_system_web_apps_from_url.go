// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchSystemWebAppsFromURL,
		Desc: "Verifies that System Web Apps can launch through their URL",
		Contacts: []string{
			"chrome-apps-platform-rationalization@google.com",
			"benreich@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// LaunchSystemWebAppsFromURL tries to navigate to System Web Apps from their chrome:// URL.
func LaunchSystemWebAppsFromURL(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	systemWebApps, err := apps.ListSystemWebApps(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get list of SWAs: ", err)
	}

	// Open up an empty Chrome browser window.
	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to open a new renderer: ", err)
	}
	defer conn.Close()

	// Get a handle to the input keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	for _, app := range systemWebApps {
		chromeURL := app.PublisherID

		// Filter out apps with empty PublisherIDs / Chrome URLs e.g. Terminal.
		if chromeURL == "" {
			continue
		}

		s.Run(ctx, app.ShortName, func(ctx context.Context, s *testing.State) {
			s.Log("Navigating to ", chromeURL)
			if err := verifyAndLaunchSystemWebAppFromURL(ctx, cr, tconn, kb, s.OutDir(), app.Name, chromeURL); err != nil {
				s.Fatalf("Failed navigating to %q: %v", chromeURL, err)
			}
		})
	}
}

// verifyAndLaunchSystemWebAppFromURL types the URL into the Chrome omnibox and verifies the SWA page loads.
func verifyAndLaunchSystemWebAppFromURL(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, outDir, appName, appURL string) (retErr error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Take a screenshot of the display before closing the SWA window (if it exists).
	var conn *chrome.Conn
	defer func() {
		if retErr != nil {
			screenshotFile := filepath.Join(outDir, appName+"_failed.png")
			if err := screenshot.Capture(ctx, screenshotFile); err != nil {
				testing.ContextLog(ctx, "Failed to take screenshot: ", err)
			}
		}

		if conn != nil {
			conn.Close()
		}
	}()

	ui := uiauto.New(tconn)
	omniboxFinder := nodewith.Name("Address and search bar").Role(role.TextField)
	if err := ui.LeftClick(omniboxFinder)(ctxWithTimeout); err != nil {
		return errors.Wrap(err, "failed to click the omnibox")
	}

	if err := keyboard.Accel(ctxWithTimeout, "ctrl+a"); err != nil {
		return errors.Wrap(err, "failed pressing enter into chrome omnibox")
	}

	if err := keyboard.Type(ctxWithTimeout, appURL); err != nil {
		return errors.Wrap(err, "failed entering URL into chrome omnibox")
	}

	if err := keyboard.Accel(ctxWithTimeout, "Enter"); err != nil {
		return errors.Wrap(err, "failed pressing enter into chrome omnibox")
	}

	conn, err := cr.NewConnForTarget(ctxWithTimeout, chrome.MatchTargetURL(appURL))
	if err != nil {
		return errors.Wrap(err, "failed getting connection to new target")
	}

	if conn.WaitForExpr(ctxWithTimeout, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "failed waiting for URL to load")
	}

	if err := conn.Eval(ctxWithTimeout, "window.close()", nil); err != nil {
		return errors.Wrap(err, "failed closing the window")
	}

	return nil
}
