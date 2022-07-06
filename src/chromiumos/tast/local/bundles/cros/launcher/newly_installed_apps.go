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
	})
}

// Testable properties of the appItemViewNode.
type appItemViewState struct {
	accessibilityDescription string
	newInstallDotPixelsCount int
}

// NewlyInstalledApps checks that newly installed apps are marked as such in launcher.
func NewlyInstalledApps(ctx context.Context, s *testing.State) {
	tc := s.Param().(launcher.TestCase)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	extDirBase, err := ioutil.TempDir("", "launcher.NewlyInstalledApps")
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
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	if !tc.TabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	launcher.OpenProductivityLauncher(ctx, tconn, tc.TabletMode)
	state, err := computeAppItemViewState(ctx, cr, tconn, tc.TabletMode)
	if err != nil {
		s.Fatal("Unable to compute app item view state: ", err)
	} else if state.accessibilityDescription != "New install" || state.newInstallDotPixelsCount < minNewInstallDotPixelsCount {
		s.Fatalf("Unexpected app item view state. Expected: accessibility description %q, new install dot pixels count >=%d. Got: accessibility description %q, new install dot pixels count %d",
			"New install", minNewInstallDotPixelsCount, state.accessibilityDescription, state.newInstallDotPixelsCount)
	}

	if err := launcher.LaunchApp(tconn, appName)(ctx); err != nil {
		s.Fatal("Unable to launch the app: ", err)
	}

	launcher.OpenProductivityLauncher(ctx, tconn, tc.TabletMode)
	state, err = computeAppItemViewState(ctx, cr, tconn, tc.TabletMode)
	if err != nil {
		s.Fatal("Unable to compute app item view state: ", err)
	} else if state.accessibilityDescription != "" || state.newInstallDotPixelsCount >= minNewInstallDotPixelsCount {
		s.Fatalf("Unexpected app item view state. Expected: accessibility description %q, new install dot pixels count <%d. Got: accessibility description %q, new install dot pixels count %d",
			"", minNewInstallDotPixelsCount, state.accessibilityDescription, state.newInstallDotPixelsCount)
	}
}

// appItemViewNode finds the "fake app 0" node ignoring the recent apps section.
func appItemViewNode(tabletMode bool) *nodewith.Finder {
	var ancestorNode *nodewith.Finder
	if tabletMode {
		ancestorNode = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	} else {
		ancestorNode = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	}
	return launcher.AppItemViewFinder(appName).Ancestor(ancestorNode).First()
}

// computeAppItemViewState returns appItemViewNode's accessibility description and new install dot pixels count.
func computeAppItemViewState(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, tabletMode bool) (*appItemViewState, error) {
	ui := uiauto.New(tconn)

	view := appItemViewNode(tabletMode)
	viewInfo, err := ui.Info(ctx, view)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get app item view info")
	}

	displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the primary display info")
	}

	displayMode, err := displayInfo.GetSelectedMode()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the selected display mode of the primary display")
	}

	rect := coords.ConvertBoundsFromDPToPX(viewInfo.Location, displayMode.DeviceScaleFactor)
	img, err := screenshot.GrabAndCropScreenshot(ctx, cr, rect)
	if err != nil {
		return nil, errors.Wrap(err, "failed to grab a screenshot")
	}

	return &appItemViewState{
		accessibilityDescription: viewInfo.Description,
		newInstallDotPixelsCount: imgcmp.CountPixels(img, colorcmp.RGB(0x1a, 0x73, 0xe8)),
	}, nil
}
