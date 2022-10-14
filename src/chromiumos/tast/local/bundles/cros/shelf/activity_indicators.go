// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type activityIndicatorAppType string

const (
	chromeApp activityIndicatorAppType = "ChromeApp"
	pwaApp    activityIndicatorAppType = "pwaApp"
	arcApp    activityIndicatorAppType = "arcApp"
)

type activityIndicatorTestParam struct {
	testAppType activityIndicatorAppType
	bt          browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ActivityIndicators,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test that opens shelf apps and checks each app's activity indicators",
		Contacts: []string{
			"mmourgos@chromium.org",
			"tbarzic@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Data:         []string{"web_app_install_force_list_index.html", "web_app_install_force_list_manifest.json", "web_app_install_force_list_service-worker.js", "web_app_install_force_list_icon-192x192.png", "web_app_install_force_list_icon-512x512.png"},
		Params: []testing.Param{{
			Name: "chrome_app",
			Val:  activityIndicatorTestParam{chromeApp, browser.TypeAsh},
		}, {
			Name:    "pwa_app",
			Val:     activityIndicatorTestParam{pwaApp, browser.TypeAsh},
			Fixture: fixture.ChromePolicyLoggedIn,
		}, {
			Name:    "arc_app",
			Val:     activityIndicatorTestParam{arcApp, browser.TypeAsh},
			Fixture: "arcBooted",
		}, {
			Name:              "chrome_app_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               activityIndicatorTestParam{chromeApp, browser.TypeLacros},
		}, {
			Name:              "pwa_app_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               activityIndicatorTestParam{pwaApp, browser.TypeLacros},
			Fixture:           fixture.LacrosPolicyLoggedInWithKeepAlive,
		}, {
			Name:              "arc_app_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               activityIndicatorTestParam{arcApp, browser.TypeLacros},
			Fixture:           "lacrosWithArcBooted",
		}},
	})
}

// ActivityIndicators verifies that shelf apps which are active have an activity indicator shown.
// Tests activity indicators for chrome browser, pwa, and arc apps.
func ActivityIndicators(ctx context.Context, s *testing.State) {
	testAppType := s.Param().(activityIndicatorTestParam).testAppType
	bt := s.Param().(activityIndicatorTestParam).bt

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var cr *chrome.Chrome
	switch testAppType {
	case chromeApp:
		var err error
		cr, err = browserfixt.NewChrome(ctx, bt, lacrosfixt.NewConfig())
		if err != nil {
			s.Fatalf("Failed to start %v browser: %v", bt, err)
		}
		defer cr.Close(cleanupCtx)
	case pwaApp:
		cr = s.FixtValue().(chrome.HasChrome).Chrome()
	case arcApp:
		cr = s.FixtValue().(*arc.PreData).Chrome
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	var appIDToLaunch string

	// Install the paramaterized app type and get the appIDToLaunch
	switch testAppType {
	case chromeApp:
		// Get the expected browser.
		browserApp, err := apps.PrimaryBrowser(ctx, tconn)
		if err != nil {
			s.Fatal("Could not find the browser app info: ", err)
		}
		appIDToLaunch = browserApp.ID
	case pwaApp:
		fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
		var cleanUp func(ctx context.Context) error
		pwaAppID, _, cleanUp, err := policyutil.InstallPwaAppByPolicy(ctx, tconn, cr, fdms, s.DataFileSystem())
		if err != nil {
			s.Fatal("Failed to install PWA: ", err)
		}
		appIDToLaunch = pwaAppID
		defer cleanUp(cleanupCtx)
	case arcApp:
		const apk = "ArcInstallAppWithAppListSortedTest.apk"
		a := s.FixtValue().(*arc.PreData).ARC
		if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
			s.Fatal("Failed installing arc app: ", err)
		}

		appName := "InstallAppWithAppListSortedMockApp"
		appIDToLaunch, err = ash.WaitForChromeAppByNameInstalled(ctx, tconn, appName, 1*time.Minute)
		if err != nil {
			s.Fatalf("Failed to wait until %s is installed: %v", appName, err)
		}
	}

	// Expect that 0 activity indicators are shown.
	numIndicators, err := numberOfActivityIndicators(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the number of activity indicators (0): ", err)
	}
	if numIndicators != 0 {
		s.Fatalf("Wrong number of activity indicators shown, got %d, want 0", numIndicators)
	}

	// Launch the files app.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}

	// Expect that 1 activity indicator is shown.
	numIndicators, err = numberOfActivityIndicators(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the number of activity indicators (1): ", err)
	}
	if numIndicators != 1 {
		s.Fatalf("Wrong number of activity indicators shown, got %d, want 1", numIndicators)
	}

	// Launch the paramaterized app type.
	if err = apps.Launch(ctx, tconn, appIDToLaunch); err != nil {
		s.Fatalf("Failed to launch %s: %v", testAppType, err)
	}
	if err = ash.WaitForApp(ctx, tconn, appIDToLaunch, time.Minute); err != nil {
		s.Fatalf("%s did not appear in shelf after launch: %v", testAppType, err)
	}

	// Expect that 2 activity indicators are shown.
	numIndicators, err = numberOfActivityIndicators(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the number of activity indicators (2): ", err)
	}
	if numIndicators != 2 {
		s.Fatalf("Wrong number of activity indicators shown, got %d, want 2", numIndicators)
	}

	// Close the paramaterized app.
	if err = apps.Close(ctx, tconn, appIDToLaunch); err != nil {
		s.Fatalf("Failed to close the %s: %v", testAppType, err)
	}
	if err := ash.WaitForAppClosed(ctx, tconn, appIDToLaunch); err != nil {
		s.Fatal("Failed to wait for testAppType to close: ", testAppType, err)
	}

	// Expect that 1 activity indicator is shown.
	numIndicators, err = numberOfActivityIndicators(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the number of activity indicators (3): ", err)
	}
	if numIndicators != 1 {
		s.Fatalf("Wrong number of activity indicators shown, got %d, want 1", numIndicators)
	}

	// Close the Files app.
	if err = files.Close(ctx); err != nil {
		s.Fatal("Failed to close the Files app: ", err)
	}
	if err := ash.WaitForAppClosed(ctx, tconn, apps.FilesSWA.ID); err != nil {
		s.Fatal("Failed to wait for Files app to close: ", err)
	}

	// Expect that 0 activity indicators are shown.
	numIndicators, err = numberOfActivityIndicators(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the number of activity indicators(4): ", err)
	}
	if numIndicators != 0 {
		s.Fatalf("Wrong number of activity indicators shown, got %d, want 0", numIndicators)
	}
}

// numberOfActivityIndicators returns the number of shelf app button activity indicator views that exist.
func numberOfActivityIndicators(ctx context.Context, tconn *chrome.TestConn) (int, error) {
	ui := uiauto.New(tconn)

	// Wait for shelf icons to complete animation before checking the number of activity indicators.
	if err := ash.WaitUntilShelfIconAnimationFinishAction(tconn)(ctx); err != nil {
		return -1, errors.Wrap(err, "failed to wait until the shelf icon animation finishes")
	}

	activityIndicators, err := ui.NodesInfo(ctx, nodewith.ClassName("ShelfAppButton::AppStatusIndicatorView"))
	if err != nil {
		return -1, errors.Wrap(err, "failed to find ShelfAppButton activity indicators")
	}

	numIndicators := len(activityIndicators)
	return numIndicators, nil
}
