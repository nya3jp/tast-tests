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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type searchDriveTestResource struct {
	cr     *chrome.Chrome
	outDir string
	tconn  *chrome.TestConn
	kb     *input.KeyboardEventWriter
	ui     *uiauto.Context
	files  *filesapp.FilesApp
}

const (
	gsPrefix = "filemanager_search_drive_test_"

	gsFake    = gsPrefix + "ipsw"
	gsFolder  = gsPrefix + "folder"
	gsDocName = gsPrefix + "gdoc.gdoc"
	gsFileZip = gsPrefix + "zip.zip"
	gsFileMp4 = gsPrefix + "media.mp4"
	gsFilePdf = gsPrefix + "pdf.pdf"

	gsFileZipSrc = "500_small_files.zip"
	gsFileMp4Src = "720_av1.mp4"
	gsFilePdfSrc = "font-test.pdf"
)

type searchDriveTestSampleType string

const (
	folder searchDriveTestSampleType = "folder"
	gDoc   searchDriveTestSampleType = "gdoc"
	zip    searchDriveTestSampleType = "zip"
	mp4    searchDriveTestSampleType = "mp4"
	pdf    searchDriveTestSampleType = "pdf"
)

type searchDriveTestSampleDetail struct {
	name             string
	fileSource       string
	expectedFileType string
}

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
		Timeout:      5 * time.Minute,
		Data:         []string{gsFileZipSrc, gsFileMp4Src, gsFilePdfSrc},
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

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(cleanupCtx)

	resources := &searchDriveTestResource{
		cr:     cr,
		outDir: s.OutDir(),
		tconn:  tconn,
		kb:     kb,
		ui:     uiauto.New(tconn),
		files:  files,
	}

	// Predefine all test files and its original path.
	testSamples := map[searchDriveTestSampleType]searchDriveTestSampleDetail{
		gDoc:   {name: gsDocName, expectedFileType: "Google document"},
		pdf:    {name: gsFilePdf, expectedFileType: "PDF document", fileSource: s.DataPath(gsFilePdfSrc)},
		zip:    {name: gsFileZip, expectedFileType: "Zip archive", fileSource: s.DataPath(gsFileZipSrc)},
		mp4:    {name: gsFileMp4, expectedFileType: "MPEG video", fileSource: s.DataPath(gsFileMp4Src)},
		folder: {name: gsFolder, expectedFileType: "Folder"},
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "SD_dump")
		for _, sampleDetail := range testSamples {
			if err := files.DeleteFileOrFolder(kb, sampleDetail.name)(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to delete file or folder: ", err)
			}
		}
	}(cleanupCtx)

	if err := files.OpenDrive()(ctx); err != nil {
		s.Fatal("Failed to navigate to Google Drive folder: ", err)
	}

	// Get the snapshot of current Drive directory to determine if a test sample needs to create.
	s.Log("Taking snapshot of current Drive directory")
	driveDirSnapshot, err := getExistingItems(ctx, resources)
	if err != nil {
		s.Fatal("Failed to get existing folders: ", err)
	}

	var (
		allTargetsGone  = make([]uiauto.Action, 0)
		allTargetsExist = make([]uiauto.Action, 0)
	)

	s.Log("Creating targets")
	for sampleType, sampleDetail := range testSamples {
		allTargetsGone = append(allTargetsGone, files.WaitUntilFileGone(sampleDetail.name))
		allTargetsExist = append(allTargetsExist, files.WaitForFile(sampleDetail.name))

		// Skip if the target already exists.
		if err := fileNotExist(driveDirSnapshot, sampleDetail.name); err != nil {
			testing.ContextLogf(ctx, "Test sample %q already exist, skip create process", sampleDetail.name)
			continue
		}

		switch sampleType {
		case folder:
			if err := files.CreateFolder(kb, sampleDetail.name)(ctx); err != nil {
				s.Fatal("Failed to create folder: ", err)
			}
		case gDoc:
			if err := createGDoc(ctx, resources, &sampleDetail); err != nil {
				s.Fatal("Failed to create Google Doc via browser: ", err)
			}
		case zip, mp4, pdf:
			if err := copyToDriveDir(ctx, resources, &sampleDetail); err != nil {
				s.Fatalf("Failed to create file %q: %v", sampleDetail.name, err)
			}
		}
	}

	s.Log("Search and verify non-existent file, and files with similar name")
	for keyword, verifyActions := range map[string][]uiauto.Action{
		gsFake:   allTargetsGone,
		gsPrefix: allTargetsExist,
	} {
		if err := searchAndVerify(resources, keyword, verifyActions)(ctx); err != nil {
			s.Fatalf("Failed to search %q and verify: %v", keyword, err)
		}
	}

	s.Log("Search and verify on each test sample")
	for sampleType, sampleDetail := range testSamples {
		sampleFileType := nodewith.Role(role.StaticText).Name(sampleDetail.expectedFileType)
		verifyActions := []uiauto.Action{
			files.WaitForFile(sampleDetail.name),  // Verify target exists.
			files.WaitUntilExists(sampleFileType), // Verify target's file type.
		}

		keyword := string(sampleType)
		if err := searchAndVerify(resources, keyword, verifyActions)(ctx); err != nil {
			s.Fatalf("Failed to search %q and verify: %v", keyword, err)
		}
	}
}

// copyToDriveDir retrieves pre-defined files and copy them into Drive folder.
func copyToDriveDir(ctx context.Context, rc *searchDriveTestResource, sample *searchDriveTestSampleDetail) error {
	if err := rc.files.OpenDownloads()(ctx); err != nil {
		return errors.Wrap(err, "failed to open `Downloads` directory")
	}

	// Copy file to `Downloads` directory.
	path := filepath.Join(filesapp.DownloadPath, sample.name)
	if err := fsutil.CopyFile(sample.fileSource, path); err != nil {
		return errors.Wrapf(err, "failed to copy file to folder %s", path)
	}
	defer func() {
		if err := os.Remove(path); err != nil {
			testing.ContextLogf(ctx, "Failed to remove target %q: %v", path, err)
		}
	}()

	// Obtain file node information.
	fileNode := nodewith.Role(role.StaticText).Name(sample.name)

	if err := uiauto.Combine("copy file from `Download` to `Drive`",
		rc.ui.WaitForLocation(fileNode),
		rc.files.LeftClick(fileNode),
		rc.kb.AccelAction("ctrl+c"),
		rc.files.OpenDrive(),
		rc.kb.AccelAction("ctrl+v"),
	)(ctx); err != nil {
		return err
	}

	// Back to `Drive` directory.
	if err := rc.files.OpenDrive()(ctx); err != nil {
		return errors.Wrap(err, "failed to return to `Drive` directory")
	}

	return nil
}

func createGDoc(ctx context.Context, res *searchDriveTestResource, sample *searchDriveTestSampleDetail) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var (
		connDrive *chrome.Conn
		connDoc   *chrome.Conn
		err       error
	)
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, res.outDir, func() bool { return retErr != nil }, res.cr, "doc_dump")
		if connDrive != nil {
			connDrive.CloseTarget(ctx)
			connDrive.Close()
		}
		if connDoc != nil {
			connDoc.CloseTarget(ctx)
			connDoc.Close()
		}
	}(cleanupCtx)

	if connDrive, err = newConnectToDrivePage(ctx, res.cr); err != nil {
		return errors.Wrap(err, "failed to connect to Google Drive")
	}

	menu := nodewith.Name("New").Role("popUpButton")
	menuOption := nodewith.HasClass("h-v h-ug a-S8Cb5b").Role(role.MenuItem).Name("Google Docs")
	docTitle := nodewith.HasClass("docs-title-input").Role(role.TextField).State(state.Editable, true).Name("Rename")

	if err := uiauto.Combine("click menu and create a new `Google Doc`",
		res.ui.LeftClickUntil(menu, res.ui.WithTimeout(3*time.Second).WaitUntilExists(menuOption)),
		res.ui.LeftClickUntil(menuOption, res.ui.WithTimeout(3*time.Second).WaitUntilGone(menuOption)),
		res.ui.WaitForLocation(docTitle),
	)(ctx); err != nil {
		return err
	}

	// The Google Doc page will pop up as a new tab.
	if connDoc, err = newConnectToCurrentTab(ctx, res.cr, res.tconn); err != nil {
		return errors.Wrap(err, "failed to connect to Google Drive")
	}

	docName := strings.TrimSuffix(sample.name, ".gdoc")
	savedIcon := nodewith.HasClass("goog-control").Name("Document status: Saved to Drive.").Role(role.Button)

	return uiauto.Combine(fmt.Sprintf("rename and save new document %s", docName),
		res.ui.RetryUntil(res.ui.DoubleClick(docTitle), res.ui.WaitUntilExists(docTitle.Focused())),
		res.ui.IfSuccessThen(res.kb.TypeAction(docName), res.ui.WaitForLocation(docTitle)),
		res.ui.RetryUntil(res.kb.AccelAction("enter"), res.ui.WaitUntilExists(savedIcon)),
	)(ctx)
}

func newConnectToDrivePage(ctx context.Context, cr *chrome.Chrome) (conn *chrome.Conn, err error) {
	if conn, err = cr.NewConn(ctx, "https://drive.google.com/drive/my-drive"); err != nil {
		return nil, errors.Wrap(err, "failed to connect to Google Drive")
	}

	if err := webutil.WaitForQuiescence(ctx, conn, 15*time.Second); err != nil {
		return conn, errors.Wrap(err, "failed to wait until the new tab is stable")
	}

	return conn, nil
}

func newConnectToCurrentTab(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (conn *chrome.Conn, err error) {
	var tabURL string
	if err := tconn.Call(ctx, &tabURL, `async () => {
		let tabs = await tast.promisify(chrome.tabs.query)({active: true});
		return tabs[0].url;
	}`); err != nil {
		return nil, errors.Wrap(err, "new tab URL not found")
	}
	testing.ContextLogf(ctx, "The url is: %s", tabURL)

	if conn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL(tabURL)); err != nil {
		return nil, errors.Wrap(err, "failed to get connection to new target")
	}

	if err := webutil.WaitForQuiescence(ctx, conn, 15*time.Second); err != nil {
		return conn, errors.Wrap(err, "failed to wait until the new tab is stable")
	}

	return conn, nil
}

// searchAndVerify performs a series of actions to search and verify Google Drive searching results.
func searchAndVerify(rc *searchDriveTestResource, keyword string, verify []uiauto.Action) uiauto.Action {
	actions := make([]uiauto.Action, 0)

	// Search action.
	actions = append(actions,
		rc.files.ClearSearch(),
		rc.files.Search(rc.kb, keyword),
	)

	// Verify action.
	actions = append(actions, verify...)

	// Leave search mode.
	actions = append(actions, rc.files.ClearSearch())

	return uiauto.Combine("search and verify", actions...)
}

// getExistingItems returns existing items from Google Drive.
func getExistingItems(ctx context.Context, rc *searchDriveTestResource) ([]uiauto.NodeInfo, error) {
	items := nodewith.HasClass("table-row").Role(role.ListBoxOption)

	if err := uiauto.Combine("open drive and wait item stable",
		rc.files.OpenDrive(),
		rc.files.WaitUntilExists(items.First()),
	)(ctx); err != nil {
		return []uiauto.NodeInfo{}, nil
	}

	return rc.files.NodesInfo(ctx, items)
}

// fileNotExist return error if file exists in Google Drive.
func fileNotExist(allNodes []uiauto.NodeInfo, file string) error {
	for _, info := range allNodes {
		if file == info.Name {
			return errors.New("file exist")
		}
	}
	return nil
}
