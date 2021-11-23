// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/api/drive/v3"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	folderName      = "filemanager_GoogleDriveFiles_test_folder_name"
	jpgFile         = "scan_source.jpg"
	pngFile         = "files_app_test.png"
	gDocFileName    = "Google Docs"
	gSheetFileName  = "Google Sheets"
	gSlidesFileName = "Google Slides"
	gDocFileExt     = ".gdoc"
	gSheetFileExt   = ".gsheet"
	gSlidesFileExt  = ".gslides"

	fileSaving = "Document status: Saving…."
	fileSaved  = "Document status: Saved to Drive."

	// A file on G-Drive might tooks longer to synchronize with fileaspp.
	longUITimeout = 30 * time.Second
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleDriveFiles,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test a file on Google Drive will appear on top of the list in filesapp after edited",
		Contacts: []string{
			"vivian.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{jpgFile, pngFile},
		Fixture:      "driveFsStarted",
		Timeout:      7 * time.Minute,
	})
}

// GoogleDriveFiles tests a file on Google Drive will be appear on top of the list in filesapp after edited.
func GoogleDriveFiles(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*drivefs.FixtureData).Chrome
	apiClient := s.FixtValue().(*drivefs.FixtureData).APIClient
	tconn := s.FixtValue().(*drivefs.FixtureData).TestAPIConn

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	// Check if test directory exists before it is created.
	fileList, err := apiClient.FindFileByName(ctx, folderName)
	if err != nil {
		s.Fatal("Failed to list the users files: ", err)
	}
	for _, file := range fileList.Files {
		// Remove the existing test directory.
		if err := apiClient.RemoveFileByID(ctx, file.Id); err != nil {
			s.Fatal("Failed to delete the already exist test directory: ", err)
		}
	}

	removeFromDrive := func(ctx context.Context, file *drive.File) {
		if err := apiClient.RemoveFileByID(ctx, file.Id); err != nil {
			s.Logf("Failed to remove file %q from drive: %v", file.Name, err)
		}
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	folder, err := apiClient.Createfolder(ctx, folderName, []string{"root"})
	if err != nil {
		s.Fatal("Failed to create folder: ", err)
	}
	defer removeFromDrive(cleanupCtx, folder)

	docs, err := apiClient.CreateBlankGoogleDoc(ctx, gDocFileName, []string{"root", folder.Id})
	if err != nil {
		s.Fatal("Failed to create blank google doc: ", err)
	}
	defer removeFromDrive(cleanupCtx, docs)

	sheets, err := apiClient.CreateBlankGoogleSheet(ctx, gSheetFileName, []string{"root", folder.Id})
	if err != nil {
		s.Fatal("Failed to create blank google sheet: ", err)
	}
	defer removeFromDrive(cleanupCtx, sheets)

	slides, err := apiClient.CreateBlankGoogleSlide(ctx, gSlidesFileName, []string{"root", folder.Id})
	if err != nil {
		s.Fatal("Failed to create blank google slide: ", err)
	}
	defer removeFromDrive(cleanupCtx, slides)

	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files App: ", err)
	}
	defer filesApp.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "filesapp_ui_dump")

	// It might takes a while for the drive folder and google files to show in filesapp.
	if err := waitUntilDriveFolderAndFilesExist(ctx, filesApp, gDocFileName+gDocFileExt, gSheetFileName+gSheetFileExt, gSlidesFileName+gSlidesFileExt); err != nil {
		s.Fatal("Failed to create test folder: ", err)
	}

	for _, image := range []string{jpgFile, pngFile} {
		if err := copyFileToDriveDir(ctx, kb, filesApp, s.DataPath(image), image); err != nil {
			s.Fatal("Failed to copy file into test directory: ", err)
		}
		defer func(ctx context.Context, fileName string) {
			fileList, _ := apiClient.FindFileByName(ctx, fileName)
			for _, file := range fileList.Files {
				s.Log("Removing file: ", file.Name)
				removeFromDrive(ctx, file)
			}
		}(cleanupCtx, image)
	}

	res := &gDriveFilesTestResources{
		files: filesApp,
		cr:    cr,
		tconn: tconn,
		kb:    kb,
		ui:    uiauto.New(tconn),
	}

	for _, testSample := range []struct {
		editor      gDriveFileTest
		description string
	}{
		{newImageEditor(jpgFile, res), "jpg test"},
		{newImageEditor(pngFile, res), "png test"},
		{newGDocEditor(gDocFileName+gDocFileExt, res), "g doc test"},
		{newGSheetEditor(gSheetFileName+gSheetFileExt, res), "g sheet test"},
		{newGSlides(gSlidesFileName+gSlidesFileExt, res), "g slide test"},
	} {
		f := func(ctx context.Context, s *testing.State) {
			cleanupSubtestCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
			defer cancel()

			if err := testSample.editor.open(ctx); err != nil {
				s.Fatalf("Failed to open %q: %v", testSample.editor.getFileName(), err)
			}
			defer func(ctx context.Context) {
				uiName := strings.ReplaceAll(testSample.description, " ", "_")
				faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, res.cr, uiName)
			}(cleanupSubtestCtx)

			if err := testSample.editor.edit(ctx); err != nil {
				s.Fatalf("Failed to edit %q: %v", testSample.editor.getFileName(), err)
			}

			// Need to close window to show filesapp.
			if err := testSample.editor.close(ctx); err != nil {
				s.Fatalf("Failed to close %q: %v", testSample.editor.getFileName(), err)
			}

			if err := testSample.editor.verify(ctx); err != nil {
				s.Fatalf("Failed to verify %q: %v", testSample.editor.getFileName(), err)
			}
		}

		if !s.Run(ctx, testSample.description, f) {
			s.Errorf("Failed to test %s's order after edited", testSample.description)
		}
	}
}

// waitUntilDriveFolderAndFilesExist waits until the folder and the files is created and expend the new created folder.
func waitUntilDriveFolderAndFilesExist(ctx context.Context, files *filesapp.FilesApp, filesName ...string) error {
	if err := uiauto.Combine("open google drive and create a folder",
		files.OpenDrive(),
		// Expand the folder under My Drive folder.
		files.WithTimeout(longUITimeout).WaitUntilExists(nodewith.Name(folderName).Role(role.StaticText)),
		files.DoubleClick(nodewith.Name("My Drive").HasClass("tree-item")),
		files.WaitUntilExists(nodewith.Name(folderName).HasClass("tree-item")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait until the drive folder is created")
	}

	// Needs to reopen the test directory to reload.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		for _, file := range filesName {
			if err := uiauto.Combine("wait for all the new google files to appear",
				files.OpenDrive(),
				files.OpenDir(folderName, filesapp.FilesTitlePrefix+folderName),
				files.WithTimeout(5*time.Second).WaitUntilExists(nodewith.Name(file).Role(role.StaticText)),
			)(ctx); err != nil {
				return errors.Wrapf(err, "failed to wait until %s is created", file)
			}
		}
		return nil
	}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait until the files is created")
	}

	return nil
}

// copyFileToDriveDir copies a file from `Download` to `Drive`
func copyFileToDriveDir(ctx context.Context, kb *input.KeyboardEventWriter, files *filesapp.FilesApp, fileSource, fileName string) error {
	if err := files.OpenDownloads()(ctx); err != nil {
		return errors.Wrap(err, "failed to open download directory")
	}

	path := filepath.Join(filesapp.DownloadPath, fileName)
	if err := fsutil.CopyFile(fileSource, path); err != nil {
		return errors.Wrap(err, "failed to copy file to download directory")
	}
	defer os.Remove(path)

	return uiauto.Combine("copy file from `Downloads` to `My Drive`",
		files.LeftClick(nodewith.Role(role.StaticText).Name(fileName)),
		kb.AccelAction("Ctrl+c"),
		files.OpenDrive(),
		files.OpenDir(folderName, filesapp.FilesTitlePrefix+folderName),
		files.WaitUntilExists(nodewith.Name("Name").Role(role.Button)),
		kb.AccelAction("Ctrl+v"),
		files.WaitUntilExists(nodewith.Role(role.StaticText).Name(fileName)),
	)(ctx)
}

func closeBrowser(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Eval(ctx, `(async () => {
		const tabs = await tast.promisify(chrome.tabs.query)({});
		await tast.promisify(chrome.tabs.remove)(tabs.filter((tab) => tab.id).map((tab) => tab.id));
	})()`, nil)
}

// gDriveFilesTestResources holds resources against GoogleDriveFiles test.
type gDriveFilesTestResources struct {
	files *filesapp.FilesApp
	cr    *chrome.Chrome
	tconn *chrome.TestConn
	kb    *input.KeyboardEventWriter
	ui    *uiauto.Context
}

type gDriveFileTest interface {
	open(ctx context.Context) error
	edit(ctx context.Context) error
	close(ctx context.Context) error
	verify(ctx context.Context) error

	getFileName() string
}

// fileSample performs actions the files need.
type fileSample struct {
	res  *gDriveFilesTestResources
	name string
}

func newSample(name string, res *gDriveFilesTestResources) *fileSample {
	return &fileSample{
		name: name,
		res:  res,
	}
}

// open opens the file in the drive folder.
func (f *fileSample) open(ctx context.Context) error {
	return uiauto.Combine("open file",
		f.res.files.OpenDrive(),
		f.res.files.OpenDir(folderName, filesapp.FilesTitlePrefix+folderName),
		f.res.files.OpenFile(f.name),
	)(ctx)
}

// verify verifies if the test sample file is on top of the file list.
func (f *fileSample) verify(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Google Sheets needs to reopen directory to reload.
		if err := uiauto.Combine("reopen test directory",
			f.res.files.OpenDrive(),
			f.res.files.OpenDir(folderName, filesapp.FilesTitlePrefix+folderName),
		)(ctx); err != nil {
			return err
		}

		firstNodeInfo, err := f.res.ui.Info(ctx, nodewith.Role(role.ListBoxOption).HasClass("table-row file").First())
		if err != nil {
			return errors.Wrap(err, "failed to get node info")
		}
		if firstNodeInfo.Name != f.name {
			return errors.New("the test sample file is not on top of the file list")
		}

		return nil
	}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait for the test sample file to be the recently edited file")
	}

	return nil
}

// getFileName gets the name of test sample file.
func (f *fileSample) getFileName() string { return f.name }

// gDocGSheetEditor represents the editor of a Google Docs or Google Sheets file.
type gDocGSheetEditor struct{ *fileSample }

// newGDocEditor creates and returns an instance of Google Docs editor.
func newGDocEditor(name string, res *gDriveFilesTestResources) *gDocGSheetEditor {
	return &gDocGSheetEditor{newSample(name, res)}
}

// newGSheetEditor creates and returns an instance of Google Sheets editor.
func newGSheetEditor(name string, res *gDriveFilesTestResources) *gDocGSheetEditor {
	return newGDocEditor(name, res)
}

// edit edits the Google Docs and the Google Sheets.
func (f *gDocGSheetEditor) edit(ctx context.Context) error {
	return uiauto.Combine("edit and save file",
		f.res.ui.WaitUntilExists(nodewith.Name(fileSaved)),
		f.res.kb.TypeAction("edit the google file"),
		f.res.kb.AccelAction("Enter"),
		f.res.ui.WaitUntilExists(nodewith.Name(fileSaving)),
		f.res.ui.WaitUntilExists(nodewith.Name(fileSaved)),
	)(ctx)
}

// close closes the opened browser.
func (f *gDocGSheetEditor) close(ctx context.Context) error {
	return closeBrowser(ctx, f.res.tconn)
}

// gSlidesEditor represents the editor of a Google Slides file.
type gSlidesEditor struct{ *fileSample }

// newGSlides creates and returns an instance of Google Slides editor.
func newGSlides(name string, res *gDriveFilesTestResources) *gSlidesEditor {
	return &gSlidesEditor{newSample(name, res)}
}

// edit edits the Google Slides.
func (f *gSlidesEditor) edit(ctx context.Context) error {
	return uiauto.Combine("edit and save Google Slides",
		f.res.ui.WaitUntilExists(nodewith.Name(fileSaved)),
		f.res.ui.LeftClick(nodewith.Role(role.Group).HasClass("sketchy-text-content").First()),
		f.res.kb.AccelAction("Enter"),
		f.res.kb.TypeAction("edit the google file"),
		f.res.ui.WaitUntilExists(nodewith.Name(fileSaving)),
		f.res.ui.WaitUntilExists(nodewith.Name(fileSaved)),
	)(ctx)
}

// close closes the opened browser.
func (f *gSlidesEditor) close(ctx context.Context) error {
	return closeBrowser(ctx, f.res.tconn)
}

// imageEditor represents the editor of a image file.
type imageEditor struct{ *fileSample }

// newImageEditor creates and returns an instance of image editor.
func newImageEditor(name string, res *gDriveFilesTestResources) *imageEditor {
	return &imageEditor{newSample(name, res)}
}

// edit edits the image file.
func (f *imageEditor) edit(ctx context.Context) error {
	return uiauto.Combine("edit and save image",
		f.res.ui.LeftClick(nodewith.Name("Crop & rotate").Role(role.ToggleButton)),
		f.res.ui.LeftClick(nodewith.Name("16:9").Role(role.Button)),
		f.res.ui.LeftClick(nodewith.Name("Done").Role(role.Button).HasClass("mdc-button  mdc-button--unelevated")),
		f.res.ui.LeftClick(nodewith.Name("Save").Role(role.Button).HasClass("mdc-button  mdc-button--unelevated")),
		f.res.ui.WaitUntilExists(nodewith.Name("Saved").Role(role.StaticText)),
	)(ctx)
}

// close closes the Gallery window.
func (f *imageEditor) close(ctx context.Context) error {
	window, err := ash.FindOnlyWindow(ctx, f.res.tconn, func(w *ash.Window) bool {
		return strings.Contains(w.Title, "Gallery")
	})
	if err != nil {
		return errors.Wrap(err, "failed to get gallery window")
	}

	return window.CloseWindow(ctx, f.res.tconn)
}
