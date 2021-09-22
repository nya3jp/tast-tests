// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SearchDrive,
		Desc: "Test Google Drive search feature on file manager app",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"lance.wang@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

// SearchDrive verifies Google Drive search on Files app.
func SearchDrive(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// To perform Google Drive search on Files, login authentically is required.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	// Launch Files App and check that Drive is accessible.
	fa, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer fa.Close(cleanupCtx)

	defer func(ctx context.Context) {
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	}(cleanupCtx)

	if err := fa.OpenDrive()(ctx); err != nil {
		s.Fatal("Failed to navigate to Google Drive folder: ", err)
	}

	infos, err := getExistingFolders(ctx, tconn, fa)
	if err != nil {
		s.Fatal("Failed to get existing folders: ", err)
	}

	const foldersCnt = 5

	var (
		prefix          = "filemanager_search_drive_test-"
		folderName      = func(i int) string { return fmt.Sprintf("%s%02d", prefix, i) }
		allFoldersGone  = make([]uiauto.Action, 0)
		allFoldersExist = make([]uiauto.Action, 0)
	)

	// Create folders.
	for i := 0; i < foldersCnt; i++ {
		folder := folderName(i)
		defer func(ctx context.Context) {
			if err := fa.DeleteFileOrFolder(kb, folder)(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to delete folder: ", err)
			}
		}(cleanupCtx)

		allFoldersGone = append(allFoldersGone, fa.WaitUntilFileGone(folder))
		allFoldersExist = append(allFoldersExist, fa.WaitForFile(folder))

		if err := folderNotExist(infos, folder); err != nil {
			testing.ContextLogf(ctx, "Folder %q exist", folder)
			continue
		}

		testing.ContextLogf(ctx, "Create folder %q", folder)
		if err := fa.CreateFolder(kb, folder)(ctx); err != nil {
			s.Fatalf("Failed to create folder %q: %v", folder, err)
		}
	}

	mismatchKeyword := folderName(foldersCnt)
	if err := searchAndVerify(fa, kb, mismatchKeyword, allFoldersGone)(ctx); err != nil {
		s.Fatalf("Failed to search %q and verify: %v", mismatchKeyword, err)
	}

	if err := searchAndVerify(fa, kb, prefix, allFoldersExist)(ctx); err != nil {
		s.Fatalf("Failed to search %q and verify: %v", prefix, err)
	}
}

func searchAndVerify(fa *filesapp.FilesApp, kb *input.KeyboardEventWriter, keyword string, verify []uiauto.Action) uiauto.Action {
	actions := make([]uiauto.Action, 0)

	// Search action.
	actions = append(actions,
		fa.ClearSearch(),
		fa.Search(kb, keyword),
	)

	// Verify action.
	actions = append(actions, verify...)

	// Leave search mode.
	actions = append(actions, fa.ClearSearch())

	return uiauto.Combine("search and verify", actions...)
}

func getExistingFolders(ctx context.Context, tconn *chrome.TestConn, fa *filesapp.FilesApp) ([]uiauto.NodeInfo, error) {
	filesBox := nodewith.Role(role.ListBox)
	items := nodewith.Role(role.StaticText).Ancestor(filesBox)

	if err := fa.WaitUntilExists(items.First())(ctx); err != nil {
		return []uiauto.NodeInfo{}, nil
	}

	return fa.NodesInfo(ctx, items)
}

func folderNotExist(allNodes []uiauto.NodeInfo, folder string) error {
	for _, info := range allNodes {
		if folder == info.Name {
			return errors.New("folder exist")
		}
	}
	return nil
}
