// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

const testFile = "files_app_test.png"

func init() {
	testing.AddTest(&testing.Test{
		Func:         Deletion,
		Desc:         "Ensure deletion of files & folders work fine",
		Contacts:     []string{"ting.chen@cienet.com", "cienet-development@googlegroups.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{testFile},
		Fixture:      "chromeLoggedIn",
	})
}

// Deletion deletes files and folders from Downloads & My files.
func Deletion(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

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

	nameMap := map[deletionTestTarget]string{
		emptyFolder:    "Empty",
		nonEmptyFolder: "NonEmpty",
		file:           testFile,
	}

	dirPath := map[string]string{
		filesapp.Downloads: filesapp.DownloadPath,
		filesapp.MyFiles:   filesapp.MyFilesPath,
	}

	for _, workingDir := range []string{
		filesapp.Downloads,
		filesapp.MyFiles,
	} {
		for _, test := range []struct {
			name   string
			target deletionTestTarget
		}{
			{"emptyFolder", emptyFolder},
			{"nonEmptyFolder", nonEmptyFolder},
			{"oneFile", file},
			{"multiple", emptyFolder | nonEmptyFolder | file},
		} {
			subTest := func(ctx context.Context, s *testing.State) {
				filesApp, err := filesapp.Launch(ctx, tconn)
				if err != nil {
					s.Fatal("Failed to launch the Files App: ", err)
				}
				defer filesApp.Close(cleanupCtx)
				defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

				if err := filesApp.OpenDir(workingDir, filesapp.FilesTitlePrefix+workingDir)(ctx); err != nil {
					s.Fatalf("Failed to open directory %q: %v", workingDir, err)
				}

				if err := setupFilesAndFolders(ctx, tconn, kb, filesApp, nameMap, dirPath[workingDir], s.DataPath(testFile)); err != nil {
					s.Fatal("Failed to set up files and folders: ", err)
				}

				testTargets := targetsToDelete(test.target, nameMap)
				if err := processForDeletion(ctx, kb, filesApp, testTargets); err != nil {
					s.Fatal("Failed to do deletion: ", err)
				}
			}

			testName := strings.ReplaceAll(workingDir+"_"+test.name, " ", "")
			if !s.Run(ctx, testName, subTest) {
				// Stop if any sub-test failed.
				s.Fatalf("Failed to run subtest %s", testName)
			}
		}
	}
}

func targetsToDelete(testTargets deletionTestTarget, nameMap map[deletionTestTarget]string) []string {
	targets := make([]string, 0)
	needToDelete := func(testType deletionTestTarget) bool { return (testTargets & testType) != 0 }
	if needToDelete(emptyFolder) {
		targets = append(targets, nameMap[emptyFolder])
	}
	if needToDelete(nonEmptyFolder) {
		targets = append(targets, nameMap[nonEmptyFolder])
	}
	if needToDelete(file) {
		targets = append(targets, testFile)
	}
	return targets
}

func processForDeletion(ctx context.Context, kb *input.KeyboardEventWriter, filesApp *filesapp.FilesApp, targets []string) error {
	actions := make([]uiauto.Action, 0)
	for _, target := range targets {
		actions = append(actions, filesApp.WaitUntilFileGone(target))
	}
	verifyTargetsAreGone := uiauto.Combine("verify all targets are gone", actions...)

	testing.ContextLog(ctx, "Trying to delete targets: ", targets)
	if len(targets) == 1 {
		target := targets[0]
		return uiauto.Combine("delete single target",
			filesApp.WithTimeout(10*time.Second).WaitForFile(target),
			filesApp.DeleteFileOrFolder(kb, target),
			verifyTargetsAreGone,
		)(ctx)
	}

	return uiauto.Combine("select all and delete",
		filesApp.SelectMultipleFiles(kb, targets...),
		kb.AccelAction("Alt+Backspace"),
		filesApp.LeftClick(nodewith.Name("Delete").HasClass("cr-dialog-ok").Role(role.Button)),
		verifyTargetsAreGone,
	)(ctx)
}

// setupFilesAndFolders prepares files and folders under working dir.
func setupFilesAndFolders(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter,
	filesApp *filesapp.FilesApp, nameMap map[deletionTestTarget]string, workingDirPath, dataPath string) error {

	folderNotExist := func(path string) uiauto.Action {
		return func(ctx context.Context) error {
			if _, err := os.Stat(path); err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			return errors.New("folder exist")
		}
	}

	testing.ContextLog(ctx, "Create folders")
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("create empty and nonempty folders",
		ui.IfSuccessThen(
			folderNotExist(filepath.Join(workingDirPath, nameMap[emptyFolder])),
			filesApp.CreateFolder(kb, nameMap[emptyFolder]),
		),
		ui.IfSuccessThen(
			folderNotExist(filepath.Join(workingDirPath, nameMap[nonEmptyFolder])),
			filesApp.CreateFolder(kb, nameMap[nonEmptyFolder]),
		),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to create folders")
	}

	path := filepath.Join(workingDirPath, nameMap[nonEmptyFolder], nameMap[file])
	if err := fsutil.CopyFile(dataPath, path); err != nil {
		return errors.Wrapf(err, "failed to copy file to folder %s", path)
	}

	path = filepath.Join(workingDirPath, nameMap[file])
	if err := fsutil.CopyFile(dataPath, path); err != nil {
		return errors.Wrapf(err, "failed to copy file to folder %s", path)
	}

	return nil
}
