// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type launcherSearchTest string

const (
	webBrowser launcherSearchTest = "web browser"
	gDoc       launcherSearchTest = "google doc"

	launcherSearchTimeout = 5 * time.Second
)

type searchDetail struct {
	searchKeyWord    string
	expectedFinder   *nodewith.Finder // Verify the node of search results is exists.
	searchResultView *nodewith.Finder // Verify the node of search result we will open.
	verifiedNode     *nodewith.Finder // Verify if the web or drive is opened.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchWeb,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "App Launcher Search: Web, Google Drive",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"victor.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "drivefs", "chrome_internal"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Timeout: time.Minute,
		}, {
			Name:    "drive",
			Fixture: "driveFsStarted",
			Timeout: 2 * time.Minute,
		}},
	})
}

// SearchWeb tests that App Launcher Search: Web, Google Drive.
func SearchWeb(ctx context.Context, s *testing.State) {
	var (
		tconn        *chrome.TestConn
		testDetail   searchDetail
		gDocFilename string
	)

	if strings.HasSuffix(s.TestName(), "drive") {
		tconn = s.FixtValue().(*drivefs.FixtureData).TestAPIConn

		// Ensure the name of gDoc file is unique by combine a long string, timestamp and a random number.
		gDocFilename = fmt.Sprintf("searchDrive_test_file-%020d-%06d", time.Now().UnixNano(), rand.Intn(100000))
		gDocNode := nodewith.NameRegex(regexp.MustCompile(`^` + gDocFilename))

		testDetail = searchDetail{
			searchKeyWord:    gDocFilename,
			expectedFinder:   gDocNode.First(),
			searchResultView: gDocNode.Role(role.ListBoxOption).HasClass("ui/app_list/SearchResultView").First(),
			verifiedNode:     gDocNode.HasClass("Label"),
		}
	} else {
		var err error
		if tconn, err = s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to create Test API connection: ", err)
		}

		testDetail = searchDetail{
			searchKeyWord:    "web browser",
			expectedFinder:   launcher.SearchResultListItemFinder.First(),
			searchResultView: launcher.SearchResultListItemFinder.Name("web browser, Google Search"),
			verifiedNode:     nodewith.NameContaining("web browser").HasClass("BrowserFrame"),
		}
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to take keyboard: ", err)
	}
	defer kb.Close()

	if strings.HasSuffix(s.TestName(), "drive") {
		apiClient := s.FixtValue().(*drivefs.FixtureData).APIClient

		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		file, err := apiClient.CreateBlankGoogleDoc(ctx, gDocFilename, []string{"root"})
		if err != nil {
			s.Fatal("Failed to create blank google doc: ", err)
		}
		defer apiClient.RemoveFileByID(cleanupCtx, file.Id)

		if err := waitForGoogleDocExists(ctx, tconn, gDocFilename); err != nil {
			s.Fatalf("Failed to verify %s in drive: %q", gDocFilename, err)
		}
	}

	if err := waitForSearchResult(ctx, tconn, kb, &testDetail); err != nil {
		s.Fatal("Failed to search by launcher: ", err)
	}
}

// waitForGoogleDocExists wait the specified file exists in files app.
func waitForGoogleDocExists(ctx context.Context, tconn *chrome.TestConn, fileName string) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	file, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open filesapp")
	}
	defer file.Close(cleanupCtx)

	return uiauto.Combine("open drive and wait for "+fileName,
		file.OpenDrive(),
		file.WithTimeout(time.Minute).WaitForFile(fileName+".gdoc"),
	)(ctx)
}

// searchAndVerify searches given detail from launcher and verifies the result in the opened browser.
func searchAndVerify(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, detail *searchDetail) error {
	ui := uiauto.New(tconn)

	dismissLauncherCtx := ctx
	ctx, cancelDismissLauncherCtx := ctxutil.Shorten(ctx, 15*time.Second) // WaitForLauncherState waits upto 10 seconds.
	defer cancelDismissLauncherCtx()

	if err := launcher.Open(tconn)(ctx); err != nil {
		return errors.Wrap(err, "failed to open launcher")
	}
	defer closeLauncher(dismissLauncherCtx, tconn, kb, ui)

	testing.ContextLogf(ctx, "Seaching keyword %q in the Launcher", detail.searchKeyWord)
	if err := launcher.Search(tconn, kb, detail.searchKeyWord)(ctx); err != nil {
		return errors.Wrapf(err, "failed to search keyword %q in the Launcher", detail.searchKeyWord)
	}

	cleanupCtx := ctx
	ctx, cancelCleanupCtx := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancelCleanupCtx()

	if err := uiauto.Combine("wait for expected result and open it",
		ui.WithTimeout(launcherSearchTimeout).WaitUntilExists(detail.expectedFinder),
		ui.LeftClick(detail.searchResultView),
	)(ctx); err != nil {
		return err
	}
	defer closeAllWindows(cleanupCtx, tconn)

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		return errors.Wrap(err, "failed to wait for launcher closed after open search result")
	}

	return ui.WithTimeout(launcherSearchTimeout).WaitUntilExists(detail.verifiedNode)(ctx)
}

// waitForSearchResult polls for search from launcher and validate search detail.
func waitForSearchResult(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, detail *searchDetail) error {
	// Long timeout is required for low end DUTs to ensure the result is properly shows in the Launcher.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return searchAndVerify(ctx, tconn, kb, detail)
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait and verify the search result by launcher")
	}

	return nil
}

// closeAllWindows closes the applications which are opened in the window.
func closeAllWindows(ctx context.Context, tconn *chrome.TestConn) error {
	shelfItems, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get shelf items")
	}

	for _, shelfItem := range shelfItems {
		if shelfItem.Status != ash.ShelfItemClosed {
			if err := apps.Close(ctx, tconn, shelfItem.AppID); err != nil {
				return errors.Wrapf(err, "failed to close the app %v", shelfItem.AppID)
			}
		}
	}

	return nil
}

// closeLauncher make sure launcher is closed.
func closeLauncher(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, ui *uiauto.Context) error {
	return ui.RetryUntil(
		kb.AccelAction("esc"),
		func(ctx context.Context) error { return ash.WaitForLauncherState(ctx, tconn, ash.Closed) },
	)(ctx)
}
