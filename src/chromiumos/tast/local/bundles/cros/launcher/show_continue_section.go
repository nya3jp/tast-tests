// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
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
		Desc:         "Verify that a local file shows to Continue Section",
		Contacts: []string{
			"anasalazar@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3*time.Minute + cws.InstallationTimeout,
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val:  launcher.TestCase{ProductivityLauncher: true, TabletMode: false},
		}, {
			Name:              "tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

func ShowContinueSection(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	opt := chrome.EnableFeatures("ProductivityLauncher")

	// Start a new chrome session to avoid reusing user sessions and verify that the privacy nudge gets shown.
	cr, err := chrome.New(ctx, opt)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

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

	// If the sort nudge is shown, trigger sort to hide the sort nudge.
	if err := dismissSortNudge(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to dismiss sort nudge: ", err)
	}

	// Create enough fake files to show the continue section.
	var numFiles int
	if tabletMode {
		numFiles = 2
	} else {
		numFiles = 3
	}

	testDocFileNames := make([]string, 0)
	for i := 0; i < numFiles; i++ {
		testFileName := fmt.Sprintf("fake-file-%d-%d.html", time.Now().UnixNano(), rand.Intn(10000))
		testDocFileNames = append(testDocFileNames, testFileName)
		// Create a test file.
		filePath := filepath.Join(filesapp.DownloadPath, testFileName)
		fileContent := fmt.Sprintf("Test file %d", i)
		if err := ioutil.WriteFile(filePath, []byte(fileContent), 0644); err != nil {
			s.Fatalf("Failed to create file %d in Downloads: %v", i, err)
		}
		defer os.Remove(filePath)
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

	// Files need to be opened for them to get picked up for the Continue Section.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	for i, filePath := range testDocFileNames {
		if err := uiauto.Combine("Open file",
			filesApp.OpenDownloads(),
			filesApp.OpenFile(filePath),
		)(ctx); err != nil {
			s.Fatalf("Failed open the file %d - %s: %v", i, filePath, err)
		}

		if err := ash.WaitForApp(ctx, tconn, chromeApp.ID, 10*time.Second); err != nil {
			s.Fatalf("File %d - %s never opened: %v", i, filePath, err)
		}

		if err := apps.Close(ctx, tconn, chromeApp.ID); err != nil {
			s.Fatal("Failed to close browser: ", err)
		}

		if err := ash.WaitForAppClosed(ctx, tconn, chromeApp.ID); err != nil {
			s.Fatal("Browser did not close successfully: ", err)
		}

	}

	if err := openLauncher(ctx, tconn, tabletMode); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	continueSection := nodewith.ClassName("ContinueSectionView")
	if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(continueSection)(ctx); err != nil {
		s.Fatal("Failed to show continue section: ", err)
	}

	// Click on the button to confirm the privacy notice.
	privacyNoticeButton := nodewith.Ancestor(continueSection).ClassName("PillButton")
	if err := uiauto.Combine("Accept privacy notice",
		ui.WaitUntilExists(privacyNoticeButton),
		ui.LeftClick(privacyNoticeButton),
		ui.WaitUntilGone(privacyNoticeButton),
	)(ctx); err != nil {
		s.Fatal("Failed to confirm privacy notice: ", err)
	}

	for i, filePath := range testDocFileNames {
		// If the continue section is shown, then we don't need to try to re open the launcher.
		fileContent := fmt.Sprintf("Test file %d", i)
		if err := openFileFromContinueSection(ctx, tconn, tabletMode, filePath, fileContent); err != nil {
			s.Fatalf("Failed to open task %d - %s: %v", i, filePath, err)
		}
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

func dismissSortNudge(ctx context.Context, tconn *chrome.TestConn, tabletMode bool) error {
	ui := uiauto.New(tconn)
	// TODO(anasalazar): Replace this logic to use the dismiss button instead of sorting.
	sortNudge := nodewith.Name("Sort your apps by name or color")
	sortNudgeFound, err := ui.IsNodeFound(ctx, sortNudge)
	if err != nil {
		return errors.Wrap(err, "failed to search for sort nudge")
	}

	if sortNudgeFound {
		var appsGrid *nodewith.Finder
		if tabletMode {
			appsGrid = nodewith.ClassName(launcher.PagedAppsGridViewClass)
		} else {
			appsGrid = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
		}

		appsGridApp := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).First()
		if err := launcher.TriggerAppListSortAndWaitForUndoButtonExist(ctx, ui, launcher.AlphabeticalSort, appsGridApp); err != nil {
			return errors.Wrap(err, "failed to trigger sort")
		}
	}
	return nil
}

func openFileFromContinueSection(ctx context.Context, tconn *chrome.TestConn, tabletMode bool, filePath, fileContent string) error {
	ui := uiauto.New(tconn)
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	// If the continue section is shown, then we don't need to try to re open the launcher.
	continueSection := nodewith.ClassName("ContinueSectionView")
	continueSectionFound, err := ui.IsNodeFound(ctx, continueSection)
	if err != nil {
		return errors.Wrap(err, "failed to search for continue section")
	}
	if !continueSectionFound {
		if err := openLauncher(ctx, tconn, tabletMode); err != nil {
			return errors.Wrap(err, "failed to open the launcher")
		}
	}
	continueTask := nodewith.Ancestor(continueSection).Name(filePath)
	if err := uiauto.Combine("Open file task",
		ui.WithTimeout(3*time.Second).WaitUntilExists(continueTask),
		ui.DoubleClick(continueTask),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to open the task on continue section")
	}

	if err := ash.WaitForApp(ctx, tconn, chromeApp.ID, 10*time.Second); err != nil {
		return errors.Wrap(err, "browser window never opened")
	}

	fileContentNode := nodewith.Name(fileContent).First()
	if err := ui.WaitUntilExists(fileContentNode)(ctx); err != nil {
		return errors.Wrap(err, "failed to find the file contents node")
	}

	if err := apps.Close(ctx, tconn, chromeApp.ID); err != nil {
		return errors.Wrap(err, "failed to trigger browser close")
	}

	if err := ash.WaitForAppClosed(ctx, tconn, chromeApp.ID); err != nil {
		return errors.Wrap(err, "browser did not close succesfully")
	}
	return nil
}
