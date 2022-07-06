// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
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
	appName                     = "fake app 0"
	appIconFileName             = "app_list_sort_smoke_white.png"
	minNewInstallDotPixelsCount = 16
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
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val:  launcher.TestCase{TabletMode: false},
		}, {
			Name:              "tablet_mode",
			Val:               launcher.TestCase{TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// NewlyInstalledApps checks that newly installed apps are marked as such in launcher.
func NewlyInstalledApps(ctx context.Context, s *testing.State) {
	tc := s.Param().(launcher.TestCase)

	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a temporary directory: ", err)
	}
	defer os.RemoveAll(extDirBase)

	iconBytes, err := launcher.ReadImageBytesFromFilePath(s.DataPath(appIconFileName))
	if err != nil {
		s.Fatal("Failed to read icon byte data: ", err)
	}
	opts, err := ash.GeneratePrepareFakeAppsWithIconDataOptions(extDirBase, []string{appName}, [][]byte{iconBytes})
	if err != nil {
		s.Fatal("Failed to create a fake app: ", err)
	}

	cr, err := chrome.New(ctx, append(opts, chrome.EnableFeatures("ProductivityLauncher"))...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tc.TabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure tablet mode state %t: %v", tc.TabletMode, err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	if !tc.TabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	launcher.OpenProductivityLauncher(ctx, tconn, tc.TabletMode)
	if err := verifyAppItemView(ctx, cr, tconn, tc.TabletMode, true); err != nil {
		s.Fatal("Unable to verify app item view: ", err)
	}

	if err := launcher.LaunchApp(tconn, appName)(ctx); err != nil {
		s.Fatal("Unable to launch the app: ", err)
	}

	launcher.OpenProductivityLauncher(ctx, tconn, tc.TabletMode)
	if err := verifyAppItemView(ctx, cr, tconn, tc.TabletMode, false); err != nil {
		s.Fatal("Unable to verify app item view: ", err)
	}
}

// getAppItemView finds the "fake app 0" node ignoring the recent apps section.
func getAppItemView(tabletMode bool) *nodewith.Finder {
	var ancestorNode *nodewith.Finder
	if tabletMode {
		ancestorNode = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	} else {
		ancestorNode = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	}
	return launcher.AppItemViewFinder(appName).Ancestor(ancestorNode).First()
}

// verifyAppItemView verifies presence of "New install" accessible description and a blue dot.
func verifyAppItemView(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, tabletMode, isNew bool) error {
	ui := uiauto.New(tconn)

	view := getAppItemView(tabletMode)
	viewInfo, err := ui.Info(ctx, view)
	if err != nil {
		return errors.Wrap(err, "failed to get app item view info")
	}

	descr := viewInfo.Description
	if isNew && descr != "New install" {
		return errors.Wrapf(err, "unexpected accessibility description, expected %q, got: %q", "New install", descr)
	} else if !isNew && descr != "" {
		return errors.Wrapf(err, "unexpected accessibility description, expected %q, got: %q", "", descr)
	}

	displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	displayMode, err := displayInfo.GetSelectedMode()
	if err != nil {
		return errors.Wrap(err, "failed to get the selected display mode of the primary display")
	}

	rect := coords.ConvertBoundsFromDPToPX(viewInfo.Location, displayMode.DeviceScaleFactor)
	img, err := screenshot.GrabAndCropScreenshot(ctx, cr, rect)
	if err != nil {
		return errors.Wrap(err, "failed to grab a screenshot")
	}

	bluePixelsCount := imgcmp.CountPixels(img, colorcmp.RGB(0x1a, 0x73, 0xe8))
	if isNew && bluePixelsCount < minNewInstallDotPixelsCount {
		return errors.Wrap(err, "newly installed app should have a blue dot")
	} else if !isNew && bluePixelsCount >= minNewInstallDotPixelsCount {
		return errors.Wrap(err, "previously opened app shouldn't have a blue dot")
	}

	return nil
}
