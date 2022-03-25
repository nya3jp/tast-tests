// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const testImage = "files_app_test.png"

func init() {
	testing.AddTest(&testing.Test{
		Func:         RecentFilesAppear,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Check the edited files are shown in Recent tab",
		Contacts: []string{
			"tim.chang@cienet.com",
			"ting.chen@cienet.com",
			"wenbojie@chromium.org",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{testImage},
		Params: []testing.Param{{
			Val: false, // #files-filters-in-recents flag is off.
		}, {
			Name: "filters_in_recents_on",
			Val:  true, // #files-filter-in-recents flag is on.
		}},
	})
}

// RecentFilesAppear checks the edited files are shown in Recent tab.
func RecentFilesAppear(ctx context.Context, s *testing.State) {
	filtersInRecentsEnabled := s.Param().(bool)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Enable or disable the flag #files-filters-in-recents
	var chromeOpts []chrome.Option
	if filtersInRecentsEnabled {
		chromeOpts = append(chromeOpts, chrome.EnableFeatures("FiltersInRecents"))
	} else {
		chromeOpts = append(chromeOpts, chrome.DisableFeatures("FiltersInRecents"))
	}

	// Start Chrome.
	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Get Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files App: ", err)
	}
	defer files.Close(cleanupCtx)

	testPath := s.DataPath(testImage)

	for _, subtest := range []struct {
		dirName string
		dirPath string
	}{
		{filesapp.Downloads, filesapp.DownloadPath},
		{filesapp.MyFiles, filesapp.MyFilesPath},
	} {
		f := func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()

			imageFileLocation := filepath.Join(subtest.dirPath, testImage)
			if err := fsutil.CopyFile(testPath, imageFileLocation); err != nil {
				s.Fatalf("Failed to copy the test image to %s: %v", imageFileLocation, err)
			}
			defer os.Remove(imageFileLocation)

			dumpName := "ui_" + strings.ReplaceAll(subtest.dirName, " ", "")
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, dumpName)

			ui := uiauto.New(tconn)
			if err := prepareFile(ctx, ui, files, testImage, subtest.dirPath); err != nil {
				s.Fatal("Failed to prepare test file: ", err)
			}

			if err := files.OpenDir(subtest.dirName, filesapp.FilesTitlePrefix+subtest.dirName)(ctx); err != nil {
				s.Fatal("Failed to open the directory: ", err)
			}

			if err := editImage(ctx, tconn, files, testImage); err != nil {
				s.Fatal("Failed to edit the file: ", err)
			}

			testing.ContextLog(ctx, "Refresh until file exists in Recent")
			if err := ui.RetryUntil(
				refreshRecent(files),
				files.FileExists(testImage),
			)(ctx); err != nil {
				s.Fatal("Failed to find file in recent: ", err)
			}

			testing.ContextLog(ctx, "Refresh until file exists in Recent Images")
			if err := ui.RetryUntil(
				goToRecentImages(files, filtersInRecentsEnabled),
				files.FileExists(testImage),
			)(ctx); err != nil {
				s.Fatal("Failed to find file in recent images: ", err)
			}
		}

		if !s.Run(ctx, subtest.dirName, f) {
			s.Errorf("Failed to run test in %q", subtest.dirName)
		}
	}
}

func prepareFile(ctx context.Context, ui *uiauto.Context, files *filesapp.FilesApp, filename, folderPath string) error {
	aYearBefore := time.Now().Local().AddDate(-1, 0, 0)
	formatedTime := aYearBefore.Format("200601021504")

	// Change the modified date to ensure the file is not shown in Recent tab.
	filePath := filepath.Join(folderPath, filename)
	if _, err := testexec.CommandContext(ctx, "touch", "-t", formatedTime, filePath).Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run command to change edit date")
	}

	testing.ContextLog(ctx, "Refresh until file gone in Recent")
	return ui.RetryUntil(
		refreshRecent(files),
		files.WithTimeout(2*time.Second).WaitUntilFileGone(filename),
	)(ctx)
}

func editImage(ctx context.Context, tconn *chrome.TestConn, files *filesapp.FilesApp, filename string) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	ui := uiauto.New(tconn)
	gallery := nodewith.NameStartingWith(apps.Gallery.Name).HasClass("BrowserFrame")
	saveBtn := nodewith.Name("Save").Role(role.Button)

	if err := uiauto.Combine("open image in Gallery",
		files.OpenFile(filename),
		ui.WaitUntilExists(gallery),
	)(ctx); err != nil {
		return err
	}
	defer func(ctx context.Context) {
		if err := apps.Close(ctx, tconn, apps.Gallery.ID); err != nil {
			testing.ContextLog(ctx, "Failed to close gallery: ", err)
		}
		if err := ui.WithTimeout(5 * time.Second).WaitUntilGone(gallery)(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to wait until gallery gone: ", err)
		}
	}(cleanupCtx)

	return uiauto.Combine("edit image in Gallery",
		ui.LeftClick(nodewith.Name("Crop & rotate").Role(role.ToggleButton)),
		ui.LeftClick(nodewith.Name("16:9").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Done").Role(role.Button)),
		ui.LeftClick(saveBtn),
		ui.WaitUntilGone(saveBtn),
	)(ctx)
}

// refreshRecent refresh the recent page by switching between directories.
func refreshRecent(files *filesapp.FilesApp) uiauto.Action {
	return uiauto.Combine("refresh recent tab",
		files.OpenDownloads(),
		files.OpenDir(filesapp.Recent, filesapp.FilesTitlePrefix+filesapp.Recent),
	)
}

// goToRecentImages navigate to the recent image view by:
//  * opening the Images menu when flag is off.
//  * opening the Recent menu and clicking the Images filter button when flag is on.
func goToRecentImages(files *filesapp.FilesApp, isFlagOn bool) uiauto.Action {
	if isFlagOn {
		return uiauto.Combine("go to recent images by filter button",
			files.OpenDir(filesapp.Recent, filesapp.FilesTitlePrefix+filesapp.Recent),
			files.LeftClick(nodewith.Name("Images").Role(role.ToggleButton)),
		)
	}
	return files.OpenDir(filesapp.Images, filesapp.FilesTitlePrefix+filesapp.Images)
}
