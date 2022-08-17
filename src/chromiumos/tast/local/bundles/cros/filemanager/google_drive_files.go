// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
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

const (
	jpegFile        = "test_jpeg.jpeg"
	pngFile         = "files_app_test.png"
	gDocFileName    = "test docs"
	gSheetFileName  = "test sheets"
	gSlidesFileName = "test slides"
	gDocFileExt     = ".gdoc"
	gSheetFileExt   = ".gsheet"
	gSlidesFileExt  = ".gslides"

	fileSaving = "Document status: Saving…."
	fileSaved  = "Document status: Saved to Drive."

	// A file on G-Drive might tooks longer to synchronize with fileaspp.
	driveSyncTimeout = 3 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleDriveFiles,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test a file on Google Drive will appear on top of the list in filesapp after edited",
		Contacts: []string{
			"vivian.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		// TODO(crbug/1299712): This test is constantly failing. The maintainers are
		// external and it overlaps with other more robust tests (e.g. filemanager.DrivefsGoogleDoc).
		// Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{jpegFile, pngFile},
		Timeout:      5*time.Minute + driveSyncTimeout*5, // There are 5 file edit actions in this test.
		Params: []testing.Param{{
			Val:     apps.Chrome.ID,
			Fixture: "driveFsStarted",
		}, {
			Name:              "lacros",
			Val:               apps.LacrosID,
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "driveFsStartedLacros",
		}},
	})
}

// GoogleDriveFiles tests a file on Google Drive will be appear on top of the list in filesapp after edited.
func GoogleDriveFiles(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*drivefs.FixtureData).Chrome
	apiClient := s.FixtValue().(*drivefs.FixtureData).APIClient
	tconn := s.FixtValue().(*drivefs.FixtureData).TestAPIConn
	mountPath := s.FixtValue().(*drivefs.FixtureData).MountPath

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// Ensure the name of folder is unique by combine a long string, timestamp and a random number.
	folderName := fmt.Sprintf("filemanager_GoogleDriveFiles_test_folder_name_%020d_%06d", time.Now().UnixNano(), rand.Intn(100000))
	folder, err := apiClient.Createfolder(ctx, folderName, []string{"root"})
	if err != nil {
		s.Fatal("Failed to create folder: ", err)
	}
	defer func(ctx context.Context) {
		if err := apiClient.RemoveFileByID(ctx, folder.Id); err != nil {
			s.Logf("Failed to remove folder %q from drive: %v", folder.Name, err)
		}
	}(cleanupCtx)

	// All descendants of the folder should be deleted as the folder be removed,
	// so no need to clean up this file.
	if _, err := apiClient.CreateBlankGoogleDoc(ctx, gDocFileName, []string{folder.Id}); err != nil {
		s.Fatal("Failed to create blank google doc: ", err)
	}

	// All descendants of the folder should be deleted as the folder be removed,
	// so no need to clean up this file.
	if _, err := apiClient.CreateBlankGoogleSheet(ctx, gSheetFileName, []string{folder.Id}); err != nil {
		s.Fatal("Failed to create blank google sheet: ", err)
	}

	// All descendants of the folder should be deleted as the folder be removed,
	// so no need to clean up this file.
	if _, err := apiClient.CreateBlankGoogleSlide(ctx, gSlidesFileName, []string{folder.Id}); err != nil {
		s.Fatal("Failed to create blank google slide: ", err)
	}

	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files App: ", err)
	}
	defer filesApp.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "filesapp_ui_dump")

	filesAppConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(fmt.Sprintf("chrome-extension://%s/main.html", apps.Files.ID)))
	if err != nil {
		s.Fatal("Failed to connect to files app foreground page: ", err)
	}
	defer filesAppConn.Close()

	resources := &gDriveFilesTestResources{
		filesApp:     filesApp,
		filesAppConn: filesAppConn,
		cr:           cr,
		tconn:        tconn,
		kb:           kb,
		ui:           uiauto.New(tconn),
		folderName:   folderName,
		browserAppID: s.Param().(string),
	}

	files := []string{gDocFileName + gDocFileExt, gSheetFileName + gSheetFileExt, gSlidesFileName + gSlidesFileExt}
	if err := waitUntilGDocsSynced(ctx, resources, folderName, files...); err != nil {
		s.Fatal("Failed to create test folder: ", err)
	}

	// The folder does not exist before contents from Drive being synchronized to DUT,
	// therefore, we can only proceed on this step after the contents from Drive are synchronized.
	for _, image := range []string{jpegFile, pngFile} {
		path := filepath.Join(mountPath, "root", folderName, image)
		// All descendants of the folder should be deleted as the folder be removed,
		// so no need to clean up this file.
		if err := fsutil.CopyFile(s.DataPath(image), path); err != nil {
			s.Fatal(err, "Failed to copy file to drive folder")
		}
	}

	for _, testSample := range []struct {
		editor      gDriveFileTest
		description string
	}{
		{newGDocEditor(gDocFileName, resources), "g doc test"},
		{newGSheetEditor(gSheetFileName, resources), "g sheet test"},
		{newGSlides(gSlidesFileName, resources), "g slide test"},
		{newImageEditor(jpegFile, resources), "jpeg test"},
		{newImageEditor(pngFile, resources), "png test"},
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
				faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, resources.cr, uiName)
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

type gDocSyncedType int

const (
	// syncedTypeFileAppear defines a file that is considered as synced from the cloud if it appears in the files app.
	syncedTypeFileAppear gDocSyncedType = iota
	// syncedTypeFileOnTop defines a file that is considered as synced from the cloud if it is on top of the file list (the most recently edited one).
	syncedTypeFileOnTop
)

func reloadFilesAppUntilSynced(ctx context.Context, res *gDriveFilesTestResources, fileName string, syncedType gDocSyncedType) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := res.filesAppConn.Eval(ctx, `location.reload()`, nil); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to reload filesapp page"))
		}

		switch syncedType {
		case syncedTypeFileAppear:
			return res.filesApp.WaitForFile(fileName)(ctx)
		case syncedTypeFileOnTop:
			topNodeInfo, err := res.ui.Info(ctx, nodewith.Role(role.ListBoxOption).HasClass("table-row file").First())
			if err != nil {
				return errors.Wrap(err, "failed to get node info")
			}
			if topNodeInfo.Name != fileName {
				return errors.Errorf("%q is not on top of the file list", fileName)
			}
			return nil
		default:
			return testing.PollBreak(errors.New("unrecognized synced type"))
		}
	}, &testing.PollOptions{Timeout: driveSyncTimeout})
}

// waitUntilGDocsSynced waits until Google documents are all synced to filesapp.
func waitUntilGDocsSynced(ctx context.Context, res *gDriveFilesTestResources, folderName string, filesName ...string) error {
	if err := res.filesApp.OpenDrive()(ctx); err != nil {
		return errors.Wrap(err, "failed to open Drive")
	}

	if err := reloadFilesAppUntilSynced(ctx, res, folderName, syncedTypeFileAppear); err != nil {
		return errors.Wrap(err, "failed to wait until folder is synced")
	}

	if err := uiauto.Combine(fmt.Sprintf("open folder %q", folderName),
		res.filesApp.DoubleClick(nodewith.Name("My Drive").HasClass("tree-item")),
		res.filesApp.WaitUntilExists(nodewith.Name(folderName).HasClass("tree-item")),
		res.filesApp.OpenDir(folderName, filesapp.FilesTitlePrefix+folderName),
	)(ctx); err != nil {
		return err
	}

	for _, file := range filesName {
		if err := reloadFilesAppUntilSynced(ctx, res, file, syncedTypeFileAppear); err != nil {
			return errors.Wrapf(err, "failed to wait until %q is synced", file)
		}
	}

	return nil
}

// gDriveFilesTestResources holds resources against GoogleDriveFiles test.
type gDriveFilesTestResources struct {
	filesApp     *filesapp.FilesApp
	filesAppConn *chrome.Conn
	cr           *chrome.Chrome
	tconn        *chrome.TestConn
	kb           *input.KeyboardEventWriter
	ui           *uiauto.Context
	folderName   string
	browserAppID string
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
		f.res.filesApp.OpenDrive(),
		f.res.filesApp.OpenDir(f.res.folderName, filesapp.FilesTitlePrefix+f.res.folderName),
		f.res.filesApp.OpenFile(f.name),
	)(ctx)
}

// verify verifies if the test sample file is on top of the file list.
func (f *fileSample) verify(ctx context.Context) error {
	if err := reloadFilesAppUntilSynced(ctx, f.res, f.name, syncedTypeFileOnTop); err != nil {
		return errors.Wrap(err, "failed to wait for the test sample file to be the recently edited file")
	}
	return nil
}

// getFileName gets the name of test sample file.
func (f *fileSample) getFileName() string { return f.name }

// gDocGSheetEditor represents the editor of a Google Docs or Google Sheets file.
type gDocGSheetEditor struct {
	*fileSample
	rootWindow *nodewith.Finder
}

// newGDocEditor creates and returns an instance of Google Docs editor.
func newGDocEditor(name string, res *gDriveFilesTestResources) *gDocGSheetEditor {
	return &gDocGSheetEditor{
		fileSample: newSample(name+gDocFileExt, res),
		rootWindow: nodewith.Role(role.Window).HasClass("Widget").NameStartingWith(fmt.Sprintf("%s - Google Docs", name)),
	}
}

// newGSheetEditor creates and returns an instance of Google Sheets editor.
func newGSheetEditor(name string, res *gDriveFilesTestResources) *gDocGSheetEditor {
	return &gDocGSheetEditor{
		fileSample: newSample(name+gSheetFileExt, res),
		rootWindow: nodewith.Role(role.Window).HasClass("Widget").NameStartingWith(fmt.Sprintf("%s - Google Sheets", name)),
	}
}

// edit edits the Google Docs and the Google Sheets.
func (editor *gDocGSheetEditor) edit(ctx context.Context) error {
	closePromptBtn := nodewith.Name("Close").Role(role.Button).Ancestor(editor.rootWindow)
	return uiauto.Combine("edit and save file",
		editor.res.ui.WaitUntilExists(nodewith.Name(fileSaved).Role(role.Button).Ancestor(editor.rootWindow)),
		uiauto.IfSuccessThen(editor.res.ui.WaitUntilExists(closePromptBtn), editor.res.ui.LeftClick(closePromptBtn)),
		editor.res.kb.TypeAction("edit the google file"),
		editor.res.kb.AccelAction("Enter"),
		editor.res.ui.WaitUntilExists(nodewith.Name(fileSaving).Role(role.Button).Ancestor(editor.rootWindow)),
		editor.res.ui.WaitUntilExists(nodewith.Name(fileSaved).Role(role.Button).Ancestor(editor.rootWindow)),
	)(ctx)
}

// close closes the window to edit the Google Sheets/Google Docs.
func (editor *gDocGSheetEditor) close(ctx context.Context) error {
	return apps.Close(ctx, editor.res.tconn, editor.res.browserAppID)
}

// gSlidesEditor represents the editor of a Google Slides file.
type gSlidesEditor struct {
	*fileSample
	rootWindow *nodewith.Finder
}

// newGSlides creates and returns an instance of Google Slides editor.
func newGSlides(name string, res *gDriveFilesTestResources) *gSlidesEditor {
	return &gSlidesEditor{
		fileSample: newSample(name+gSlidesFileExt, res),
		rootWindow: nodewith.Role(role.Window).HasClass("Widget").NameStartingWith(fmt.Sprintf("%s - Google Slides", name)),
	}
}

// edit edits the Google Slides.
func (editor *gSlidesEditor) edit(ctx context.Context) error {
	closePromptBtn := nodewith.Name("Close").Role(role.Button).Ancestor(editor.rootWindow)
	return uiauto.Combine("edit and save Google Slides",
		editor.res.ui.WaitUntilExists(nodewith.Name(fileSaved).Role(role.Button).Ancestor(editor.rootWindow)),
		uiauto.IfSuccessThen(editor.res.ui.WaitUntilExists(closePromptBtn), editor.res.ui.LeftClick(closePromptBtn)),
		editor.res.ui.LeftClick(nodewith.Role(role.Group).HasClass("sketchy-text-content").Ancestor(editor.rootWindow).First()),
		editor.res.kb.AccelAction("Enter"),
		editor.res.kb.TypeAction("edit the google file"),
		editor.res.ui.WaitUntilExists(nodewith.Name(fileSaving).Role(role.Button).Ancestor(editor.rootWindow)),
		editor.res.ui.WaitUntilExists(nodewith.Name(fileSaved).Role(role.Button).Ancestor(editor.rootWindow)),
	)(ctx)
}

// close closes the window to edit the Google Slides.
func (editor *gSlidesEditor) close(ctx context.Context) error {
	return apps.Close(ctx, editor.res.tconn, editor.res.browserAppID)
}

// imageEditor represents the editor of a image file.
type imageEditor struct {
	*fileSample
	rootWindow *nodewith.Finder
}

// newImageEditor creates and returns an instance of image editor.
func newImageEditor(name string, res *gDriveFilesTestResources) *imageEditor {
	return &imageEditor{
		fileSample: newSample(name, res),
		rootWindow: nodewith.NameStartingWith(apps.Gallery.Name).HasClass("BrowserFrame"),
	}
}

// edit edits the image file.
func (editor *imageEditor) edit(ctx context.Context) error {
	return uiauto.Combine("edit and save image",
		editor.res.ui.LeftClick(nodewith.Name("Crop & rotate").Role(role.ToggleButton).Ancestor(editor.rootWindow)),
		editor.res.ui.LeftClick(nodewith.Name("16:9").Role(role.Button).Ancestor(editor.rootWindow)),
		editor.res.ui.LeftClick(nodewith.Name("Done").Role(role.Button).HasClass("mdc-button  mdc-button--unelevated").Ancestor(editor.rootWindow)),
		editor.res.ui.LeftClick(nodewith.Name("Save").Role(role.Button).HasClass("mdc-button  mdc-button--unelevated").Ancestor(editor.rootWindow)),
		editor.res.ui.WaitUntilExists(nodewith.Name("Saved").Role(role.StaticText).Ancestor(editor.rootWindow)),
	)(ctx)
}

// close closes the window to edit the image.
func (editor *imageEditor) close(ctx context.Context) error {
	return apps.Close(ctx, editor.res.tconn, apps.Gallery.ID)
}
