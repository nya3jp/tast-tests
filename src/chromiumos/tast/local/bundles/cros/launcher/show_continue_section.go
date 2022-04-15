// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowContinueSection,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that a google doc created via Drive API shows to Continue Section",
		Contacts: []string{
			"anasalazar@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3*time.Minute + cws.InstallationTimeout,
		Params: []testing.Param{{
			Name:    "productivity_launcher_clamshell_mode",
			Val:     launcher.TestCase{ProductivityLauncher: true, TabletMode: false},
			Fixture: "chromeLoggedInWithGaiaProductivityLauncher",
		}, {
			Name:              "productivity_launcher_tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: true},
			Fixture:           "chromeLoggedInWithGaiaProductivityLauncher",
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

func ShowContinueSection(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)
	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	if !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	if err := openLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	// Trigger sort to hide the sort nudge
	var appsGrid *nodewith.Finder
	var sortMethod launcher.SortType
	if tabletMode {
		appsGrid = nodewith.ClassName(launcher.PagedAppsGridViewClass)
		sortMethod = launcher.ColorSort
	} else {
		appsGrid = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
		sortMethod = launcher.AlphabeticalSort
	}

	appsGridApp := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).First()
	if err := launcher.TriggerAppListSortAndWaitForUndoButtonExist(ctx, ui, sortMethod, appsGridApp); err != nil {
		s.Fatal("Failed to trigger sort: ", err)
	}

	// Create enough fake files to show the continue section
	var numFiles int
	if tabletMode {
		numFiles = 2
	} else {
		numFiles = 3
	}

	testDocFileNames := make([]string, 0)
	for i := 0; i < numFiles; i++ {
		fileName := fmt.Sprintf("fake-file-%d-%d.txt", time.Now().UnixNano(), rand.Intn(10000))
		testDocFileNames = append(testDocFileNames, fileName)
		// Create a blank file
		filePath := filepath.Join(filesapp.DownloadPath, fileName)
		if err := ioutil.WriteFile(filePath, []byte("testString"), 0644); err != nil {
			s.Fatalf("Failed to create file %d in Downloads: %v", i, err)
		}
	}

	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not launch the Files App: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Cannot create keyboard: ", err)
	}
	defer keyboard.Close()

	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err := uiauto.Combine("Open files",
		filesApp.OpenDownloads(),
		filesApp.SelectMultipleFiles(keyboard, testDocFileNames...),
		keyboard.AccelAction("Enter"),
	)(ctx); err != nil {
		s.Fatal("Failed open the created files: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, chromeApp.ID, 10*time.Second); err != nil {
		s.Fatal("Files never opened: ", err)
	}

	if err := openLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	continueSection := nodewith.ClassName("ContinueSectionView")
	if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(continueSection)(ctx); err != nil {
		s.Fatal("Failed to show continue section: ", err)
	}
}

func openLauncher(ctx context.Context, tconn *chrome.TestConn, tabletMode bool) error {
	if tabletMode {
		touchScreen, err := input.Touchscreen(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get the touch screen")
		}
		defer touchScreen.Close()

		stw, err := touchScreen.NewSingleTouchWriter()
		if err != nil {
			return errors.Wrap(err, "failed to get the single touch event writer")
		}
		defer stw.Close()

		// Make sure the shelf bounds is stable before dragging.
		if err := ash.WaitForStableShelfBounds(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for stable shelf bounds")
		}
		if err := ash.DragToShowHomescreen(ctx, touchScreen.Width(), touchScreen.Height(), stw, tconn); err != nil {
			return errors.Wrap(err, "failed to show homescreen")
		}
	} else {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			return errors.Wrap(err, "failed to open bubble launcher")
		}
	}
	return nil
}
