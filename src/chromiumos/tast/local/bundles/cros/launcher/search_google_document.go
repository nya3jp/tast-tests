// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// A file on Google Drive might tooks longer to synchronize with fileaspp.
const driveSyncTimeout = 3 * time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchGoogleDocument,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "App Launcher Search: Google Document in Drive",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"victor.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "drivefs", "chrome_internal"},
		Fixture:      "driveFsStarted",
		Timeout:      2*time.Minute + driveSyncTimeout,
	})
}

// SearchGoogleDocument tests that App Launcher Search: Google Document in Drive.
func SearchGoogleDocument(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*drivefs.FixtureData).Chrome
	tconn := s.FixtValue().(*drivefs.FixtureData).TestAPIConn
	apiClient := s.FixtValue().(*drivefs.FixtureData).APIClient

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to take keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Ensure the name of gDoc file is unique by combine a long string, timestamp and a random number.
	gDocFilename := fmt.Sprintf("searchDrive_test_file-%020d-%06d", time.Now().UnixNano(), rand.Intn(100000))

	file, err := apiClient.CreateBlankGoogleDoc(ctx, gDocFilename, []string{"root"})
	if err != nil {
		s.Fatal("Failed to create blank google doc: ", err)
	}
	defer apiClient.RemoveFileByID(cleanupCtx, file.Id)

	if err := waitForGoogleDocSynced(ctx, cr, tconn, gDocFilename); err != nil {
		s.Fatalf("Failed to verify %s in drive: %q", gDocFilename, err)
	}

	// The expected result will not be an app, so launcher.SearchAndLaunchWithQuery and other similar functions do not work.
	if err := uiauto.Combine(fmt.Sprintf("search %q in launcher", gDocFilename),
		launcher.Open(tconn),
		launcher.Search(tconn, kb, gDocFilename),
	)(ctx); err != nil {
		s.Fatalf("Failed to search %s in launcher: %v", gDocFilename, err)
	}

	resultFinder := launcher.SearchResultListItemFinder.Role(role.ListBoxOption).NameRegex(regexp.MustCompile(`^` + gDocFilename)).First()
	ui := uiauto.New(tconn)

	if err := ui.LeftClick(resultFinder)(ctx); err != nil {
		s.Fatalf("Failed to left click %s in launcher: %v", gDocFilename, err)
	}
	defer ash.CloseAllWindows(cleanupCtx, tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "launched_result_ui_dump")

	browserRootFinder := nodewith.Role(role.Window).HasClass("BrowserRootView")
	expectedNode := browserRootFinder.NameRegex(regexp.MustCompile(fmt.Sprintf("^%s - Google Docs - Google Chrome - .*", gDocFilename)))

	if err := uiauto.New(tconn).WaitUntilExists(expectedNode)(ctx); err != nil {
		s.Fatal("Failed to verify search result: ", err)
	}
}

// waitForGoogleDocSynced wait the specified file exists in files app.
func waitForGoogleDocSynced(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, fileName string) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open files app")
	}
	defer files.Close(cleanupCtx)

	filesAppConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(fmt.Sprintf("chrome-extension://%s/main.html", apps.Files.ID)))
	if err != nil {
		return errors.Wrap(err, "failed to connect to files app foreground page")
	}
	defer filesAppConn.Close()
	reloadFilesappPage := func(ctx context.Context) error { return filesAppConn.Eval(ctx, `location.reload()`, nil) }

	if err := files.OpenDrive()(ctx); err != nil {
		return errors.Wrap(err, "failed to open drive")
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := files.WaitForFile(fileName + ".gdoc")(ctx); err != nil {
			if err := reloadFilesappPage(ctx); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to reload files app page"))
			}
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: driveSyncTimeout})
}
