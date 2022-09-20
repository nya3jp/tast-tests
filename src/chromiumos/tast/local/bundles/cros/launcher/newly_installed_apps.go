// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type newlyInstalledAppType int

const (
	newlyInstalledCwsApp newlyInstalledAppType = iota
)

type newlyInstalledAppsTestCase struct {
	appID      string
	appName    string
	appType    newlyInstalledAppType
	tabletMode bool
}

const (
	minNewInstallDotPixelsCount = 16
	newInstallDescription       = "New install"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NewlyInstalledApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that newly installed apps are marked as such in launcher",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"amitrokhin@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "cws_clamshell_mode",
				Val: newlyInstalledAppsTestCase{
					appID:      "mljpablpddhocfbnokacjggdbmafjnon",
					appName:    "Wicked Good Unarchiver",
					appType:    newlyInstalledCwsApp,
					tabletMode: false,
				},
				Fixture: "chromeLoggedInWithGaia",
			},
			{
				Name: "cws_tablet_mode",
				Val: newlyInstalledAppsTestCase{
					appID:      "mljpablpddhocfbnokacjggdbmafjnon",
					appName:    "Wicked Good Unarchiver",
					appType:    newlyInstalledCwsApp,
					tabletMode: true,
				},
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Fixture:           "chromeLoggedInWithGaia",
			},
		},
	})
}

// NewlyInstalledApps checks that newly installed apps are marked as such in launcher.
func NewlyInstalledApps(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tc := s.Param().(newlyInstalledAppsTestCase)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	if tc.appType == newlyInstalledCwsApp {
		if err := cws.InstallApp(ctx, cr, tconn, cws.App{
			Name: tc.appName,
			URL:  "https://chrome.google.com/webstore/detail/wicked-good-unarchiver/" + tc.appID,
		}); err != nil {
			s.Fatal("Unable to install cws app: ", err)
		}
		defer closeAppAndUninstallViaSettings(ctx, cr, tconn, tc.appName, tc.appID)
	}

	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, tc.tabletMode, true /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	view := appItemViewNode(tc.appName, tc.tabletMode)

	if isNewInstall, err := isInNewInstallState(ctx, cr, tconn, view); err != nil {
		s.Fatal("Unable to compute new install state: ", err)
	} else if !isNewInstall {
		s.Fatalf("Unexpected new install state before launching the app; got %t, want %t", isNewInstall, true)
	}
	if err := launcher.HideLauncher(tconn, !tc.tabletMode)(ctx); err != nil {
		s.Fatal("Failed to hide launcher: ", err)
	}

	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.App{ID: tc.appID, Name: tc.appName})(ctx); err != nil {
		s.Fatal("Unable to launch the app: ", err)
	}

	if err := launcher.OpenProductivityLauncher(ctx, tconn, tc.tabletMode); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}
	if isNewInstall, err := isInNewInstallState(ctx, cr, tconn, view); err != nil {
		s.Fatal("Unable to compute new install state: ", err)
	} else if isNewInstall {
		s.Fatalf("Unexpected new install state after launching the app; got %t, want %t", isNewInstall, false)
	}
}

// appItemViewNode finds the app node ignoring the recent apps section.
func appItemViewNode(appName string, tabletMode bool) *nodewith.Finder {
	var ancestorNode *nodewith.Finder
	if tabletMode {
		ancestorNode = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	} else {
		ancestorNode = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	}
	return launcher.AppItemViewFinder(appName).Ancestor(ancestorNode).First()
}

// isInNewInstallState computes if the app view is in new install state.
func isInNewInstallState(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, view *nodewith.Finder) (bool, error) {
	hasDescr, err := hasNewInstallDescription(ctx, tconn, view)
	if err != nil {
		return false, err
	}

	hasDot, err := hasNewInstallDot(ctx, cr, tconn, view)
	if err != nil {
		return false, err
	}

	if hasDescr != hasDot {
		return false, errors.New("Accessibility description and new install dot should be in sync")
	}

	return hasDescr && hasDot, nil
}

// hasNewInstallDescription computes whether the app view has new install accessibility description.
func hasNewInstallDescription(ctx context.Context, tconn *chrome.TestConn, view *nodewith.Finder) (bool, error) {
	ui := uiauto.New(tconn)
	viewInfo, err := ui.Info(ctx, view)
	if err != nil {
		return false, errors.Wrap(err, "failed to get app item view info")
	}
	return viewInfo.Description == newInstallDescription, nil
}

// hasNewInstallDot computes whether the app view has a blue dot.
func hasNewInstallDot(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, view *nodewith.Finder) (bool, error) {
	ui := uiauto.New(tconn)
	viewLocation, err := ui.Location(ctx, view)
	if err != nil {
		return false, errors.Wrap(err, "failed to get app item view location")
	}

	displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to get the primary display info")
	}

	displayMode, err := displayInfo.GetSelectedMode()
	if err != nil {
		return false, errors.Wrap(err, "failed to get the selected display mode of the primary display")
	}

	rect := coords.ConvertBoundsFromDPToPX(*viewLocation, displayMode.DeviceScaleFactor)
	img, err := screenshot.GrabAndCropScreenshot(ctx, cr, rect)
	if err != nil {
		return false, errors.Wrap(err, "failed to grab a screenshot")
	}

	hasLightModePixels := imgcmp.CountPixels(img, colorcmp.RGB(0x1a, 0x73, 0xe8)) >= minNewInstallDotPixelsCount
	hasDarkModePixels := imgcmp.CountPixels(img, colorcmp.RGB(0x8a, 0xb4, 0xf8)) >= minNewInstallDotPixelsCount

	return hasLightModePixels || hasDarkModePixels, nil
}

// closeAppAndUninstallViaSettings closes the app and uninstalls it via ossettings.
func closeAppAndUninstallViaSettings(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, name, id string) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, tconn, id)
	if err != nil {
		return errors.Wrap(err, "failed to check app's existance")
	}

	if !isInstalled {
		testing.ContextLogf(ctx, "App %q is already uninstalled", name)
		return nil
	}

	// Some apps have "always on top" popup that prevents from clicking on correct nodes in Settings.
	if err := apps.Close(ctx, tconn, id); err != nil {
		return errors.Wrap(err, "failed to close the app")
	}

	defer func() {
		settings := ossettings.New(tconn)
		settings.Close(ctx)
	}()

	testing.ContextLogf(ctx, "Uninstall app: %q", name)
	if err := ossettings.UninstallApp(ctx, tconn, cr, name, id); err != nil {
		return errors.Wrap(err, "failed to uninstall the app via os settings")
	}

	return ash.WaitForChromeAppUninstalled(ctx, tconn, id, 30*time.Second)
}
