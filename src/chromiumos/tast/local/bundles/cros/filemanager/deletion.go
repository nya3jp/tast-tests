// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

type deletionTargetType uint8

const (
	emptyFolder deletionTargetType = 1 << iota
	nonEmptyFolder
	file
)

const testFile = "files_app_test.png"

func init() {
	testing.AddTest(&testing.Test{
		Func:         Deletion,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Ensure deletion of files & folders work fine",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"ting.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{testFile},
		Fixture:      "chromeLoggedIn",
		SearchFlags: []*testing.StringPair{{
			Key:   "feature_id",
			Value: "screenplay-2bf9ed18-db1b-4587-9aae-195121f2acae",
		}, {
			Key:   "feature_id",
			Value: "screenplay-4c745151-7307-4658-aa58-1bb97592b4a6",
		},
		},
	})
}

// Deletion deletes files and folders from Downloads & My files.
func Deletion(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

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

	targetsName := map[deletionTargetType]string{
		emptyFolder:    "Empty",
		nonEmptyFolder: "NonEmpty",
		file:           testFile,
	}

	myFilesPath, err := cryptohome.MyFilesPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get users MyFiles path: ", err)
	}

	dirPath := map[string]string{
		filesapp.Downloads: filepath.Join(myFilesPath, "Downloads"),
		filesapp.MyFiles:   myFilesPath,
	}

	res := &deletionTestResource{
		tconn:    tconn,
		kb:       kb,
		filesApp: filesApp,
		dataPath: s.DataPath(testFile),
	}

	for workingDir, workingDirPath := range dirPath {
		for _, test := range []struct {
			name        string
			testTargets deletionTargetType
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

				cleanupSubtestCtx := ctx
				ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
				defer cancel()

				targets := generateTestTargets(test.testTargets, targetsName, workingDirPath)
				defer cleanUp(cleanupSubtestCtx, targets)

				if err := setupFilesAndFolders(res, targets); err != nil {
					s.Fatal("Failed to set up files and folders: ", err)
				}

				if err := processForDeletion(ctx, res, targets); err != nil {
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

type deletionTargetDetail struct {
	name         string
	path         string
	isFolder     bool
	skipDeletion bool
}

type deletionTestResource struct {
	tconn    *chrome.TestConn
	kb       *input.KeyboardEventWriter
	filesApp *filesapp.FilesApp
	dataPath string
}

// generateTestTargets generates the test targets, returns the map of deletionTargetType and its detail.
func generateTestTargets(testTargets deletionTargetType, targetsName map[deletionTargetType]string,
	workingDirPath string) *map[deletionTargetType]*deletionTargetDetail {

	targets := map[deletionTargetType]*deletionTargetDetail{}
	needToDelete := func(targetType deletionTargetType) bool { return (testTargets & targetType) != 0 }
	if needToDelete(emptyFolder) {
		targets[emptyFolder] = &deletionTargetDetail{
			name:     targetsName[emptyFolder],
			path:     filepath.Join(workingDirPath, targetsName[emptyFolder]),
			isFolder: true,
		}
	}
	if needToDelete(nonEmptyFolder) {
		targets[nonEmptyFolder] = &deletionTargetDetail{
			name:     targetsName[nonEmptyFolder],
			path:     filepath.Join(workingDirPath, targetsName[nonEmptyFolder]),
			isFolder: true,
		}
		targets[nonEmptyFolder|file] = &deletionTargetDetail{
			name:         targetsName[file],
			path:         filepath.Join(workingDirPath, targetsName[nonEmptyFolder], targetsName[file]),
			skipDeletion: true, // The content of the folder will be deleted as the folder is deleted.
		}
	}
	if needToDelete(file) {
		targets[file] = &deletionTargetDetail{
			name: targetsName[file],
			path: filepath.Join(workingDirPath, targetsName[file]),
		}
	}
	return &targets
}

// waitUntilFileDeleted waits the target is deleted from system
// to prevents the deletion conflict at the cleanup step.
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

func processForDeletion(ctx context.Context, res *deletionTestResource, targets *map[deletionTargetType]*deletionTargetDetail) error {
	targetsName := make([]string, 0)
	for _, detail := range *targets {
		if detail.skipDeletion {
			continue
		}
		targetsName = append(targetsName, detail.name)
	}

	testing.ContextLog(ctx, "Trying to delete targets: ", targetsName)
	return res.filesApp.DeleteMultipleFilesOrFolders(res.kb, targetsName...)(ctx)
}

func cleanUp(ctx context.Context, targets *map[deletionTargetType]*deletionTargetDetail) {
	for _, target := range *targets {
		if err := os.RemoveAll(target.path); err != nil {
			testing.ContextLogf(ctx, "Failed to remove target %q: %v", target.path, err)
		}
	}
}

// setupFilesAndFolders prepares files and folders.
func setupFilesAndFolders(res *deletionTestResource, targets *map[deletionTargetType]*deletionTargetDetail) error {
	// Setup folders.
	for _, target := range *targets {
		if target.isFolder {
			if err := os.MkdirAll(target.path, 0755); err != nil {
				return errors.Wrap(err, "failed to create folder")
			}
			// The folders must be owned by `chronous` to ensure it can be deleted by `filesapp` through UI control.
			if err := os.Chown(target.path, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
				return errors.Wrapf(err, "failed to chown of folder: %q", target.path)
			}
		}
	}
	// Setup files.
	for _, target := range *targets {
		if !target.isFolder {
			if err := fsutil.CopyFile(res.dataPath, target.path); err != nil {
				return errors.Wrapf(err, "failed to copy file: %q to: %q", res.dataPath, target.path)
			}
		}
	}
	return nil
}
