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
		Func:         RemoveContinueSectionTask,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that a task gets removed from the Continue Section",
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

func RemoveContinueSectionTask(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	opt := chrome.EnableFeatures("ProductivityLauncher", "FeedbackOnContinueSectionRemove")

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
		numFiles = 3
	} else {
		numFiles = 4
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
	filesApp.Close(ctx)

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

	filePath := testDocFileNames[0]
	if err := removeFileFromContinueSection(ctx, tconn, tabletMode, filePath); err != nil {
		s.Fatalf("Failed to attempt to remove task 0 - %s: %v", filePath, err)
	}

	feedbackDialog := nodewith.ClassName("RemoveTaskFeedbackDialog").First()
	cancelDialogButton := nodewith.Ancestor(feedbackDialog).ClassName("PillButton").Name("Cancel")
	if err := uiauto.Combine("Cancel remove feedback dialog",
		ui.WaitUntilExists(feedbackDialog),
		ui.LeftClick(cancelDialogButton),
		ui.WaitUntilGone(feedbackDialog),
	)(ctx); err != nil {
		s.Fatal("Failed to cancel the remove feedback dialog: ", err)
	}
	
	if err := ui.Exists(nodewith.Ancestor(continueSection).Name(filePath))(ctx); err != nil {
		s.Fatalf("Failed to confirm the task 0 - %s still exists: %v", filePath, err)
	}

	continueTask := nodewith.Ancestor(continueSection).Name(filePath)
	taskInfo, err := ui.Info(ctx, continueTask)
	if err != nil {
		s.Fatalf("Failed to find info: ", err)
	}
	s.Logf("Node unique:")
	s.Logf("Node name %s:", taskInfo.Name)
	s.Logf("Node class name %s:", taskInfo.ClassName)
	s.Logf("Node value %s:", taskInfo.Value)
	s.Logf("Node location %s:", taskInfo.Location)
	s.Logf("Node state total %d:", len(taskInfo.State))
	for key, state := range taskInfo.State {
		s.Logf("State %s: %d", key, state)
	}

	// Try to remove the file again 
	if err := removeFileFromContinueSection(ctx, tconn, tabletMode, filePath); err != nil {
		s.Fatalf("Failed to attempt to remove task 0 - %s: %v", filePath, err)
	}

	removeDialogButton := nodewith.Ancestor(feedbackDialog).ClassName("PillButton").Name("Remove")
	radioButtonSuggestion := nodewith.Ancestor(feedbackDialog).ClassName("RadioButton").First()
	if err := uiauto.Combine("Accept privacy notice",
		ui.WaitUntilExists(feedbackDialog),
		ui.LeftClick(radioButtonSuggestion),
		ui.LeftClick(removeDialogButton),
		ui.WaitUntilGone(feedbackDialog),
	)(ctx); err != nil {
		s.Fatal("Failed to confirm remove feedback dialog: ", err)
	}
	s.Logf("Removing: %s", filePath)
	taskInfoArray, err := ui.NodesInfo(ctx, continueTask)
	if err != nil {
		s.Fatalf("Failed to find info: ", err)
	}
	for i, task := range taskInfoArray{
		s.Logf("Node %d:", i)
		s.Logf("Node %d name %s:", i, task.Name)
		s.Logf("Node %d class name %s:", i, task.ClassName)
		s.Logf("Node %d location %s:", i, task.Location)
		s.Logf("Node %d value %s:", i, task.Value)
		for key, state := range task.State {
			s.Logf("Node %d State %s: %s", i, state, key)
		}
	}

	if err :=ui.WaitUntilExists(nodewith.Ancestor(continueSection).Name(filePath))(ctx); err != nil {
		s.Fatalf("Failed to confirm the task 0 - %s was removed: %v", filePath, err)
	}

	filePath = testDocFileNames[1]
	continueTask = nodewith.Ancestor(continueSection).Name(filePath)
	taskInfoArray, err = ui.NodesInfo(ctx, continueTask)
	if err != nil {
		s.Fatalf("Failed to find info: ", err)
	}
	for i, task := range taskInfoArray{
		s.Logf("Node %d:", i)
		s.Logf("Node %d name %s:", i, task.Name)
		s.Logf("Node %d class name %s:", i, task.ClassName)
		s.Logf("Node %d location %s:", i, task.Location)
		s.Logf("Node %d value %s:", i, task.Value)
		for key, state := range task.State {
			s.Logf("Node %d State %s: %s", i, state, key)
		}
	}
	if err := removeFileFromContinueSection(ctx, tconn, tabletMode, filePath); err != nil {
		s.Fatalf("Failed to attempt to remove task 1 - %s: %v", filePath, err)
	}
	
	if err := ui.Gone(feedbackDialog)(ctx); err == nil {
		s.Fatalf("Failed to verify that the feedback dialog was not displayed", err)
	}

	if err :=ui.WaitUntilExists(nodewith.Ancestor(continueSection).Name(filePath))(ctx); err != nil {
		s.Fatalf("Failed to confirm the task 1 - %s was removed: %v", filePath, err)
	}

}

func removeFileFromContinueSection(ctx context.Context, tconn *chrome.TestConn, tabletMode bool, filePath string) error {
	ui := uiauto.New(tconn)
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
	if err := uiauto.Combine("Remove file task",
		ui.WithTimeout(3*time.Second).WaitUntilExists(continueTask),
		ui.RightClick(continueTask),
		ui.LeftClick(nodewith.Name("Remove Suggestion").ClassName("MenuItemView")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to remove the task on continue section")
	}
	return nil;
}