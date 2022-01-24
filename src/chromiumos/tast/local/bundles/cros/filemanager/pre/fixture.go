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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

var testFiles = []string{
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
		Contacts:        []string{"msalomao@google.com"},
		Impl:            &loggedInFixtureForFileManager{},
		Data:            testFiles,
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Parent:          "chromeLoggedInGuestWithArchiveMount",
		Name:            "chromeLoggedInGuestForFileManager",
		Desc:            "Logged into a guest user session with test files copied over",
		Contacts:        []string{"msalomao@google.com"},
		Impl:            &loggedInFixtureForFileManager{},
		Data:            testFiles,
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
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

	// Load ZIP files.
	for _, zipFile := range testFiles {
		zipFileLocation := filepath.Join(filesapp.DownloadPath, zipFile)

		if err := fsutil.CopyFile(s.DataPath(zipFile), zipFileLocation); err != nil {
			s.Fatalf("Cannot copy ZIP file to %s: %s", zipFileLocation, err)
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

	// Open the Downloads folder.
	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Cannot open Downloads folder: ", err)
	}

	// Find and click the 'Name' button to order the file entries alphabetically.
	orderByNameButton := nodewith.Name("Name").Role(role.Button)
	if err := files.LeftClick(orderByNameButton)(ctx); err != nil {
		s.Fatal("Cannot find and click 'Name' button: ", err)
	}

	// Wait until the ZIP files are correctly ordered in the list box.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		listBox := nodewith.Role(role.ListBox).Focusable().Multiselectable().Vertical()
		listBoxOption := nodewith.Role(role.ListBoxOption).Ancestor(listBox)
		nodes, err := files.NodesInfo(ctx, listBoxOption)
		if err != nil {
			return testing.PollBreak(err)
		}

		// The names of the descendant nodes should be ordered alphabetically.
		for i, node := range nodes {
			if node.Name != testFiles[i] {
				return errors.New("the files are still not ordered properly")
			}
		}

		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		s.Fatal("Cannot sort ZIP files properly in the Files app list box: ", err)
	}

	return files
}

func (f *loggedInFixtureForFileManager) TearDown(ctx context.Context, s *testing.FixtState) {
	faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, f.tconn)

	for _, zipFile := range testFiles {
		zipFileLocation := filepath.Join(filesapp.DownloadPath, zipFile)
		os.Remove(zipFileLocation)
	}
}

func (f *loggedInFixtureForFileManager) Reset(ctx context.Context) error {
	// Open the Files App.
	files, err := filesapp.Launch(ctx, f.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch the Files App")
	}
	f.files = files

	// Open the Downloads folder.
	if err := f.files.OpenDownloads()(ctx); err != nil {
		return errors.Wrap(err, "failed to open Downloads folder")
	}

	return nil
}

func (f *loggedInFixtureForFileManager) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *loggedInFixtureForFileManager) PostTest(ctx context.Context, s *testing.FixtTestState) {}
