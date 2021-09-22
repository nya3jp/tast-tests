// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// searchDriveTestResource represents test resources used for test case filemanager.SearchDrive.
type searchDriveTestResource struct {
	cr          *chrome.Chrome
	outDir      string
	tconn       *chrome.TestConn
	kb          *input.KeyboardEventWriter
	ui          *uiauto.Context
	files       *filesapp.FilesApp
	drivefsRoot string
}

const (
	gsPrefix = "filemanager_search_drive_test_"

	gsFake      = gsPrefix + "ipsw"
	gsFolder    = gsPrefix + "folder"
	gsDocName   = gsPrefix + "gdoc"
	gsFileZip   = gsPrefix + "zip"
	gsFileMedia = gsPrefix + "mp4"
	gsFilePdf   = gsPrefix + "pdf"
)

// searchDriveTestSampleType represents a file types used for test case filemanager.SearchDrive.
type searchDriveTestSampleType string

const (
	folder searchDriveTestSampleType = "folder"
	gDoc   searchDriveTestSampleType = "gdoc"
	zip    searchDriveTestSampleType = "zip"
	mp4    searchDriveTestSampleType = "mp4"
	pdf    searchDriveTestSampleType = "pdf"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchDrive,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test Google Drive search feature on file manager app",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"lance.wang@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "drivefs"},
		VarDeps:      []string{"drivefs.accountPool", "drivefs.clientCredentials", "drivefs.refreshTokens"},
		Fixture:      "driveFsStarted",
		Timeout:      5 * time.Minute,
	})
}

// SearchDrive verifies Google Drive search on Files app.
func SearchDrive(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*drivefs.FixtureData).Chrome
	apiClient := s.FixtValue().(*drivefs.FixtureData).APIClient
	tconn := s.FixtValue().(*drivefs.FixtureData).TestAPIConn
	mountPath := s.FixtValue().(*drivefs.FixtureData).MountPath
	drivefsRoot := filepath.Join(mountPath, "root")

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	// Predefine all test files and its original path.
	testSamples := map[searchDriveTestSampleType]searchDriveTestSampleDetail{
		gDoc:   {name: gsDocName, fileExtension: "gdoc", expectedFileType: "Google document", mime: drivefs.GoogleDoc},
		pdf:    {name: gsFilePdf, fileExtension: "pdf", expectedFileType: "PDF document", mime: drivefs.PDF},
		zip:    {name: gsFileZip, fileExtension: "", expectedFileType: "Zip archive", mime: drivefs.Zip},
		mp4:    {name: gsFileMedia, fileExtension: "", expectedFileType: "MPEG video", mime: drivefs.MP4},
		folder: {name: gsFolder, fileExtension: "", expectedFileType: "Folder", mime: drivefs.Folder},
	}

	// Ensure the name of test folder file is unique by combine a long string, timestamp and a random number.
	testFolderName := fmt.Sprintf("searchDrive_test-%020d-%06d", time.Now().UnixNano(), rand.Intn(100000))
	testFolder, err := apiClient.CreateNewFile(ctx, testFolderName, []string{"root"}, drivefs.Folder)
	if err != nil {
		s.Fatal("Failed to create a test folder: ", err)
	}
	defer apiClient.RemoveFileByID(cleanupCtx, testFolder.Id)

	s.Log("Creating targets")
	var filesInTestFolder []string
	for sampleType, sampleDetail := range testSamples {
		filesInTestFolder = append(filesInTestFolder, sampleDetail.fileFullName())

		gDoc, err := apiClient.CreateNewFile(ctx, sampleDetail.name, []string{testFolder.Id}, sampleDetail.mime)
		if err != nil {
			s.Fatalf("Failed to create document with type %q: %s", sampleType, err)
		}
		defer apiClient.RemoveFileByID(cleanupCtx, gDoc.Id)
	}

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "dump_before_files_closure")

	s.Log("Ensure all files are ready for testing")
	if err := ensureFilesReady(ctx, files, testFolderName, filesInTestFolder); err != nil {
		s.Fatal("Failed to ensure all files are ready: ", err)
	}

	resources := &searchDriveTestResource{
		cr:          cr,
		outDir:      s.OutDir(),
		tconn:       tconn,
		kb:          kb,
		ui:          uiauto.New(tconn),
		files:       files,
		drivefsRoot: drivefsRoot,
	}

	s.Log("Search and verify on each test sample")
	var (
		allTargetsGone  []uiauto.Action
		allTargetsExist []uiauto.Action
	)
	for sampleType, sampleDetail := range testSamples {
		allTargetsGone = append(allTargetsGone, files.WaitUntilFileGone(sampleDetail.fileFullName()))
		allTargetsExist = append(allTargetsExist, files.WaitForFile(sampleDetail.fileFullName()))

		tableRow := nodewith.Name(sampleDetail.fileFullName()).HasClass("table-row").Role(role.ListBoxOption)
		sampleFileType := nodewith.Role(role.StaticText).Name(sampleDetail.expectedFileType).Ancestor(tableRow)
		verifyActions := []uiauto.Action{
			files.WaitForFile(sampleDetail.fileFullName()), // Verify target exists.
			files.WaitUntilExists(sampleFileType),          // Verify target's file type.
		}

		keyword := string(sampleType)
		if err := searchAndVerify(ctx, resources, keyword, verifyActions); err != nil {
			s.Fatalf("Failed to search %q and verify: %v", keyword, err)
		}
	}

	s.Log("Search and verify non-existent file, and files with similar name")
	for keyword, verifyActions := range map[string][]uiauto.Action{
		gsFake:   allTargetsGone,
		gsPrefix: allTargetsExist,
	} {
		if err := searchAndVerify(ctx, resources, keyword, verifyActions); err != nil {
			s.Fatalf("Failed to search %q and verify: %v", keyword, err)
		}
	}
}

type searchDriveTestSampleDetail struct {
	name             string
	fileExtension    string
	expectedFileType string
	mime             drivefs.FileMime
}

// fileFullName returns file's full name(including file extension if applied).
func (file *searchDriveTestSampleDetail) fileFullName() string {
	if file.fileExtension != "" {
		return fmt.Sprintf("%s.%s", file.name, file.fileExtension)
	}
	return file.name
}

// ensureFilesReady ensures test folder and its files are ready for testing.
func ensureFilesReady(ctx context.Context, files *filesapp.FilesApp, testFolderName string, filesName []string) error {
	if err := uiauto.Combine("open google drive and create a folder",
		files.OpenDrive(),
		// Expand the folder under My Drive folder.
		files.WithTimeout(time.Minute).WaitUntilExists(nodewith.Name(testFolderName).Role(role.StaticText)),
		files.DoubleClick(nodewith.Name("My Drive").HasClass("tree-item")),
		files.WaitUntilExists(nodewith.Name(testFolderName).HasClass("tree-item")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait until the drive folder is created")
	}

	// Needs to reopen the test directory to reload.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		for _, file := range filesName {
			if err := uiauto.Combine("wait for all the new google files to appear",
				files.OpenDrive(),
				files.OpenDir(testFolderName, filesapp.FilesTitlePrefix+testFolderName),
				files.WithTimeout(5*time.Second).WaitUntilExists(nodewith.Name(file).HasClass("table-row").Role(role.ListBoxOption)),
			)(ctx); err != nil {
				return errors.Wrapf(err, "failed to wait until %s is created", file)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 1 * time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait until the files is created")
	}
	return nil
}

// searchAndVerify performs a series of actions to search and verify Google Drive searching results.
func searchAndVerify(ctx context.Context, res *searchDriveTestResource, keyword string, verify []uiauto.Action) error {
	// Search results might be displayed inappropriately.
	// Therefore, need to polling on entire action.
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := res.files.Search(res.kb, keyword)(ctx); err != nil {
			return errors.Wrapf(err, "failed to search keyword %q", keyword)
		}
		defer res.files.ClearSearch()(ctx)

		if err := uiauto.Combine("search and verify", verify...)(ctx); err != nil {
			return errors.Wrapf(err, "failed to search and verify keyword %q", keyword)
		}

		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute})
}
