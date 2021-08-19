// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/action"
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
	"chromiumos/tast/local/sysutil"
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
		Func: Deletion,
		Desc: "Ensure deletion of files & folders work fine",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"ting.chen@cienet.com",
		},
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

	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer filesApp.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

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
			{"nonEmptyFolder", nonEmptyFolder},
			{"emptyFolder", emptyFolder},
			{"oneFile", file},
			{"multiple", emptyFolder | nonEmptyFolder | file},
		} {
			subTest := func(ctx context.Context, s *testing.State) {
				if err := filesApp.OpenDir(workingDir, filesapp.FilesTitlePrefix+workingDir)(ctx); err != nil {
					s.Fatalf("Failed to open directory %q: %v", workingDir, err)
				}

				testTargets := targetsToDelete(test.target, nameMap)
				cleanUp, err := setupFilesAndFolders(ctx, tconn, kb, filesApp, testTargets, nameMap, dirPath[workingDir], s.DataPath(testFile))
				defer cleanUp()
				if err != nil {
					s.Fatal("Failed to set up files and folders: ", err)
				}

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
		targets = append(targets, nameMap[file])
	}
	return targets
}

// waitUntilFileDeleted prevents the deletion conflict at the cleanup step.
func waitUntilFileDeleted(path string) action.Action {
	return func(ctx context.Context) error {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				return errors.New("file still exists")
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return err
		}
		return nil
	}
}

func processForDeletion(ctx context.Context, kb *input.KeyboardEventWriter, filesApp *filesapp.FilesApp, targets []string) error {
	actions := make([]uiauto.Action, 0)
	for _, target := range targets {
		actions = append(actions, filesApp.WaitUntilFileGone(target))
		actions = append(actions, waitUntilFileDeleted(target))
	}
	verifyTargetsAreGone := uiauto.Combine("verify all targets are gone", actions...)

	testing.ContextLog(ctx, "Trying to delete targets: ", targets)
	deleteBtn := nodewith.Name("Delete").HasClass("cr-dialog-ok").Role(role.Button)
	return uiauto.Combine(fmt.Sprintf("select and delete targets: %v", targets),
		filesApp.SelectMultipleFiles(kb, targets...),
		kb.AccelAction("Alt+Backspace"),
		filesApp.LeftClick(deleteBtn),
		filesApp.WaitUntilGone(deleteBtn),
		verifyTargetsAreGone,
	)(ctx)
}

// setupFilesAndFolders prepares files and folders under working dir.
func setupFilesAndFolders(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter,
	filesApp *filesapp.FilesApp, targets []string, nameMap map[deletionTestTarget]string, workingDirPath, dataPath string) (cleanUp func(), err error) {
	cleanTargets := make([]string, 0)
	cleanUp = func() {
		for _, target := range cleanTargets {
			os.RemoveAll(target)
		}
	}

	var path string
	for _, target := range targets {
		testing.ContextLogf(ctx, "Create target: %s", target)

		if target == nameMap[file] {
			path = filepath.Join(workingDirPath, testFile)
			if err := fsutil.CopyFile(dataPath, path); err != nil {
				return cleanUp, errors.Wrapf(err, "failed to copy file to folder %s", path)
			}
		} else {
			path = filepath.Join(workingDirPath, target)
			if err := os.MkdirAll(path, 0755); err != nil {
				return cleanUp, errors.Wrap(err, "failed to create folder")
			}

			// The folders must be owned by `chronous` to ensure it can be deleted by `filesapp` through UI control.
			if err := os.Chown(path, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
				return cleanUp, errors.Wrap(err, "failed to chown dir")
			}
		}
		cleanTargets = append(cleanTargets, path)

		if target == nameMap[nonEmptyFolder] {
			path = filepath.Join(workingDirPath, target, testFile)
			if err := fsutil.CopyFile(dataPath, path); err != nil {
				return cleanUp, errors.Wrapf(err, "failed to copy file to folder %s", path)
			}
		}

		if err := filesApp.WaitForFile(target)(ctx); err != nil {
			return cleanUp, errors.Wrap(err, "failed to wait until all files exist")
		}
	}
	return cleanUp, nil
}
