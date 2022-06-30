// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ActivityIndicators,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test that opened shelf apps have activity indicators",
		Contacts: []string{
			"mmourgos@chromium.org",
			"tbarzic@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Fixture:      "arcBooted",
	})
}

// ActivityIndicators verifies that shelf apps which are active have
// an activity indicator shown.
func ActivityIndicators(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Install a mock Android app.
	const apk = "ArcInstallAppWithAppListSortedTest.apk"
	a := s.FixtValue().(*arc.PreData).ARC
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing arc app: ", err)
	}

	appName := "InstallAppWithAppListSortedMockApp"
	installedArcAppID, err := ash.WaitForChromeAppByNameInstalled(ctx, tconn, appName, 1*time.Minute)
	if err != nil {
		s.Fatalf("Failed to wait until %s is installed: %v", appName, err)
	}
	s.Log(installedArcAppID)

	// Expect that 0 activity indicators are shown.
	if err = expectNumberOfActivityIndicators(ctx, tconn, 0); err != nil {
		s.Fatal("Failed to expect the number of activity indicators: ", err)
	}

	// Launch the files app and check that one indicator is shown.
	if err = apps.Launch(ctx, tconn, apps.Files.ID); err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}
	if err = ash.WaitForApp(ctx, tconn, apps.Files.ID, time.Minute); err != nil {
		s.Fatal("Files app did not appear in shelf after launch: ", err)
	}

	// Expect that 1 activity indicator is shown.
	if err = expectNumberOfActivityIndicators(ctx, tconn, 1); err != nil {
		s.Fatal("Failed to expect the number of activity indicators: ", err)
	}

	// Get the expected browser.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the Chrome app: ", err)
	}

	// Launch the browser app.
	if err = apps.Launch(ctx, tconn, chromeApp.ID); err != nil {
		s.Fatal("Failed to launch browser app: ", err)
	}
	if err = ash.WaitForApp(ctx, tconn, chromeApp.ID, time.Minute); err != nil {
		s.Fatal("Browser app did not appear in shelf after launch: ", err)
	}

	// Expect that 2 activity indicators are shown.
	if err = expectNumberOfActivityIndicators(ctx, tconn, 2); err != nil {
		s.Fatal("Failed to expect the number of activity indicators: ", err)
	}

	// Close the browser app.
	if err = apps.Close(ctx, tconn, chromeApp.ID); err != nil {
		s.Fatal("Failed to close the browser app: ", err)
	}
	if err := ash.WaitForAppClosed(ctx, tconn, chromeApp.ID); err != nil {
		s.Fatal("Failed to wait for browser app to close: ", err)
	}

	// Expect that 1 activity indicator is shown.
	if err = expectNumberOfActivityIndicators(ctx, tconn, 1); err != nil {
		s.Fatal("Failed to expect the number of activity indicators: ", err)
	}

	// Launch the arc app.
	if err = apps.Launch(ctx, tconn, installedArcAppID); err != nil {
		s.Fatal("Failed to launch the arc app: ", err)
	}
	if err = ash.WaitForApp(ctx, tconn, installedArcAppID, time.Minute); err != nil {
		s.Fatal("Arc app did not appear in shelf after launch: ", err)
	}

	// Expect that 2 activity indicators are shown.
	if err = expectNumberOfActivityIndicators(ctx, tconn, 2); err != nil {
		s.Fatal("Failed to expect the number of activity indicators: ", err)
	}

	// Close the arc app.
	if err = apps.Close(ctx, tconn, installedArcAppID); err != nil {
		s.Fatal("Failed to close the arc app: ", err)
	}
	if err := ash.WaitForAppClosed(ctx, tconn, installedArcAppID); err != nil {
		s.Fatal("Failed to wait for arc app to close: ", err)
	}

	// Expect that 1 activity indicators are shown.
	if err = expectNumberOfActivityIndicators(ctx, tconn, 1); err != nil {
		s.Fatal("Failed to expect the number of activity indicators: ", err)
	}

	// Close the Files app.
	if err = apps.Close(ctx, tconn, apps.Files.ID); err != nil {
		s.Fatal("Failed to close the Files app: ", err)
	}
	if err := ash.WaitForAppClosed(ctx, tconn, apps.Files.ID); err != nil {
		s.Fatal("Failed to wait for Files app to close: ", err)
	}

	// Expect that 0 activity indicators are shown.
	if err = expectNumberOfActivityIndicators(ctx, tconn, 0); err != nil {
		s.Fatal("Failed to expect the number of activity indicators: ", err)
	}
}

// expectNumberOfActivityIndicators checks the number of shelf app button indicator views that exist and compares that
// to expectedNumberOfIndicators. If the number of indicators is different than expected, then an error is thrown.
func expectNumberOfActivityIndicators(ctx context.Context, tconn *chrome.TestConn, expectedNumberOfIndicators int) error {
	ui := uiauto.New(tconn)

	activityIndicators, err := ui.NodesInfo(ctx, nodewith.ClassName("ShelfAppButton::AppStatusIndicatorView"))
	if err != nil {
		return errors.Wrap(err, "failed to find ShelfAppButton activity indicators")
	}
	numIndicators := len(activityIndicators)

	if err != nil {
		errors.Wrap(err, "failed to get number of activity indicators")
	}
	if numIndicators != expectedNumberOfIndicators {
		errors.Wrapf(err, "wrong number of activity indicators shown, got %d, want %d", numIndicators, expectedNumberOfIndicators)
	}
	return nil
}
