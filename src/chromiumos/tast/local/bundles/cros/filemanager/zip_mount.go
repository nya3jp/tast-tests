// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ZipMount,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Files App can mount archives (ZIP, RAR, 7Z...)",
		Contacts: []string{
			"fdegros@chromium.org",
			"jboulic@chromium.org",
			"msalomao@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      8 * time.Minute,
		Data: []string{
			"Smile 😀.txt.bz2",
			"Smile 😀.txt.gz",
			"Smile 😀.txt.lz",
			"Smile 😀.txt.lzma",
			"Smile 😀.txt.xz",
			"Smile 😀.txt.Z",
			"Smile 😀.txt.zst",
			"Texts.zip",
			"Texts.rar",
			"Texts.7z",
			"Texts.crx",
			"Texts.iso",
			"Texts.tar",
			"Texts.tar.bz2",
			"Texts.tar.bz",
			"Texts.tbz2",
			"Texts.tbz",
			"Texts.tb2",
			"Texts.tz2",
			"Texts.tar.gz",
			"Texts.tgz",
			"Texts.tar.lz",
			"Texts.tlz",
			"Texts.tar.lzma",
			"Texts.tlzma",
			"Texts.tar.xz",
			"Texts.txz",
			"Texts.tar.zst",
			"Texts.tzst",
			"Texts.tar.Z",
			"Texts.taZ",
			"Texts.tZ",
		},
	})
}

func ZipMount(ctx context.Context, s *testing.State) {
	// TODO(crbug.com/1326797) Remove once it is enabled by default.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("FilesArchivemount2"))
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Get Downloads folder path.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Cannot get Downloads folder path: ", err)
	}

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Cannot connect to test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Open the Files App.
	app, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Cannot launch Files App: ", err)
	}

	type TestCase struct {
		Archives     []string
		WantContents []string
	}

	tcs := []TestCase{{
		Archives: []string{
			"Texts.zip",
			"Texts.rar",
			"Texts.7z",
			"Texts.crx",
			"Texts.iso",
			"Texts.tar",
			"Texts.tar.bz2",
			"Texts.tar.bz",
			"Texts.tbz2",
			"Texts.tbz",
			"Texts.tb2",
			"Texts.tz2",
			"Texts.tar.gz",
			"Texts.tgz",
			"Texts.tar.lz",
			"Texts.tlz",
			"Texts.tar.lzma",
			"Texts.tlzma",
			"Texts.tar.xz",
			"Texts.txz",
			"Texts.tar.zst",
			"Texts.tzst",
			"Texts.tar.Z",
			"Texts.taZ",
			"Texts.tZ",
		},
		WantContents: []string{"Texts"},
	}, {
		Archives: []string{
			"Smile 😀.txt.bz2",
			"Smile 😀.txt.gz",
			"Smile 😀.txt.lz",
			"Smile 😀.txt.lzma",
			"Smile 😀.txt.xz",
			"Smile 😀.txt.Z",
			"Smile 😀.txt.zst",
		},
		WantContents: []string{"Smile 😀.txt"},
	}}

	for _, tc := range tcs {
		for _, archive := range tc.Archives {
			s.Logf("Testing archive %q", archive)

			// Copy archive into Downloads folder.
			archiveLocation := filepath.Join(downloadsPath, archive)
			if err := fsutil.CopyFile(s.DataPath(archive), archiveLocation); err != nil {
				s.Fatalf("Cannot copy %q to %q: %v", archive, downloadsPath, err)
			}

			// Mount, check and unmount the archive.
			if err := testArchive(ctx, app, archive, tc.WantContents); err != nil {
				s.Errorf("Error with archive %q: %v", archive, err)
			}

			// Remove the archive file from Downloads folder.
			if err := os.Remove(archiveLocation); err != nil {
				s.Errorf("Cannot remove archive %q: %v", archiveLocation, err)
			}
		}
	}
}

// testArchive mounts, checks and unmounts a single archive file located
// in the Downloads folder.
func testArchive(ctx context.Context, app *filesapp.FilesApp, archive string, wantContents []string) error {
	// Open the Downloads folder.
	if err := app.OpenDownloads()(ctx); err != nil {
		return errors.Wrap(err, "cannot open Downloads folder")
	}

	// Select archive.
	if err := uiauto.Combine("wait for test archive and select",
		app.WithTimeout(5*time.Second).WaitForFile(archive),
		app.SelectFile(archive),
	)(ctx); err != nil {
		return errors.Wrapf(err, "cannot select archive %q", archive)
	}

	// Wait for Open button in the top bar.
	open := nodewith.Name("Open").Role(role.Button)
	if err := uiauto.Combine("find and click Open menu item",
		app.WithTimeout(5*time.Second).WaitUntilExists(open),
		app.LeftClick(open),
	)(ctx); err != nil {
		return errors.Wrapf(err, "cannot mount archive %q", archive)
	}

	// Find and open the mounted archive.
	archiveNode := nodewith.Name(archive).Role(role.TreeItem)
	if err := uiauto.Combine("find and click tree item",
		app.WithTimeout(5*time.Second).WaitUntilExists(archiveNode),
		app.LeftClick(archiveNode),
	)(ctx); err != nil {
		return errors.Wrapf(err, "cannot open mounted archive %q", archive)
	}

	// Ensure that the Files App is displaying the content of the mounted archive.
	rootWebArea := nodewith.Name("Files - " + archive).Role(role.RootWebArea)
	if err := app.WithTimeout(5 * time.Second).WaitUntilExists(rootWebArea)(ctx); err != nil {
		return errors.Wrapf(err, "cannot see content of mounted archive %q", archive)
	}

	// Check contents of mounted archive.
	for _, want := range wantContents {
		label := nodewith.Name(want).Role(role.ListBoxOption)
		if err := app.WithTimeout(5 * time.Second).WaitUntilExists(label)(ctx); err != nil {
			return errors.Wrapf(err, "cannot see %q in mounted archive %q", want, archive)
		}
	}

	// Find the Eject button within the appropriate tree item.
	ejectButton := nodewith.Name("Eject device").Role(role.Button).Ancestor(archiveNode)
	if err := uiauto.Combine("find and click eject button - "+archive,
		app.WithTimeout(5*time.Second).WaitUntilExists(ejectButton),
		app.LeftClick(ejectButton),
	)(ctx); err != nil {
		return errors.Wrapf(err, "cannot find Eject button of mounted archive %q", archive)
	}

	// Check that the tree item corresponding to the previously mounted archive
	// was removed.
	if err := app.WithTimeout(5 * time.Second).WaitUntilGone(archiveNode)(ctx); err != nil {
		return errors.Wrapf(err, "cannot eject mounted archive %q", archive)
	}

	return nil
}
