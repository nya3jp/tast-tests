// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"io/ioutil"
	"os"

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

	opts, err := ash.GeneratePrepareFakeAppsOptions(extDirBase, 1)
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

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tc.TabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure tablet mode state %t: %v", tc.TabletMode, err)
	}
	defer cleanup(ctx)

	if !tc.TabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	launcher.OpenProductivityLauncher(ctx, tconn, tc.TabletMode)
	verifyAppItemView(ctx, s, cr, tconn, true)

	if err := launcher.LaunchApp(tconn, "fake app 0")(ctx); err != nil {
		s.Fatal("Unable to launch the app: ", err)
	}

	launcher.OpenProductivityLauncher(ctx, tconn, tc.TabletMode)
	verifyAppItemView(ctx, s, cr, tconn, false)
}

// getAppItemView finds the "fake app 0" node ignoring the recent apps section.
func getAppItemView(tabletMode bool) *nodewith.Finder {
	var ancestorNode *nodewith.Finder
	if tabletMode {
		ancestorNode = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	} else {
		ancestorNode = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	}
	return launcher.AppItemViewFinder("fake app 0").Ancestor(ancestorNode).First()
}

// verifyAppItemView verifies presence of "New install" accessible description and a blue dot.
func verifyAppItemView(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, isNew bool) {
	tc := s.Param().(launcher.TestCase)
	ui := uiauto.New(tconn)

	view := getAppItemView(tc.TabletMode)
	viewInfo, err := ui.Info(ctx, view)
	if err != nil {
		s.Fatal("Failed to get app item view info: ", err)
	}

	descr := viewInfo.Description
	if isNew && descr != "New install" {
		s.Fatalf("Unexpected accessibility description, expected %q, got: %q", "New install", descr)
	} else if !isNew && descr != "" {
		s.Fatalf("Unexpected accessibility description, expected %q, got: %q", "", descr)
	}

	displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	displayMode, err := displayInfo.GetSelectedMode()
	if err != nil {
		s.Fatal("Failed to get the selected display mode of the primary display: ", err)
	}

	rect := coords.ConvertBoundsFromDPToPX(viewInfo.Location, displayMode.DeviceScaleFactor)
	img, err := screenshot.GrabAndCropScreenshot(ctx, cr, rect)
	if err != nil {
		s.Fatal("Failed to grab a screenshot: ", err)
	}

	bluePixelsCount := imgcmp.CountPixels(img, colorcmp.RGB(0x1a, 0x73, 0xe8))
	if isNew && bluePixelsCount == 0 {
		s.Fatal("Newly installed app should have a blue dot")
	} else if !isNew && bluePixelsCount != 0 {
		s.Fatal("Previously opened app shouldn't have a blue dot")
	}
}
