// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
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
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type launcherSearchTest string

const (
	webBrowser launcherSearchTest = "web browser"
	gDoc       launcherSearchTest = "google doc"

	gDocFilename = "ui_launcher_search"
)

type searchDetail struct {
	searchKeyWord    string
	expectedFinder   *nodewith.Finder
	searchResultView *nodewith.Finder
	verifiedNode     *nodewith.Finder
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchWebAndDrive,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Fixture:      "chromeLoggedWithGaia",
		Desc:         "App Launcher Search: Web, Google Drive",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"victor.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
	})
}

// SearchWebAndDrive tests that App Launcher Search: Web, Google Drive.
func SearchWebAndDrive(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}
	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to take keyboard: ", err)
	}
	defer kb.Close()

	defer func(ctx context.Context) {
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	}(cleanupCtx)

	// Cleanup the Drive folder in advance to avoid g-doc file name duplicated with existing file.
	testing.ContextLog(ctx, "Delete all files under 'Drive' folder")
	if err := deleteFilesFromDriveFolder(ctx, tconn, kb); err != nil {
		s.Fatal("Failed to delete all files on Google Drive")
	}

	testing.ContextLog(ctx, "Create a new document on Google Drive")
	if err := newGoogleDoc(ctx, cr, tconn, kb, ui, gDocFilename); err != nil {
		s.Fatal("Failed to create a new Google Document: ", err)
	}
	defer func(ctx context.Context) {
		if err := deletDocFromDriveFolder(ctx, tconn, kb); err != nil {
			testing.ContextLogf(ctx, "Failed to remove %q", gDocFilename)
		}
	}(cleanupCtx)

	for _, search := range map[launcherSearchTest]searchDetail{
		webBrowser: {
			searchKeyWord:    "CNN",
			expectedFinder:   nodewith.HasClass("ui/app_list/SearchResultView").First(),
			searchResultView: nodewith.Name("CNN").Role(role.ListBoxOption).HasClass("ui/app_list/SearchResultView"),
			verifiedNode:     nodewith.Name("CNN").Role(role.Heading),
		},
		gDoc: {
			searchKeyWord:    gDocFilename,
			expectedFinder:   nodewith.Name(fmt.Sprintf("%s, Files", gDocFilename)).First(),
			searchResultView: nodewith.Name(fmt.Sprintf("%s, Files", gDocFilename)).Role(role.ListBoxOption).HasClass("ui/app_list/SearchResultView").First(),
			verifiedNode:     nodewith.Name("ui_launcher_search - Google Docs").HasClass("Label"),
		},
	} {
		f := func(ctx context.Context, s *testing.State) {
			cleanupSubTestCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupSubTestCtx, s.OutDir(), s.HasError, cr, search.searchKeyWord)

			if err := searchByLauncher(ctx, tconn, kb, cr, ui, search); err != nil {
				s.Fatal("Failed to search by launcher: ", err)
			}
		}

		if !s.Run(ctx, search.searchKeyWord, f) {
			s.Errorf("Failed to run subtest: %q", search.searchKeyWord)
		}
	}
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

// searchByLauncher opens apps and web by launcher and close it.
func searchByLauncher(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, cr *chrome.Chrome, ui *uiauto.Context, detail searchDetail) error {
	// Low end DUTs frequently encounter an issue that DUTs cannot find the name of the files in Google Drive.
	// Even though we could find the files we want, the context usually exceeds.
	// Setting the timeout to be 1 minutes can prevent this issue.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := launcher.Open(tconn)(ctx); err != nil {
			return errors.Wrap(err, "failed to open launcher")
		}
		defer func(ctx context.Context) {
			kb.AccelAction("Esc")(ctx) // Close search content.
			kb.AccelAction("Esc")(ctx) // Close launcher.
			waitForShelfAnimationComplete(ctx, tconn)
		}(ctx)

		testing.ContextLogf(ctx, "Seaching %q from launcher", detail.searchKeyWord)
		if err := launcher.Search(tconn, kb, detail.searchKeyWord)(ctx); err != nil {
			return errors.Wrapf(err, "failed to search %q from launcher", detail.searchKeyWord)
		}

		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(detail.expectedFinder)(ctx); err != nil {
			if err2 := uiauto.Combine("clear search box",
				ui.LeftClick(nodewith.Name("Clear searchbox text").Role(role.Button)),
				ui.WithTimeout(5*time.Second).WaitUntilExists(nodewith.NameStartingWith("Search your device").Role(role.TextField)),
			)(ctx); err2 != nil {
				return errors.Wrap(err2, "failed to clear search box")
			}
			return errors.Wrapf(err, "failed to find %q from launcher", detail.searchKeyWord)
		}

		if err := ui.LeftClick(detail.searchResultView)(ctx); err != nil {
			return errors.Wrapf(err, "failed to click list box option of %q on Launcher", detail.searchKeyWord)
		}

		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(detail.verifiedNode)(ctx); err != nil {
			return errors.Wrapf(err, "failed to verify %q", detail.searchKeyWord)
		}

		return nil

	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return errors.Wrap(err, "failed to searh")
	}

	testing.ContextLogf(ctx, "Closing app: %q", detail.searchKeyWord)
	return closeAllWindows(ctx, tconn)
}

// deleteFilesFromDriveFolder deletes all files on My Drive.
func deleteFilesFromDriveFolder(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch File app")
	}
	defer files.Close(cleanupCtx)

	deleteBtn := nodewith.Name("Delete").Role(role.Button).HasClass("icon-button menu-button")
	confirmBtn := nodewith.Name("Delete").Role(role.Button).HasClass("cr-dialog-ok")
	confirmedMessage := nodewith.NameStartingWith("Are you sure you want to delete").Role(role.StaticText)

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("select all files in `Drive` via filesapp",
		files.OpenDrive(),
		kb.AccelAction("Ctrl+A"),
	)(ctx); err != nil {
		return err
	}

	if err := ui.WaitUntilExists(deleteBtn)(ctx); err != nil {
		testing.ContextLog(ctx, "Google drive is empty now")
		return nil
	}

	return uiauto.Combine("delete the files in 'Drive'",
		ui.LeftClick(deleteBtn),
		ui.WaitUntilExists(confirmedMessage),
		ui.LeftClickUntil(confirmBtn, ui.WithTimeout(5*time.Second).WaitUntilGone(confirmBtn)),
	)(ctx)
}

// deletDocFromDriveFolder cleans up the file created by this automation.
func deletDocFromDriveFolder(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	file, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open filesapp to delete")
	}
	defer file.Close(cleanupCtx)

	return uiauto.Combine("delete the file, 'MAT0108b_test'",
		file.OpenDrive(),
		file.DeleteFileOrFolder(kb, fmt.Sprintf("%s.gdoc", gDocFilename)),
	)(ctx)
}

// newGoogleDoc creates a new Google Doc and renames it.
func newGoogleDoc(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, ui *uiauto.Context, filename string) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	conn, err := cr.NewConn(ctx, "https://drive.google.com/drive/my-drive")
	if err != nil {
		return errors.Wrap(err, "failed to search with Google drive")
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)

	if err := webutil.WaitForQuiescence(ctx, conn, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait until the page is stable")
	}

	if err := uiauto.Combine("create doc",
		ui.LeftClick(nodewith.Name("New").Role(role.PopUpButton)),
		ui.LeftClick(nodewith.Name("Google Docs").Role(role.InlineTextBox)),
		ui.LeftClick(nodewith.Name("Document content").Role(role.TextField)),
	)(ctx); err != nil {
		return err
	}

	urlCurrent, err := activeTabURL(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the url of new tab")
	}

	testing.ContextLogf(ctx, "The url of untitled document: %s", urlCurrent)
	connNewGoogleDoc, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(urlCurrent))
	if err != nil {
		return errors.Wrap(err, "failed to get connection to new target")
	}
	defer connNewGoogleDoc.Close()

	if err := webutil.WaitForQuiescence(ctx, connNewGoogleDoc, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait until the page is stable")
	}

	if err := uiauto.Combine("rename the document",
		ui.LeftClick(nodewith.Name("Untitled document").Role(role.InlineTextBox).State(state.Editable, true)),
		kb.AccelAction("Ctrl+A"),
		kb.TypeAction(filename),
		kb.AccelAction("Enter"),
		ui.WaitUntilExists(nodewith.Name("Document status: Saved to Drive.")),
	)(ctx); err != nil {
		return err
	}

	return connNewGoogleDoc.CloseTarget(ctx)
}

// activeTabURL returns the URL of the active tab.
func activeTabURL(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var tabURL string
	if err := tconn.Call(ctx, &tabURL, `async () => {
                let tabs = await tast.promisify(chrome.tabs.query)({active: true});
                return tabs[0].url;
        }`); err != nil {
		return "", errors.Wrap(err, "active tab URL not found")
	}
	return tabURL, nil
}

// waitForShelfAnimationComplete waits for 1 seconds to shelf animation complete.
func waitForShelfAnimationComplete(ctx context.Context, tconn *chrome.TestConn) {
	testing.Poll(ctx, func(ctx context.Context) error {
		shelfInfo, err := ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
		if err != nil {
			return testing.PollBreak(err)
		}
		if shelfInfo.IsShelfWidgetAnimating {
			return errors.New("shelf is still animating")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Second})
}
