// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	testFile1 = "image01.png"
	testFile2 = "image02.jpg"
	testFile3 = "image03.png"

	regularFileURLPrefix = "https://storage.googleapis.com/chromiumos-test-assets-public/tast/cros/"
	downloadedFile1      = "white_wallpaper.jpg"
	downloadedFile2      = "contentpreview_20210511.png"
)

// ofhResource holds resources used by test case holdingspace.ContentSelectionAndLaunch.
type ofhResource struct {
	cr     *chrome.Chrome
	tconn  *chrome.TestConn
	kb     *input.KeyboardEventWriter
	ui     *uiauto.Context
	files  *filesapp.FilesApp
	server *httptest.Server
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ContentSelectionAndLaunch,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verifies pin, unpin, select and launch single/multi-file(s) operations can be done on Holding Space",
		Contacts:     []string{"lance.wang@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{testFile1, testFile2, testFile3},
		Fixture:      "chromeLoggedIn",
	})
}

// ContentSelectionAndLaunch verifies pinned files on Holding Space can be selected, opened and unpinned.
func ContentSelectionAndLaunch(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}
	defer files.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	resources := ofhResource{
		cr:    cr,
		tconn: tconn,
		kb:    kb,
		ui:    uiauto.New(tconn),
		files: files,
	}

	// Predefine all imported files and its original path.
	importedFiles := []*fileTarget{
		{name: testFile1, source: s.DataPath(testFile1), path: filepath.Join(filesapp.DownloadPath, testFile1)},
		{name: testFile2, source: s.DataPath(testFile2), path: filepath.Join(filesapp.DownloadPath, testFile2)},
		{name: testFile3, source: s.DataPath(testFile3), path: filepath.Join(filesapp.DownloadPath, testFile3)},
	}

	// Set up three image files for later use.
	for _, file := range importedFiles {
		if err := fsutil.CopyFile(file.source, file.path); err != nil {
			s.Fatalf("Failed to copy file %q: %s", file.name, err)
		}
		defer func(ctx context.Context, name, path string) {
			if err := os.Remove(path); err != nil {
				s.Logf("Failed to remove file %s: %v", name, err)
			}
		}(cleanupCtx, file.name, file.path)
	}

	// Pin files to Holding Space and verify open operation from it.
	if err := pinAndOpenFilesToHoldingspace(ctx, &resources, importedFiles); err != nil {
		s.Fatal("Failed to pin and verify open files: ", err)
	}

	// Verify all targets are pinned to Holding Space.
	if err := verifyHoldingspaceItems(ctx, &resources, importedFiles); err != nil {
		s.Fatal("Failed to verify pinned files: ", err)
	}

	// Predefine all downloadable files and its URL.
	baseURL, err := url.Parse(regularFileURLPrefix)
	fullURL := baseURL
	downloadableFiles := []*fileTarget{
		{name: downloadedFile1, source: path.Join(baseURL.Path, "arc", downloadedFile1), path: filepath.Join(filesapp.DownloadPath, downloadedFile1)},
		{name: downloadedFile2, source: path.Join(baseURL.Path, "apps", downloadedFile2), path: filepath.Join(filesapp.DownloadPath, downloadedFile2)},
	}
	resources.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, file := range downloadableFiles {
			fullURL.Path = file.source
			io.WriteString(w, fmt.Sprintf(`<a href=%s>%s</a><br>`, fullURL, file.name))
		}
	}))
	defer resources.server.Close()

	// Pin multiple files to Holding Space.
	if err := downloadAndPinToHoldingspace(ctx, &resources, downloadableFiles, downloadedFile1, downloadedFile2); err != nil {
		s.Fatal("Failed to pin downloaded files to Holding Space: ", err)
	}
	defer func() {
		for _, file := range downloadableFiles {
			if err := os.Remove(file.path); err != nil {
				s.Logf("Failed to remove file %s: %v", file.name, err)
			}
		}
	}()

	// Verify "Show in Folder" will NOT appear when selecting multiple items.
	if err := verifyMultipleSelect(&resources, testFile1, testFile3)(ctx); err != nil {
		s.Fatal("Failed to verify `Show in Folder`  option: ", err)
	}

	// Unpin items from Holding Space.
	if err := unpinFilesFromHoldingspace(&resources, testFile1, testFile3)(ctx); err != nil {
		s.Fatal("Failed unpin items from Holding Space: ", err)
	}
}

type fileTarget struct {
	name   string
	source string
	path   string
}

// pinAndOpenFilesToHoldingspace pins given files to Holding Space and launches it from there.
func pinAndOpenFilesToHoldingspace(ctx context.Context, res *ofhResource, fileTargets []*fileTarget) error {
	for _, file := range fileTargets {
		if err := uiauto.Combine(fmt.Sprintf("pin file %s to Holding Space and double-click it", file.name),
			res.files.OpenDownloads(),
			res.files.PinToShelf(file.name),
			res.ui.LeftClick(holdingspace.FindTray()),
			res.ui.DoubleClick(holdingspace.FindPinnedFileChip().Name(file.name)),
		)(ctx); err != nil {
			return err
		}

		if err := apps.Close(ctx, res.tconn, apps.Gallery.ID); err != nil {
			return errors.Wrapf(err, "failed to close Gallery after verifying file %q", file.name)
		}
	}

	return nil
}

// verifyHoldingspaceItems verify all targets are pinned to Holding Space by comparing number of items.
func verifyHoldingspaceItems(ctx context.Context, res *ofhResource, fileTargets []*fileTarget) error {
	if err := res.ui.LeftClick(holdingspace.FindTray())(ctx); err != nil {
		return errors.Wrap(err, "failed to expand Holdingspace")
	}

	if items, err := res.ui.WithTimeout(3*time.Second).NodesInfo(ctx, holdingspace.FindPinnedFileChip()); err != nil {
		return errors.Wrap(err, "failed to get nodes info of Holding Space")
	} else if len(items) != len(fileTargets) {
		return errors.Errorf("unexpected number of pinned items: got %d, want %d", len(items), len(fileTargets))
	}

	return nil
}

// downloadAndPinToHoldingspace downloads and pin multiple files to Holding Space.
func downloadAndPinToHoldingspace(ctx context.Context, res *ofhResource, fileTargets []*fileTarget, firstSelectItem, secondSelectItem string) error {
	// Open browser and download a sample file.
	conn, err := res.cr.NewConn(ctx, res.server.URL)
	if err != nil {
		return errors.Wrap(err, "failed to open Chrome browser")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := webutil.WaitForRender(ctx, conn, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for render")
	}

	for _, file := range fileTargets {
		closeNotificationButton := nodewith.Name("Notification close").HasClass("PaddedButton").Role(role.Button)
		dlLink := nodewith.Name(file.name).Role(role.Link)
		saveLink := nodewith.Name("Save link as…").HasClass("MenuItemView").Role(role.MenuItem)
		saveBtn := nodewith.Name("Save").HasClass("ok primary").Role(role.Button)
		if err := uiauto.Combine(fmt.Sprintf("download sample file %q", file.name),
			res.ui.RightClick(dlLink),
			res.ui.LeftClick(saveLink),
			res.ui.LeftClick(saveBtn),
			res.ui.RetryUntil(
				res.ui.LeftClick(holdingspace.FindTray()),
				res.ui.WithTimeout(3*time.Second).WaitForLocation(holdingspace.FindDownloadChip().NameStartingWith(file.name)),
			),
			res.ui.LeftClick(holdingspace.FindTray()),
			res.ui.LeftClickUntil(closeNotificationButton, res.ui.WithTimeout(5*time.Second).WaitUntilGone(closeNotificationButton)),
		)(ctx); err != nil {
			return err
		}
	}

	if err := uiauto.Combine("select multiple downloaded files and pin them",
		res.ui.LeftClick(holdingspace.FindTray()),
		res.kb.AccelPressAction("shift"),
		res.ui.LeftClick(holdingspace.FindDownloadChip().Name(firstSelectItem)),
		res.ui.LeftClick(holdingspace.FindDownloadChip().Name(secondSelectItem)),
		res.ui.RightClick(holdingspace.FindDownloadChip().Name(secondSelectItem)),
		res.ui.LeftClick(holdingspace.FindContextMenuItem().Name("Pin")),
		res.kb.AccelReleaseAction("shift"),
	)(ctx); err != nil {
		return err
	}

	return nil
}

// verifyMultipleSelect verifies "Show in Folder" will NOT appear when selecting multiple items.
func verifyMultipleSelect(res *ofhResource, firstSelectItem, secondSelectItem string) action.Action {
	return uiauto.Combine("click multiple Holding Space items and right-click",
		res.ui.RetryUntil(
			res.ui.LeftClick(holdingspace.FindTray()),
			res.ui.WithTimeout(time.Second).WaitUntilExists(holdingspace.FindPinnedFileChip().First()),
		),
		res.kb.AccelPressAction("shift"),
		res.ui.LeftClick(holdingspace.FindPinnedFileChip().Name(firstSelectItem)),
		res.ui.LeftClick(holdingspace.FindPinnedFileChip().Name(secondSelectItem)),
		res.ui.RightClick(holdingspace.FindPinnedFileChip().Name(secondSelectItem)),
		res.ui.WaitUntilGone(holdingspace.FindContextMenuItem().Name("Show in Folder")),
		res.kb.AccelReleaseAction("shift"),
		res.kb.AccelAction("esc"),
	)
}

// unpinFilesFromHoldingspace verifies "unpin files from Holdingspace" operation.
func unpinFilesFromHoldingspace(res *ofhResource, firstSelectItem, secondSelectItem string) action.Action {
	return uiauto.Combine("click multiple Holding Space items and unpin",
		res.ui.RetryUntil(
			res.ui.LeftClick(holdingspace.FindTray()),
			res.ui.WithTimeout(time.Second).WaitUntilExists(holdingspace.FindPinnedFileChip().First()),
		),
		res.kb.AccelPressAction("shift"),
		res.ui.LeftClick(holdingspace.FindPinnedFileChip().Name(firstSelectItem)),
		res.ui.LeftClick(holdingspace.FindPinnedFileChip().Name(secondSelectItem)),
		res.ui.RightClick(holdingspace.FindPinnedFileChip().Name(secondSelectItem)),
		res.ui.LeftClick(holdingspace.FindContextMenuItem().Name("Unpin")),
		res.kb.AccelReleaseAction("shift"),
	)
}
