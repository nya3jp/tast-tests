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

type deletionTestTarget uint8

const (
	emptyFolder deletionTestTarget = 1 << iota
	nonEmptyFolder
	file
)

type deletionTestParams struct {
	workingDir      string
	targetsToDelete deletionTestTarget
}

const (
	myfiles          = "My files"
	downloads        = "Downloads"
	testFile         = "files_app_test.png"
	filesTitlePrefix = "Files - "
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Deletion,
		Desc:         "Ensure deletion of files & folders work fine",
		Contacts:     []string{"ting.chen@cienet.com", "cienet-development@googlegroups.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{testFile},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{
			{
				Name: "myfiles_one_file",
				Val: deletionTestParams{
					workingDir:      myfiles,
					targetsToDelete: file,
				},
			}, {
				Name: "myfiles_empty_folder",
				Val: deletionTestParams{
					workingDir:      myfiles,
					targetsToDelete: emptyFolder,
				},
			}, {
				Name: "myfiles_nonempty_folder",
				Val: deletionTestParams{
					workingDir:      myfiles,
					targetsToDelete: nonEmptyFolder,
				},
			}, {
				Name: "myfiles_multiple",
				Val: deletionTestParams{
					workingDir:      myfiles,
					targetsToDelete: emptyFolder | nonEmptyFolder | file,
				},
			}, {
				Name: "download_one_file",
				Val: deletionTestParams{
					workingDir:      downloads,
					targetsToDelete: file,
				},
			}, {
				Name: "download_empty_folder",
				Val: deletionTestParams{
					workingDir:      downloads,
					targetsToDelete: emptyFolder,
				},
			}, {
				Name: "download_nonempty_folder",
				Val: deletionTestParams{
					workingDir:      downloads,
					targetsToDelete: nonEmptyFolder,
				},
			}, {
				Name: "download_multiple",
				Val: deletionTestParams{
					workingDir:      downloads,
					targetsToDelete: emptyFolder | nonEmptyFolder | file,
				},
			},
		},
	})
}

// Deletion deletes files and folders from Downloads & My files.
func Deletion(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	param := s.Param().(deletionTestParams)

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
		s.Fatal("Failed to create test API connection: ", err)
	}

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(cleanupCtx)

	targetName := map[deletionTestTarget]string{
		emptyFolder:    "Empty",
		nonEmptyFolder: "NonEmpty",
		file:           s.DataPath(testFile),
	}

	cleanUp, err := setupFileAndFolders(ctx, tconn, kb, &param, files, targetName)
	if err != nil {
		s.Fatal("Failed to set up file and folders: ", err)
	}
	defer cleanUp()
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	targets := make([]string, 0)
	needToDelete := func(target deletionTestTarget) bool { return (param.targetsToDelete & target) != 0 }
	if needToDelete(emptyFolder) {
		targets = append(targets, targetName[emptyFolder])
	}
	if needToDelete(nonEmptyFolder) {
		targets = append(targets, targetName[nonEmptyFolder])
	}
	if needToDelete(file) {
		targets = append(targets, testFile)
	}

	actions := make([]uiauto.Action, 0)
	for _, target := range targets {
		actions = append(actions, files.WaitUntilFileGone(target))
	}
	verifyTargetsAreGone := uiauto.Combine("verify all target are gone", actions...)

	testing.ContextLog(ctx, "Trying to delete targets: ", targets)
	if len(targets) == 1 {
		target := targets[0]
		if err := uiauto.Combine("delete single target",
			files.WithTimeout(10*time.Second).WaitForFile(target),
			files.DeleteFileOrFolder(kb, target),
			verifyTargetsAreGone,
		)(ctx); err != nil {
			s.Fatalf("Failed to delete %s: %v", target, err)
		}
		return
	}

	if err := uiauto.Combine("select all and delete",
		files.SelectMultipleFiles(kb, targets...),
		kb.AccelAction("Alt+Backspace"),
		files.LeftClick(nodewith.Name("Delete").HasClass("cr-dialog-ok").Role(role.Button)),
		verifyTargetsAreGone,
	)(ctx); err != nil {
		s.Fatal("Failed to delete multiple targets: ", err)
	}
}

// setupFileAndFolders prepares files and folders under working dir.
func setupFileAndFolders(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, param *deletionTestParams, files *filesapp.FilesApp, targetName map[deletionTestTarget]string) (cleanUp func(), err error) {
	dirmap := map[string]string{
		downloads: filesapp.DownloadPath,
		myfiles:   filesapp.MyFilesPath,
	}

	dirName := param.workingDir
	dirPath := dirmap[dirName]

	cleanUp = func() {
		for _, targetPath := range []string{
			filepath.Join(dirPath, testFile),
			filepath.Join(dirPath, targetName[nonEmptyFolder], testFile),
			filepath.Join(dirPath, targetName[emptyFolder]),
			filepath.Join(dirPath, targetName[nonEmptyFolder]),
		} {
			os.Remove(targetPath)
		}
	}

	testing.ContextLogf(ctx, "Open dir %s", dirName)
	if err := uiauto.Combine("open dir",
		files.OpenDir(dirName, filesTitlePrefix+dirName),
		files.LeftClick(nodewith.Role(role.ListBox)),
	)(ctx); err != nil {
		return cleanUp, errors.Wrap(err, "failed to open directory")
	}

	testing.ContextLog(ctx, "Create folders")
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("create empty and nonempty folders",
		ui.IfSuccessThen(
			files.FileGone(targetName[emptyFolder]),
			files.CreateFolder(kb, targetName[emptyFolder]),
		),
		ui.IfSuccessThen(
			files.FileGone(targetName[nonEmptyFolder]),
			files.CreateFolder(kb, targetName[nonEmptyFolder]),
		),
	)(ctx); err != nil {
		return cleanUp, errors.Wrap(err, "failed to create folders")
	}

	testing.ContextLog(ctx, "Copy file into non-empty folder")
	path := filepath.Join(dirPath, targetName[nonEmptyFolder], testFile)
	if err := fsutil.CopyFile(targetName[file], path); err != nil {
		return cleanUp, errors.Wrapf(err, "failed to copy file to folder %s", path)
	}

	path = filepath.Join(dirPath, testFile)
	if err := fsutil.CopyFile(targetName[file], path); err != nil {
		return cleanUp, errors.Wrapf(err, "failed to copy file to folder %s", path)
	}

	return cleanUp, nil
}
