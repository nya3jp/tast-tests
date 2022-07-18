// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"io/ioutil"
	"os"
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
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	appIconFileName             = "app_list_sort_smoke_white.png"
	cwsAppID                    = "mljpablpddhocfbnokacjggdbmafjnon"
	cwsAppName                  = "Wicked Good Unarchiver"
	fakeAppName                 = "fake app 0"
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
		Data:         []string{appIconFileName},
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  launcher.TestCase{TabletMode: false},
			},
			{
				Name:              "tablet_mode",
				Val:               launcher.TestCase{TabletMode: true},
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			},
		},
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

// NewlyInstalledApps checks that newly installed apps are marked as such in launcher.
func NewlyInstalledApps(ctx context.Context, s *testing.State) {
	tc := s.Param().(launcher.TestCase)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	extDirBase, err := ioutil.TempDir("", "launcher_NewlyInstalledApps")
	if err != nil {
		s.Fatal("Failed to create a temporary directory: ", err)
	}
	defer os.RemoveAll(extDirBase)

	iconBytes, err := launcher.ReadImageBytesFromFilePath(s.DataPath(appIconFileName))
	if err != nil {
		s.Fatal("Failed to read icon byte data: ", err)
	}
	opts, err := ash.GeneratePrepareFakeAppsWithIconDataOptions(extDirBase, []string{fakeAppName}, [][]byte{iconBytes})
	if err != nil {
		s.Fatal("Failed to create a fake app: ", err)
	}

	cr, err := chrome.New(ctx, append(opts, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")), chrome.EnableFeatures("ProductivityLauncher"))...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	if err := cws.InstallApp(ctx, cr, tconn, cws.App{
		Name: cwsAppName,
		URL:  "https://chrome.google.com/webstore/detail/wicked-good-unarchiver/" + cwsAppID,
	}); err != nil {
		s.Fatal("Unable to install cws app: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tc.TabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure tablet mode state %t: %v", tc.TabletMode, err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	if !tc.TabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	for _, app := range []apps.App{
		{Name: cwsAppName, ID: cwsAppID},
		{Name: fakeAppName, ID: apps.Chrome.ID},
	} {
		if err := launcher.OpenProductivityLauncher(ctx, tconn, tc.TabletMode); err != nil {
			s.Fatal("Failed to open launcher: ", err)
		}
		view := appItemViewNode(app.Name, tc.TabletMode)
		if isNewInstall, err := isInNewInstallState(ctx, cr, tconn, view); err != nil {
			s.Fatal("Unable to compute new install state: ", err)
		} else if !isNewInstall {
			s.Fatalf("Unexpected new install state before launching the app; got %t, want %t", isNewInstall, true)
		}
		if err := launcher.HideLauncher(tconn, !tc.TabletMode)(ctx); err != nil {
			s.Fatal("Failed to hide launcher: ", err)
		}

		if err := launcher.LaunchAndWaitForAppOpen(tconn, app)(ctx); err != nil {
			s.Fatal("Unable to launch the app: ", err)
		}

		if err := launcher.OpenProductivityLauncher(ctx, tconn, tc.TabletMode); err != nil {
			s.Fatal("Failed to open launcher: ", err)
		}
		if isNewInstall, err := isInNewInstallState(ctx, cr, tconn, view); err != nil {
			s.Fatal("Unable to compute new install state: ", err)
		} else if isNewInstall {
			s.Fatalf("Unexpected new install state after launching the app; got %t, want %t", isNewInstall, false)
		}
		if err := launcher.HideLauncher(tconn, !tc.TabletMode)(ctx); err != nil {
			s.Fatal("Failed to hide launcher: ", err)
		}
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

	return imgcmp.CountPixels(img, colorcmp.RGB(0x1a, 0x73, 0xe8)) >= minNewInstallDotPixelsCount, nil
}
