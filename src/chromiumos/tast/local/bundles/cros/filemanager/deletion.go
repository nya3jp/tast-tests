// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const testFile = "files_app_test.png"

func init() {
	testing.AddTest(&testing.Test{
		Func:         Deletion,
		Desc:         "Ensure deletion of files & folders work fine",
		Contacts:     []string{"ting.chen@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{testFile},
		Fixture:      "chromeLoggedIn",
	})
}

const (
	filesTitlePrefix = "Files - "
	emptyFolder      = "Empty"
	nonEmptyFolder   = "NonEmpty"
)

// Deletion delete files and folders from Downloads & My files.
func Deletion(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	testFilePath := s.DataPath(testFile)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer tconn.Close()

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	defer files.Close(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	if err := processForDeletion(ctx, "My files", filesapp.MyFilesPath, testFilePath, files, kb); err != nil {
		s.Fatal("Failed to do deletion in 'My files': ", err)
	}

	if err := processForDeletion(ctx, "Downloads", filesapp.DownloadPath, testFilePath, files, kb); err != nil {
		s.Fatal("Failed to do deletion in 'Downloads': ", err)
	}
}

func processForDeletion(ctx context.Context, dirName, dstPath, testFilePath string, files *filesapp.FilesApp, kb *input.KeyboardEventWriter) error {
	fileInDir := filepath.Join(dstPath, testFile)
	testing.ContextLog(ctx, "Copy file to ", fileInDir)
	if err := fsutil.CopyFile(testFilePath, fileInDir); err != nil {
		return errors.Wrapf(err, "failed to copy the test file to %q", fileInDir)
	}
	// Restore DUT's status in case the deletion fails.
	defer os.Remove(fileInDir)

	folders := []string{
		emptyFolder,
		nonEmptyFolder,
	}
	for _, folder := range folders {
		testing.ContextLog(ctx, "Open dir ", dirName)
		if err := uiauto.Combine("open dir",
			files.OpenDir(dirName, filesTitlePrefix+dirName),
			files.LeftClick(nodewith.Role(role.ListBox)),
		)(ctx); err != nil {
			return err
		}

		if err := files.FileExists(folder)(ctx); err != nil {
			testing.ContextLog(ctx, "Create folder ", folder)
			if err := files.CreateFolder(kb, folder)(ctx); err != nil {
				return err
			}
		}
	}

	fileInNonEmpty := filepath.Join(dstPath, nonEmptyFolder, testFile)
	testing.ContextLog(ctx, "Copy file to ", fileInNonEmpty)
	if err := fsutil.CopyFile(testFilePath, fileInNonEmpty); err != nil {
		return errors.Wrapf(err, "failed to copy the test file to %q", fileInNonEmpty)
	}
	// Restore DUT's status in case the deletion fails.
	defer os.Remove(fileInNonEmpty)

	testing.ContextLog(ctx, "Delete files and folders under ", dirName)
	return uiauto.Combine("delete files and folders",
		files.OpenDir(dirName, filesTitlePrefix+dirName),
		files.WithTimeout(10*time.Second).WaitForFile(testFile),
		files.DeleteFileOrFolder(kb, testFile),
		files.DeleteFileOrFolder(kb, emptyFolder),
		files.DeleteFileOrFolder(kb, nonEmptyFolder),
	)(ctx)
}
