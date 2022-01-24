// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

var fileManagerArchiveTestFiles = []string{
	"Encrypted_AES-256.zip",
	"Encrypted_ZipCrypto.zip",
	"Texts.7z",
	"Texts.rar",
	"Texts.zip",
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Parent:          "chromeLoggedInWithArchiveMount",
		Name:            "chromeLoggedInForFileManager",
		Desc:            "Logged into a user session with test files copied over",
		Contacts:        []string{"chromeos-files-syd@google.com"},
		Impl:            &loggedInFixtureForFileManager{},
		Data:            fileManagerArchiveTestFiles,
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PreTestTimeout:  chrome.ResetTimeout,
		PostTestTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Parent:          "chromeLoggedInGuestWithArchiveMount",
		Name:            "chromeLoggedInGuestForFileManager",
		Desc:            "Logged into a guest user session with test files copied over",
		Contacts:        []string{"chromeos-files-syd@google.com"},
		Impl:            &loggedInFixtureForFileManager{},
		Data:            fileManagerArchiveTestFiles,
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PreTestTimeout:  chrome.ResetTimeout,
		PostTestTimeout: chrome.ResetTimeout,
	})
}

// loggedInFixtureForFileManager is a fixture to start Chrome with a fixed set of
// files copied over for file manager tests.
type loggedInFixtureForFileManager struct {
	files *filesapp.FilesApp
	tconn *chrome.TestConn
}

func (f *loggedInFixtureForFileManager) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr := s.ParentValue().(*chrome.Chrome)

	// Delete all test files if SetUp fails
	success := false
	defer func() {
		if !success {
			os.RemoveAll(filesapp.DownloadPath)
		}
	}()

	// Load archives.
	for _, archive := range fileManagerArchiveTestFiles {
		archiveLocation := filepath.Join(filesapp.DownloadPath, archive)

		if err := fsutil.CopyFile(s.DataPath(archive), archiveLocation); err != nil {
			s.Fatalf("Cannot copy archive to %s: %s", archiveLocation, err)
		}
	}

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Cannot create test API connection: ", err)
	}
	f.tconn = tconn

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Cannot launch the Files App: ", err)
	}
	f.files = files

	orderByNameButton := nodewith.Name("Name").Role(role.Button)
	uiauto.Combine(
		"Open the Downloads folder and sort files alphabetically",
		files.OpenDownloads(),
		files.LeftClick(orderByNameButton),
	)(ctx)

	// Wait until the archives are correctly ordered in the list box.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		listBox := nodewith.Role(role.ListBox).Focusable().Multiselectable().Vertical()
		listBoxOption := nodewith.Role(role.ListBoxOption).Ancestor(listBox)
		nodes, err := files.NodesInfo(ctx, listBoxOption)
		if err != nil {
			return testing.PollBreak(err)
		}

		// The names of the descendant nodes should be ordered alphabetically.
		for i, node := range nodes {
			if node.Name != fileManagerArchiveTestFiles[i] {
				return errors.New("the files are still not ordered properly")
			}
		}

		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		s.Fatal("Cannot sort archives properly in the Files app list box: ", err)
	}

	success = true
	return files
}

func (f *loggedInFixtureForFileManager) TearDown(ctx context.Context, s *testing.FixtState) {
	for _, archive := range fileManagerArchiveTestFiles {
		archiveLocation := filepath.Join(filesapp.DownloadPath, archive)
		if err := os.Remove(archiveLocation); err != nil {
			s.Fatal("Failed to delete file: ", err)
		}
	}
}

func (f *loggedInFixtureForFileManager) Reset(ctx context.Context) error {
	return nil
}

func (f *loggedInFixtureForFileManager) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// Open the Files App.
	files, err := filesapp.Launch(ctx, f.tconn)
	if err != nil {
		s.Fatal(err, "Cannot launch the Files App: ", err)
	}
	f.files = files

	// Open the Downloads folder.
	if err := f.files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Cannot open Downloads folder: ", err)
	}
}

func (f *loggedInFixtureForFileManager) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Close the Files App.
	if err := f.files.Close(ctx); err != nil {
		s.Fatal(err, "Cannot close the Files App: ", err)
	}
}
