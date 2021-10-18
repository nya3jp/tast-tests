// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/apps/webstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleMapBasicCheck,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Webstore app google map basic functionality check",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromeLoggedIn",
		Timeout:      5 * time.Minute,
	})
}

func GoogleMapBasicCheck(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var (
		urlAppName = "google-maps-platform-api"
		urlAppID   = "mlikepnkghhlnkgeejmlkfeheihlehne"
		appURL     = fmt.Sprintf("https://chrome.google.com/webstore/detail/%s/%s", urlAppName, urlAppID)
		startPoint = "Bengaluru"
		endPoint   = "Goa"
		appName    = "Google Maps Platform API Checker"
	)

	const (
		iter           = 5
		imageThreshold = 10
	)

	// App Install parameters
	app := webstore.App{Name: appName,
		URL:           appURL,
		VerifyText:    "Remove from Chrome",
		AddRemoveText: "Add to Chrome",
		ConfirmText:   "Add extension",
	}

	s.Logf("Installing %q app", appName)
	if err := webstore.UpgradeWebstoreApp(ctx, cr, tconn, app); err != nil {
		s.Fatal("Failed to install webapp: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		// App Uninstall parameters
		app = webstore.App{Name: appName,
			URL:           appURL,
			VerifyText:    "Add to Chrome",
			AddRemoveText: "Remove from Chrome",
			ConfirmText:   "Remove",
		}

		s.Logf("Uninstalling %q app", appName)
		if err := webstore.UpgradeWebstoreApp(ctx, cr, tconn, app); err != nil {
			s.Fatal("Failed to uninstall webapp: ", err)
		}
	}(cleanupCtx)

	googleMapURL := "https://www.google.com/maps"
	conn, err := cr.NewConn(ctx, googleMapURL)
	if err != nil {
		s.Fatal("Failed to open google map: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)

	// Google map zoom-in and zoom-out.
	s.Log("Zooming In")
	for i := 0; i <= iter; i++ {
		zoomIn := "document.getElementById('widget-zoom-in').click()"
		if err := mapFunctionalities(ctx, conn, zoomIn); err != nil {
			s.Fatal("Failed to zoom in: ", err)
		}
	}

	s.Log("Zooming out")
	for i := 0; i <= iter; i++ {
		zoomOut := "document.getElementById('widget-zoom-out').click()"
		if err := mapFunctionalities(ctx, conn, zoomOut); err != nil {
			s.Fatal("Failed to zoom out: ", err)
		}
	}

	earthScreenshot := filepath.Join(s.OutDir(), "earth.png")
	mapScreenshot := filepath.Join(s.OutDir(), "map.png")

	screenshot.Capture(ctx, mapScreenshot)

	// Switching between earth view and map view.
	s.Log("Switching to Earth view")
	const earthMapSwitch = "document.querySelectorAll('[aria-labelledby=widget-minimap-icon-overlay]')[0].click()"
	if err := mapFunctionalities(ctx, conn, earthMapSwitch); err != nil {
		s.Fatal("Failed to switch to earth view: ", err)
	}

	const showImagery = `document.querySelector('[aria-label="Show imagery"]').textContent`
	if err := mapFunctionalities(ctx, conn, showImagery); err != nil {
		s.Fatal("Failed to get show imagery element: ", err)
	}

	screenshot.Capture(ctx, earthScreenshot)

	s.Log("Switching to Map view")
	if err := mapFunctionalities(ctx, conn, earthMapSwitch); err != nil {
		s.Fatal("Failed to switch to map view: ", err)
	}

	if err := mapFunctionalities(ctx, conn, showImagery); err != nil {
		s.Fatal("Failed to get show imagery element: ", err)
	}

	screenshot.Capture(ctx, mapScreenshot)

	// Make sure screenshot comparison after switching between map view
	// and earth view, are sufficiently different.
	stdout, _ := testexec.CommandContext(ctx, "perceptualdiff", "-verbose", "-threshold", "1", mapScreenshot, earthScreenshot).Output(testexec.DumpLogOnError)
	var perceptualDiffRe = regexp.MustCompile((`(\d+) pixels are different`))
	if !perceptualDiffRe.MatchString(string(stdout)) {
		s.Fatalf("Failed: images %q and %q are perceptually identical", mapScreenshot, earthScreenshot)
	}

	// Performing find directions using Google Map.
	directions := `document.querySelector('[aria-label="Directions"]').click()`
	if err := mapFunctionalities(ctx, conn, directions); err != nil {
		s.Fatal("Failed to click on searchbox direction button: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard eventwriter: ", err)
	}

	if err := enterString(ctx, kb, startPoint); err != nil {
		s.Fatal("Failed to enter source point in directions searchbox: ", err)
	}

	if err := enterString(ctx, kb, endPoint); err != nil {
		s.Fatal("Failed to enter destination point in directions searchbox: ", err)
	}

	// Closing map directions searchbox
	closeDirection := `document.querySelector('[aria-label="Close directions"]').click()`
	if err := mapFunctionalities(ctx, conn, closeDirection); err != nil {
		s.Fatal("Failed to close searchbox direction: ", err)
	}
}

// mapFunctionalities will evaluates given jsExpr.
func mapFunctionalities(ctx context.Context, conn *chrome.Conn, jsExpr string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, jsExpr, nil); err != nil {
			return errors.Wrap(err, "failed to get element")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 10 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to find jsExpr in given timeout")
	}
	return nil
}

// enterString will type textToType using onboard keyboard.
func enterString(ctx context.Context, kb *input.KeyboardEventWriter, textToType string) error {
	testing.ContextLogf(ctx, "Typing %q", textToType)
	if err := kb.Type(ctx, textToType); err != nil {
		return errors.Wrapf(err, "failed to type %q", textToType)
	}
	if err := kb.Accel(ctx, "enter"); err != nil {
		return errors.Wrap(err, "failed to press enter key")
	}
	return nil
}
