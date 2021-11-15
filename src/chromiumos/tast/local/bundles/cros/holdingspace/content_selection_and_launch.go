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
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// downloadFileName is an online resource file name which will be downloaded and used for the test case.
	downloadFileName = "720_av1_20201117.mp4"
	// downloadFileSource is a complete online resource file path which will be downloaded and used for the test case.
	downloadFileSource = "https://storage.googleapis.com/chromiumos-test-assets-public/tast/cros/video/" + downloadFileName
	// downloadedFileNum is the number of downloaded files used in the test case.
	downloadedFileNum = 3

	shortTimeout = 3 * time.Second
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ContentSelectionAndLaunch,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verifies pin, unpin, select and launch single/multi-file(s) operations can be done on Holding Space",
		Contacts: []string{
			"lance.wang@cienet.com",
			"dmblack@google.com",
			"tote-eng@google.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Timeout:      10 * time.Minute,
	})
}

// contentSelectionResource holds resources used by test case holdingspace.ContentSelectionAndLaunch.
type contentSelectionResource struct {
	cr     *chrome.Chrome
	tconn  *chrome.TestConn
	kb     *input.KeyboardEventWriter
	ui     *uiauto.Context
	tc     *touch.Context
	files  *filesapp.FilesApp
	outDir string
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

	touch, err := touch.New(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to initialize touch: ", err)
	}
	defer touch.Close()

	resources := &contentSelectionResource{
		cr:     cr,
		tconn:  tconn,
		kb:     kb,
		ui:     uiauto.New(tconn),
		tc:     touch,
		outDir: s.OutDir(),
	}

	// Set up downloaded files for later use.
	var downloadedFiles []string
	defer func() {
		for _, file := range downloadedFiles {
			filePath := filepath.Join(filesapp.DownloadPath, file)
			if err := os.Remove(filePath); err != nil {
				s.Logf("Failed to remove file with path %q: %v", filePath, err)
			}
		}
	}()

	// Generates contents in the "downloaded files" section in "holdingspace" through a simulated download action.
	if downloadedFiles, err = generateDownloadedFiles(ctx, resources, downloadFileSource, downloadedFileNum); err != nil {
		s.Fatal("Failed to generate downloaded files: ", err)
	}

	if err := verifyExistenceOnHoldingspace(ctx, resources.ui, downloadedFiles, holdingspace.FindDownloadChip(), true); err != nil {
		s.Fatal("Failed to verify downloaded files' existence on Holdingspace: ", err)
	}

	for _, test := range []contentOnHoldingspaceTest{
		&clamshellTest{resources},
		&tabletTest{resources},
	} {
		subTestName := fmt.Sprintf("holding space operation in %q mode", test.modeName())
		s.Run(ctx, subTestName, func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, test.isTabletMode())
			if err != nil {
				s.Fatal("Failed to ensure tablet mode: ", err)
			}
			defer cleanup(cleanupCtx)

			s.Logf("Performing pin verification on files %v ", downloadedFiles)
			if err := test.pinFiles(ctx, downloadedFiles); err != nil {
				s.Fatal("Failed to complete pin action: ", err)
			}

			for _, file := range downloadedFiles {
				s.Logf("Performing single click verification on file %s", file)
				if err := test.singleClick(ctx, file); err != nil {
					s.Fatal("Failed to complete single click action: ", err)
				}

				s.Logf("Performing open file verification on file %s", file)
				if err := test.openFile(ctx, file); err != nil {
					s.Fatal("Failed to complete open file action: ", err)
				}
			}

			s.Logf("Performing multi-select verification on files %v ", downloadedFiles)
			if err := test.multiSelect(ctx, downloadedFiles); err != nil {
				s.Fatal("Failed to complete multi-select action: ", err)
			}

			s.Logf("Performing unpin and menu-item verification on files %v ", downloadedFiles)
			if err := test.unpinFiles(ctx, downloadedFiles); err != nil {
				s.Fatal("Failed to complete unpin action: ", err)
			}
		})
	}
}

// generateDownloadedFiles generates local server and downloads files to holdingspace.
func generateDownloadedFiles(ctx context.Context, res *contentSelectionResource, source string, numberOfFiles int) (downloadedFiles []string, retErr error) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, fmt.Sprintf(`<a href=%s>%s</a><br>`, source, downloadFileName))
	}))
	defer server.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Open browser with server URL.
	conn, err := res.cr.NewConn(ctx, server.URL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open browser")
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, res.outDir, func() bool { return retErr != nil }, res.cr, "test_server_ui_dump")

	if err := webutil.WaitForRender(ctx, conn, 10*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to wait for finish render")
	}

	// Download files.
	for i := 0; i < numberOfFiles; i++ {
		file := fmt.Sprintf("video%d.mp4", i+1)
		if err := downloadTarget(ctx, res, file); err != nil {
			return downloadedFiles, errors.Wrap(err, "failed to download")
		}
		downloadedFiles = append(downloadedFiles, file)
	}

	return downloadedFiles, nil
}

// downloadTarget downloads the target file via browser.
func downloadTarget(ctx context.Context, res *contentSelectionResource, targetName string) error {
	saveBtn := nodewith.Name("Save").HasClass("ok primary").Role(role.Button)
	notificationView := nodewith.HasClass("MessageView").NameStartingWith("Download complete").Role(role.GenericContainer)
	targetDownloadCompleteMessage := nodewith.HasClass("Label").Name(targetName).Role(role.StaticText).Ancestor(notificationView)

	if err := uiauto.Combine(fmt.Sprintf("download target %q", targetName),
		res.ui.RightClick(nodewith.Name(downloadFileName).Role(role.Link)),
		res.ui.LeftClick(nodewith.Name("Save link as…").HasClass("MenuItemView").Role(role.MenuItem)),
		// Rename the file.
		res.ui.WaitForLocation(saveBtn),
		res.kb.AccelAction("ctrl+a"),
		res.kb.AccelAction("backspace"),
		res.kb.TypeAction(targetName),
		// Save and verify the file has been downloaded.
		res.ui.LeftClick(saveBtn),
		res.ui.WaitUntilExists(targetDownloadCompleteMessage),
	)(ctx); err != nil {
		return err
	}

	// Clear the notification.
	return ash.CloseNotifications(ctx, res.tconn)
}

// verifyExistenceOnHoldingspace checks if given files are on the specified section of Holdingspace.
func verifyExistenceOnHoldingspace(ctx context.Context, ui *uiauto.Context, fileNames []string, chip *nodewith.Finder, shouldExist bool) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	dismissHoldingSpace, err := openHoldingSpace(ctx, ui)
	if err != nil {
		return errors.Wrap(err, "failed to open holdingspace")
	}
	defer dismissHoldingSpace(cleanupCtx)

	verifyChip := ui.WaitUntilGone
	if shouldExist {
		verifyChip = ui.WaitUntilExists
	}

	for _, fileName := range fileNames {
		if err := verifyChip(chip.NameStartingWith(fileName))(ctx); err != nil {
			return errors.Wrapf(err, "failed to verify existence of file %q", fileName)
		}
	}

	return nil
}

// verifyGalleryAppShown verifies if Gallery app with given file name is launched.
// This is an universal function for both clamshell and tablet mode tests.
func verifyGalleryAppShown(ui *uiauto.Context, fileName string) action.Action {
	galleryWindow := nodewith.HasClass("BrowserFrame").Name(fmt.Sprintf("%s - %s", apps.Gallery.Name, fileName)).Role(role.Window)
	return ui.WaitUntilExists(galleryWindow)
}

type contentOnHoldingspaceTest interface {
	// modeName returns the name of the mode.
	modeName() string

	// isTabletMode returns a boolean represents the test is in tablet mode or not.
	isTabletMode() bool

	// singleClick performs single click action and verify if thumbtack icon is shown.
	singleClick(ctx context.Context, fileName string) error

	// openFile performs double click action and verify if thumbtack icon is shown.
	openFile(ctx context.Context, fileName string) error

	// multiSelect performs multi select action and verify if thumbtack icon is shown.
	multiSelect(ctx context.Context, fileNames []string) error

	// pinFiles pins given files to Tote.
	pinFiles(ctx context.Context, fileNames []string) error

	// unpinFiles unpins given files from Tote.
	// This function also verifies context menu items when multi-selecting.
	unpinFiles(ctx context.Context, fileNames []string) error
}

// clamshellTest holds resources for clamshell mode test.
type clamshellTest struct{ *contentSelectionResource }

func (t *clamshellTest) modeName() string   { return "clamshell" }
func (t *clamshellTest) isTabletMode() bool { return false }

func (t *clamshellTest) singleClick(ctx context.Context, fileName string) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dismissHoldingSpace, err := openHoldingSpace(ctx, t.ui)
	if err != nil {
		return errors.Wrap(err, "failed to open holdingspace")
	}
	defer dismissHoldingSpace(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, t.outDir, func() bool { return retErr != nil }, t.cr, fmt.Sprintf("ui_dump_%s_single_click", t.modeName()))

	return t.leftClickAndVerify(t.ui, holdingspace.FindPinnedFileChip(), fileName)(ctx)
}

func (t *clamshellTest) openFile(ctx context.Context, fileName string) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dismissHoldingSpace, err := openHoldingSpace(ctx, t.ui)
	if err != nil {
		return errors.Wrap(err, "failed to open holdingspace")
	}
	defer dismissHoldingSpace(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, t.outDir, func() bool { return retErr != nil }, t.cr, fmt.Sprintf("ui_dump_%s_open_file", t.modeName()))

	// fileChip is the chip node shown on Holdingspace
	fileChip := holdingspace.FindPinnedFileChip().NameStartingWith(fileName)
	if err := uiauto.Combine("double click the file and verify gallery app is opened",
		t.ui.DoubleClick(fileChip),
		verifyGalleryAppShown(t.ui, fileName),
	)(ctx); err != nil {
		return err
	}

	return apps.Close(cleanupCtx, t.tconn, apps.Gallery.ID)
}

func (t *clamshellTest) multiSelect(ctx context.Context, fileNames []string) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dismissHoldingSpace, err := openHoldingSpace(ctx, t.ui)
	if err != nil {
		return errors.Wrap(err, "failed to open holdingspace")
	}
	defer dismissHoldingSpace(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, t.outDir, func() bool { return retErr != nil }, t.cr, fmt.Sprintf("ui_dump_%s_multi_select", t.modeName()))

	if err := t.selectAllFiles(ctx, holdingspace.FindPinnedFileChip(), fileNames); err != nil {
		return errors.Wrap(err, "failed to select all files")
	}

	return nil
}

func (t *clamshellTest) pinFiles(ctx context.Context, fileNames []string) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dismissHoldingSpace, err := openHoldingSpace(ctx, t.ui)
	if err != nil {
		return errors.Wrap(err, "failed to open holdingspace")
	}
	defer dismissHoldingSpace(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, t.outDir, func() bool { return retErr != nil }, t.cr, fmt.Sprintf("ui_dump_%s_pin_", t.modeName()))

	if err := t.selectAllFiles(ctx, holdingspace.FindDownloadChip(), fileNames); err != nil {
		return errors.Wrap(err, "failed to select all files")
	}

	menuOption := holdingspace.FindContextMenuItem()
	pinOption := menuOption.Name("Pin")
	if err := uiauto.Combine("pin files to Holdingspace",
		t.ui.RetryUntil(
			t.ui.RightClick(holdingspace.FindChip().NameStartingWith(fileNames[0])),
			t.ui.Exists(pinOption),
		),
		t.ui.LeftClick(pinOption),
		t.ui.WaitUntilGone(pinOption),
	)(ctx); err != nil {
		return err
	}

	// Verify all downloaded file are now in Holdingspace.
	return verifyExistenceOnHoldingspace(ctx, t.ui, fileNames, holdingspace.FindPinnedFileChip(), true)
}

func (t *clamshellTest) unpinFiles(ctx context.Context, fileNames []string) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dismissHoldingSpace, err := openHoldingSpace(ctx, t.ui)
	if err != nil {
		return errors.Wrap(err, "failed to open holdingspace")
	}
	defer dismissHoldingSpace(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, t.outDir, func() bool { return retErr != nil }, t.cr, fmt.Sprintf("ui_dump_%s_unpin_", t.modeName()))

	if err := t.selectAllFiles(ctx, holdingspace.FindPinnedFileChip(), fileNames); err != nil {
		return errors.Wrap(err, "failed to select all files")
	}

	menuOption := holdingspace.FindContextMenuItem()
	unpinOption := menuOption.Name("Unpin")
	if err := uiauto.Combine("verify menu options and unpin them",
		// Right-click to open menu.
		t.ui.RightClick(holdingspace.FindPinnedFileChip().NameStartingWith(fileNames[0])),
		// Verify "Copy", "Paste" and "Show in folder" will not show up while multi-selecting.
		t.ui.EnsureGoneFor(menuOption.Name("Show in folder"), shortTimeout),
		t.ui.EnsureGoneFor(menuOption.Name("Copy"), shortTimeout),
		t.ui.EnsureGoneFor(menuOption.Name("Paste"), shortTimeout),
		// Unpin items.
		t.ui.LeftClick(unpinOption),
		t.ui.WaitUntilGone(unpinOption),
	)(ctx); err != nil {
		return err
	}

	// Verify all downloaded file are no longer in Holdingspace.
	return verifyExistenceOnHoldingspace(ctx, t.ui, fileNames, holdingspace.FindPinnedFileChip(), false)
}

// leftClickAndVerify left clicks the specified file and verifies if it's selected.
func (t *clamshellTest) leftClickAndVerify(ui *uiauto.Context, chip *nodewith.Finder, fileName string) action.Action {
	fileChip := chip.NameStartingWith(fileName)                                    // fileChip is the chip node shown on Holdingspace.
	toggleImageButton := nodewith.HasClass("ToggleImageButton").Ancestor(fileChip) // toggleImageButton is the button shown within the chip when it's clicked.

	return uiauto.Combine("left click the file and verify it's selected",
		ui.LeftClick(fileChip),
		ui.WaitUntilExists(toggleImageButton),
	)
}

// selectAllFiles selects all specified files.
func (t *clamshellTest) selectAllFiles(ctx context.Context, chip *nodewith.Finder, fileNames []string) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := t.kb.AccelPress(ctx, "shift"); err != nil {
		return errors.Wrap(err, "failed to long-press ctrl")
	}
	defer t.kb.AccelRelease(cleanupCtx, "shift")

	for _, file := range fileNames {
		if err := t.leftClickAndVerify(t.ui, chip, file)(ctx); err != nil {
			return err
		}
	}

	return nil
}

// tabletTest holds resources for tablet mode test.
type tabletTest struct{ *contentSelectionResource }

func (t *tabletTest) modeName() string   { return "tablet" }
func (t *tabletTest) isTabletMode() bool { return true }

func (t *tabletTest) singleClick(ctx context.Context, fileName string) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dismissHoldingSpace, err := openHoldingSpace(ctx, t.ui)
	if err != nil {
		return errors.Wrap(err, "failed to open holdingspace")
	}
	defer dismissHoldingSpace(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, t.outDir, func() bool { return retErr != nil }, t.cr, fmt.Sprintf("ui_dump_%s_single_click", t.modeName()))

	// fileChip is the chip node shown on Holdingspace
	fileChip := holdingspace.FindPinnedFileChip().NameStartingWith(fileName)
	if err := uiauto.Combine("left click the file and verify gallery app is opened",
		t.tc.Tap(fileChip),
		verifyGalleryAppShown(t.ui, fileName),
	)(ctx); err != nil {
		return err
	}

	return apps.Close(ctx, t.tconn, apps.Gallery.ID)
}

func (t *tabletTest) openFile(ctx context.Context, fileName string) error {
	// Single click on a content in holding space will open it under tablet mode, which is the same as "(*tabletTest).singleClick".
	// Therefore, we skip this test as it will be performed by other test.
	return nil
}

func (t *tabletTest) multiSelect(ctx context.Context, fileNames []string) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dismissHoldingSpace, err := openHoldingSpace(ctx, t.ui)
	if err != nil {
		return errors.Wrap(err, "failed to open holdingspace")
	}
	defer dismissHoldingSpace(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, t.outDir, func() bool { return retErr != nil }, t.cr, fmt.Sprintf("ui_dump_%s_multi_select", t.modeName()))

	if err := t.selectAllFiles(ctx, holdingspace.FindPinnedFileChip(), fileNames); err != nil {
		return errors.Wrap(err, "failed to select all files")
	}

	return nil
}

func (t *tabletTest) pinFiles(ctx context.Context, fileNames []string) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dismissHoldingSpace, err := openHoldingSpace(ctx, t.ui)
	if err != nil {
		return errors.Wrap(err, "failed to open holdingspace")
	}
	defer dismissHoldingSpace(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, t.outDir, func() bool { return retErr != nil }, t.cr, fmt.Sprintf("ui_dump_%s_pin_", t.modeName()))

	if err := t.selectAllFiles(ctx, holdingspace.FindDownloadChip(), fileNames); err != nil {
		return errors.Wrap(err, "failed to select all files")
	}

	menuOption := holdingspace.FindContextMenuItem()
	pinOption := menuOption.Name("Pin")
	if err := uiauto.Combine("pin files to Holdingspace",
		t.ui.RetryUntil(
			t.ui.RightClick(holdingspace.FindChip().NameStartingWith(fileNames[0])),
			t.ui.Exists(pinOption),
		),
		t.ui.LeftClick(pinOption),
		t.ui.WaitUntilGone(pinOption),
	)(ctx); err != nil {
		return err
	}

	// Verify all downloaded file are now in Holdingspace.
	return verifyExistenceOnHoldingspace(ctx, t.ui, fileNames, holdingspace.FindPinnedFileChip(), true)
}

func (t *tabletTest) unpinFiles(ctx context.Context, fileNames []string) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dismissHoldingSpace, err := openHoldingSpace(ctx, t.ui)
	if err != nil {
		return errors.Wrap(err, "failed to open holdingspace")
	}
	defer dismissHoldingSpace(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, t.outDir, func() bool { return retErr != nil }, t.cr, fmt.Sprintf("ui_dump_%s_unpin_", t.modeName()))

	if err := t.selectAllFiles(ctx, holdingspace.FindPinnedFileChip(), fileNames); err != nil {
		return errors.Wrap(err, "failed to select all files")
	}

	menuOption := holdingspace.FindContextMenuItem()
	showInFolderOption := menuOption.Name("Show in folder")
	copyOption := menuOption.Name("Copy")
	pasteOption := menuOption.Name("Paste")
	unpinOption := menuOption.Name("Unpin")
	if err := uiauto.Combine("verify menu options and unpin them",
		// Right-click to open menu.
		t.ui.RightClick(holdingspace.FindPinnedFileChip().NameStartingWith(fileNames[0])),
		// Verify "Copy", "Paste" and "Show in folder" will not show up while multi-selecting.
		t.ui.EnsureGoneFor(showInFolderOption, shortTimeout),
		t.ui.EnsureGoneFor(copyOption, shortTimeout),
		t.ui.EnsureGoneFor(pasteOption, shortTimeout),
		// Unpin items.
		t.ui.LeftClick(unpinOption),
		t.ui.WaitUntilGone(unpinOption),
	)(ctx); err != nil {
		return err
	}

	// Verify all downloaded file are no longer in Holdingspace.
	return verifyExistenceOnHoldingspace(ctx, t.ui, fileNames, holdingspace.FindPinnedFileChip(), false)
}

// selectAllFiles selects all specified files.
func (t *tabletTest) selectAllFiles(ctx context.Context, chip *nodewith.Finder, fileNames []string) error {
	// verifySelected verifies "rounded" disappears when the chip item is selected.
	verifySelected := func(fileChip *nodewith.Finder) action.Action {
		roundImage := nodewith.HasClass("RoundedImageView").Role(role.Unknown).Ancestor(fileChip)
		return t.ui.WaitUntilGone(roundImage)
	}

	firstFile := fileNames[0]
	// Under tablet mode, when we select a file via touch screen,
	// there will be a popup menu shown up, we need to exit it
	// in order to select the rest of files.
	if err := uiauto.Combine("select the first file and exit the popup menu",
		t.tc.LongPress(chip.NameStartingWith(firstFile)),
		t.ui.WaitUntilExists(holdingspace.FindContextMenu()),
		t.kb.AccelAction("esc"),
		t.ui.WaitUntilGone(holdingspace.FindContextMenu()),
		verifySelected(chip.NameStartingWith(firstFile)),
	)(ctx); err != nil {
		return err
	}

	// Loop and tap the rest files to complete multi-selection.
	for i := 1; i < len(fileNames); i++ {
		if err := uiauto.Combine(fmt.Sprintf("tap file %q", fileNames[i]),
			t.tc.Tap(chip.NameStartingWith(fileNames[i])),
			verifySelected(chip.NameStartingWith(fileNames[i])),
		)(ctx); err != nil {
			return err
		}
	}

	return nil
}

// openHoldingSpace opens the holding space.
// This function is safe to call even if it's already opened.
func openHoldingSpace(ctx context.Context, ui *uiauto.Context) (uiauto.Action, error) {
	cleanup := ui.MouseClickAtLocation(0, coords.Point{X: 0, Y: 0})

	dialog := nodewith.HasClass("RootView").Name("Tote: recent screen captures, downloads, and pinned files").Role(role.Dialog)
	if isFound, err := ui.IsNodeFound(ctx, dialog); err != nil {
		return cleanup, err
	} else if !isFound {
		return cleanup, uiauto.Combine("open holding space",
			ui.LeftClick(holdingspace.FindTray()),
			ui.WaitUntilExists(dialog),
		)(ctx)
	}

	return cleanup, nil
}
