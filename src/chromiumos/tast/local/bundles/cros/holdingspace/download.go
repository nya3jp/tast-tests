// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type downloadParams struct {
	testfunc    func(*downloadResource, []string, uiauto.Action) uiauto.Action
	browserType browser.Type
	files       []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Download,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verifies download behavior in holding space",
		Contacts: []string{
			"dmblack@google.com",
			"tote-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "cancel",
			Val: downloadParams{
				testfunc:    testDownloadCancel,
				browserType: browser.TypeAsh,
				files:       []string{"download.txt"},
			},
		}, {
			Name: "pause_and_resume",
			Val: downloadParams{
				testfunc:    testDownloadPauseAndResume,
				browserType: browser.TypeAsh,
				files:       []string{"download.txt"},
			},
		}, {
			Name: "pin_and_unpin",
			Val: downloadParams{
				testfunc:    testDownloadPinAndUnpin,
				browserType: browser.TypeAsh,
				files:       []string{"download.txt"},
			},
		}, {
			Name: "pin_unpin_multiple",
			Val: downloadParams{
				testfunc:    testDownloadPinAndUnpinMultiple,
				browserType: browser.TypeAsh,
				files:       []string{"download1.txt", "download2.txt", "download3.txt"},
			},
		}, {
			Name: "remove",
			Val: downloadParams{
				testfunc:    testDownloadRemove,
				browserType: browser.TypeAsh,
				files:       []string{"download.txt"},
			},
		}, {
			Name: "launch",
			Val: downloadParams{
				testfunc:    testDownloadLaunch,
				browserType: browser.TypeAsh,
				files:       []string{"download.txt"},
			},
		}, {
			Name: "launch_multiple",
			Val: downloadParams{
				testfunc:    testDownloadLaunchMultiple,
				browserType: browser.TypeAsh,
				files:       []string{"download1.txt", "download2.txt", "download3.txt"},
			},
		}, {
			Name: "lacros_cancel",
			Val: downloadParams{
				testfunc:    testDownloadCancel,
				browserType: browser.TypeLacros,
				files:       []string{"download.txt"},
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "lacros_pause_and_resume",
			Val: downloadParams{
				testfunc:    testDownloadPauseAndResume,
				browserType: browser.TypeLacros,
				files:       []string{"download.txt"},
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "lacros_pin_and_unpin",
			Val: downloadParams{
				testfunc:    testDownloadPinAndUnpin,
				browserType: browser.TypeLacros,
				files:       []string{"download.txt"},
			},
			ExtraSoftwareDeps: []string{"lacros", "lacros_stable"},
		}, {
			Name: "lacros_pin_and_unpin_unstable",
			Val: downloadParams{
				testfunc:    testDownloadPinAndUnpin,
				browserType: browser.TypeLacros,
				files:       []string{"download.txt"},
			},
			ExtraSoftwareDeps: []string{"lacros", "lacros_unstable"},
		}, {
			Name: "lacros_pin_unpin_multiple",
			Val: downloadParams{
				testfunc:    testDownloadPinAndUnpinMultiple,
				browserType: browser.TypeLacros,
				files:       []string{"download1.txt", "download2.txt", "download3.txt"},
			},
			ExtraSoftwareDeps: []string{"lacros", "lacros_stable"},
		}, {
			Name: "lacros_pin_unpin_multiple_unstable",
			Val: downloadParams{
				testfunc:    testDownloadPinAndUnpinMultiple,
				browserType: browser.TypeLacros,
				files:       []string{"download1.txt", "download2.txt", "download3.txt"},
			},
			ExtraSoftwareDeps: []string{"lacros", "lacros_unstable"},
		}, {
			Name: "lacros_remove",
			Val: downloadParams{
				testfunc:    testDownloadRemove,
				browserType: browser.TypeLacros,
				files:       []string{"download.txt"},
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "lacros_launch",
			Val: downloadParams{
				testfunc:    testDownloadLaunch,
				browserType: browser.TypeLacros,
				files:       []string{"download.txt"},
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "lacros_launch_multiple",
			Val: downloadParams{
				testfunc:    testDownloadLaunchMultiple,
				browserType: browser.TypeLacros,
				files:       []string{"download1.txt", "download2.txt", "download3.txt"},
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
		Timeout: 3 * time.Minute,
	})
}

// downloadResource holds resources used by test case holdingspace.Download.*.
type downloadResource struct {
	kb          *input.KeyboardEventWriter
	ui          *uiauto.Context
	browserType browser.Type
	outDir      string
}

// Download verifies download behavior in holding space. It is expected that
// initiating a download will result in an item being added to holding space
// from which the user can cancel/pause/resume the download. Upon download
// completion, the user should be able to pin the download.
func Download(ctx context.Context, s *testing.State) {
	params := s.Param().(downloadParams)
	bt := params.browserType

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Connect to a fresh ash-chrome instance (cr) to ensure holding space first-run state,
	// also get a browser instance (br) for browser functionality in common.
	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig())
	if err != nil {
		s.Fatalf("Failed to connect to %v browser: %v", bt, err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	res := &downloadResource{
		kb:          kb,
		ui:          uiauto.New(tconn),
		browserType: bt,
		outDir:      s.OutDir(),
	}

	// Ensure the tray does not exist prior adding anything to holding space.
	if err = res.ui.EnsureGoneFor(holdingspace.FindTray(), 5*time.Second)(ctx); err != nil {
		s.Fatal("Tray exists: ", err)
	}

	// Cache the name and location of the download.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}

	// Create a local server. If a request indicates `redirect=true`, the response
	// HTML will cause automatic redirection back to the root URL after a short
	// delay. Otherwise, the response will result in a download being started that
	// will block completion until the `unblockDownloadChannel` is signaled.
	initiateDownloadHandler := func(downloadFileName string) (http.HandlerFunc, chan struct{}) {
		unblockDownloadChannel := make(chan struct{})
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "text/html")
			if redirect := r.URL.Query().Get("redirect"); redirect == "true" {
				fmt.Fprintf(w, "<meta http-equiv='refresh' content='1;url=/' />")
				return
			}
			w.Header().Add("Content-Disposition", "attachment; filename="+downloadFileName)
			fmt.Fprintf(w, "Download started\n")
			f := w.(http.Flusher)
			f.Flush()
			<-unblockDownloadChannel
			fmt.Fprintf(w, "Download finished\n")
		}), unblockDownloadChannel
	}

	var unblockDownloadChannels []chan struct{}

	for _, file := range params.files {
		handler, unblockDownloadChannel := initiateDownloadHandler(file)
		unblockDownloadChannels = append(unblockDownloadChannels, unblockDownloadChannel)

		server := httptest.NewServer(handler)
		defer server.Close()

		// Connect to the local server. Note that this method will block until the
		// browser has finished navigating to the desired URL. Since we actually want
		// to start a download and not navigate the browser we'll use a redirect
		// workaround to satisfy the requirement to navigate.
		conn, err := br.NewConn(ctx, server.URL+"?redirect=true")
		if err != nil {
			s.Fatal("Failed to connect to local server: ", err)
		}
		defer func(ctx context.Context, file string) {
			conn.Close()
			conn.CloseTarget(ctx)
			filePath := filepath.Join(downloadsPath, file)
			os.Remove(filePath)
		}(cleanupCtx, file)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	if err := uiauto.Combine("open bubble and confirm initial state",
		// Left click the tray to open the bubble.
		res.ui.LeftClick(holdingspace.FindTray()),

		// The pinned files section should contain an educational prompt and chip
		// informing the user that they can pin a file from the Files app.
		res.ui.WaitUntilExists(holdingspace.FindPinnedFilesSectionFilesAppPrompt()),
		res.ui.WaitUntilExists(holdingspace.FindPinnedFilesSectionFilesAppChip()),
	)(ctx); err != nil {
		s.Fatal("Failed to open bubble and confirm initial state: ", err)
	}

	// Perform additional parameterized testing.
	if err := params.testfunc(res, params.files, func(ctx context.Context) error {
		for _, ch := range unblockDownloadChannels {
			close(ch)
		}
		return nil
	})(ctx); err != nil {
		s.Fatal("Fail to perform parameterized testing: ", err)
	}

	// Remove the file at `downloadLocation` which is backing the download. Note that
	// this will result in any associated holding space items being removed.
	for _, file := range params.files {
		filePath := filepath.Join(downloadsPath, file)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			s.Fatalf("Failed to remove file with path %q: %s", filePath, err)
		}

		// Ensure all holding space chips associated with the underlying download are
		// removed when the backing file is removed.
		if err := res.ui.WaitUntilGone(holdingspace.FindChip().Name(file))(ctx); err != nil {
			s.Fatalf("Chip %q exists: %s", file, err)
		}
	}
}

// testDownloadCancel performs testing of cancelling a download.
func testDownloadCancel(
	res *downloadResource, downloadedFiles []string, unblockDownload uiauto.Action) uiauto.Action {
	if len(downloadedFiles) != 1 {
		return errorAction("testDownloadCancel requires exactly one file to be downloaded")
	}
	downloadName := downloadedFiles[0]

	return uiauto.Combine("test cancel",
		// Right click the download chip to show the context menu. Note that the
		// download chip is currently bound to an in-progress download.
		res.ui.RightClick(holdingspace.FindDownloadChip().Name("Downloading "+downloadName)),

		// Left click the "Cancel" context menu item. Note that this will result in
		// the underlying download being cancelled and the context menu being
		// closed.
		res.ui.LeftClick(holdingspace.FindContextMenuItem().Name("Cancel")),

		// Unblock the download so that the local server can complete the download
		// request. This is necessary even though the download has been cancelled to
		// keep the local server from hanging.
		unblockDownload,

		// Ensure the download chip is removed with its backing file.
		res.ui.WaitUntilGone(holdingspace.FindDownloadChip().Name(downloadName)),
	)
}

// testDownloadPauseAndResume performs testing of pausing and resuming a download.
func testDownloadPauseAndResume(
	res *downloadResource, downloadedFiles []string, unblockDownload uiauto.Action) uiauto.Action {
	if len(downloadedFiles) != 1 {
		return errorAction("testDownloadPauseAndResume requires exactly one file to be downloaded")
	}
	downloadName := downloadedFiles[0]

	return uiauto.Combine("test pause and resume",
		// Right click the download chip to show the context menu. Note that the
		// download chip is currently bound to an in-progress download.
		res.ui.RightClick(holdingspace.FindDownloadChip().Name("Downloading "+downloadName)),

		// Left click the "Pause" context menu item. Note that this will result in
		// the underlying download being paused and the context menu being closed.
		res.ui.LeftClick(holdingspace.FindContextMenuItem().Name("Pause")),

		// Right click the download chip to show the context menu. Note that the
		// download chip is currently bound to a paused download.
		res.ui.RightClick(holdingspace.FindDownloadChip().Name("Download paused "+downloadName)),

		// Left click the "Resume" context menu item. Note that this will result in
		// the underlying download being resumed and the context menu being closed.
		res.ui.LeftClick(holdingspace.FindContextMenuItem().Name("Resume")),

		// Unblock the download so that the local server can complete the download
		// request. Until the download is unblocked, the local server will hang.
		unblockDownload,

		// Wait for the download to complete.
		res.ui.WaitUntilExists(holdingspace.FindDownloadChip().Name(downloadName)),
	)
}

// testDownloadPinAndUnpin performs testing of pinning and unpinning a download.
func testDownloadPinAndUnpin(
	res *downloadResource, downloadedFiles []string, unblockDownload uiauto.Action) uiauto.Action {
	if len(downloadedFiles) != 1 {
		return errorAction("testDownloadPinAndUnpin requires exactly one file to be downloaded")
	}
	downloadName := downloadedFiles[0]

	return uiauto.Combine("test pin and unpin",
		// Unblock the download so that the local server can complete the download
		// request. Until the download is unblocked, the local server will hang.
		unblockDownload,

		// Right click the download chip to show the context menu. Note that this
		// will wait until the underlying download has completed.
		res.ui.RightClick(holdingspace.FindDownloadChip().Name(downloadName)),

		// Left click the "Pin" context menu item. Note that this will result in
		// a pinned holding space item being created for the underlying download and
		// the context menu being closed.
		res.ui.LeftClick(holdingspace.FindContextMenuItem().Name("Pin")),

		// Ensure the pinned file chip is created.
		res.ui.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(downloadName)),

		// Right click the download chip to show the context menu.
		res.ui.RightClick(holdingspace.FindDownloadChip().Name(downloadName)),

		// Left click the "Unpin" context menu item. Note that this will result in
		// the pinned file chip being removed and the context menu being closed.
		res.ui.LeftClick(holdingspace.FindContextMenuItem().Name("Unpin")),

		// Ensure that the pinned file chip is removed.
		res.ui.WaitUntilGone(holdingspace.FindPinnedFileChip().Name(downloadName)),
		res.ui.EnsureGoneFor(holdingspace.FindPinnedFileChip().Name(downloadName), 5*time.Second),

		// Ensure that the download chip continues to exist despite the pinned
		// holding space item associated with the same download being destroyed.
		res.ui.Exists(holdingspace.FindDownloadChip().Name(downloadedFiles[0])),
	)
}

// testDownloadPinAndUnpinMultiple performs testing of pinning and unpinning a download.
func testDownloadPinAndUnpinMultiple(
	res *downloadResource, downloadedFiles []string, unblockDownload uiauto.Action) uiauto.Action {
	const shortTimeout = 3 * time.Second

	menuOption := holdingspace.FindContextMenuItem()
	pinOption := menuOption.Name("Pin")
	unpinOption := menuOption.Name("Unpin")

	return uiauto.Combine("test pin and unpin multiple",
		// Unblock the download so that the local server can complete the download
		// request. Until the download is unblocked, the local server will hang.
		unblockDownload,

		// Select all downloaded files by using keyboard shortcuts.
		selectAllFiles(res, holdingspace.FindDownloadChip(), downloadedFiles),

		// Right click the first downloaded file to show the context menu.
		// The menu might not appear immediately, so we RetryUntil to operate it multiple times if failed.
		// The time it would take for one round could be 15 seconds maximum, so we set a longer timeout for RetryUntil().
		res.ui.WithTimeout(time.Minute).WithInterval(time.Second).RetryUntil(
			res.ui.RightClick(holdingspace.FindChip().NameStartingWith(downloadedFiles[0])),
			res.ui.Exists(pinOption),
		),

		// Select the "Pin" option.
		res.ui.LeftClick(pinOption),

		// Wait until the menu disappears.
		res.ui.WaitUntilGone(pinOption),

		// Right click the first downloaded file to show the context menu
		// and select the "Unpin" option.
		res.ui.RightClick(holdingspace.FindDownloadChip().NameStartingWith(downloadedFiles[0])),

		// Verify "Copy", "Paste" and "Show in folder" will not show up while multi-selecting.
		res.ui.EnsureGoneFor(menuOption.Name("Show in folder"), shortTimeout),
		res.ui.EnsureGoneFor(menuOption.Name("Copy"), shortTimeout),
		res.ui.EnsureGoneFor(menuOption.Name("Paste"), shortTimeout),

		// Unpin items.
		res.ui.LeftClick(unpinOption),
		res.ui.WaitUntilGone(unpinOption),
	)
}

// testDownloadRemove performs testing of removing a download.
func testDownloadRemove(
	res *downloadResource, downloadedFiles []string, unblockDownload uiauto.Action) uiauto.Action {
	if len(downloadedFiles) != 1 {
		return errorAction("testDownloadRemove requires exactly one file to be downloaded")
	}
	downloadName := downloadedFiles[0]

	return uiauto.Combine("test remove",
		// Unblock the download so that the local server can complete the download
		// request. Until the download is unblocked, the local server will hang.
		unblockDownload,

		// Right click the download chip to show the context menu. Note that this
		// will wait until the underlying download has completed.
		res.ui.RightClick(holdingspace.FindDownloadChip().Name(downloadName)),

		// Left click the "Remove" context menu item. Note that this will result in
		// the holding space item for the underlying download being removed and the
		// context menu being closed.
		res.ui.LeftClick(holdingspace.FindContextMenuItem().Name("Remove")),

		// Ensure the download chip is removed.
		res.ui.WaitUntilGone(holdingspace.FindDownloadChip().Name(downloadName)),
		res.ui.EnsureGoneFor(holdingspace.FindDownloadChip().Name(downloadName), 5*time.Second),
	)
}

// testDownloadLaunch performs testing of launching a download.
func testDownloadLaunch(
	res *downloadResource, downloadedFiles []string, unblockDownload uiauto.Action) uiauto.Action {
	if len(downloadedFiles) != 1 {
		return errorAction("testDownloadLaunch requires exactly one file to be downloaded")
	}
	downloadName := downloadedFiles[0]

	// fileChip is the chip node shown on Holdingspace
	fileChip := holdingspace.FindDownloadChip().NameStartingWith(downloadName)
	return uiauto.Combine("test launch",
		// Unblock the download so that the local server can complete the download
		// request. Until the download is unblocked, the local server will hang.
		unblockDownload,

		// Double-click the download chip to launch the download.
		res.ui.DoubleClick(fileChip),
		verifyLaunchFiles(res, downloadedFiles),
	)
}

// testDownloadLaunchMultiple performs testing of launching multiple downloads.
func testDownloadLaunchMultiple(
	res *downloadResource, downloadedFiles []string, unblockDownload uiauto.Action) uiauto.Action {
	return uiauto.Combine("test launch multiple",
		// Unblock the download so that the local server can complete the download
		// request. Until the download is unblocked, the local server will hang.
		unblockDownload,

		// Select all downloaded files by using keyboard shortcuts.
		selectAllFiles(res, holdingspace.FindDownloadChip(), downloadedFiles),

		// Press the "Enter" key to launch the downloads.
		res.kb.AccelAction("enter"),

		// Verify that the downloads are launched in new tabs.
		verifyLaunchFiles(res, downloadedFiles),
	)
}

// errorAction returns an error object as uiauto.Action.
func errorAction(message string) uiauto.Action {
	return func(ctx context.Context) error {
		return errors.New(message)
	}
}

// selectAllFiles selects all specified files.
func selectAllFiles(res *downloadResource, chip *nodewith.Finder, fileNames []string) uiauto.Action {
	return func(ctx context.Context) error {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		if err := res.kb.AccelPress(ctx, "shift"); err != nil {
			return errors.Wrap(err, "failed to long-press shift")
		}
		defer res.kb.AccelRelease(cleanupCtx, "shift")

		for _, file := range fileNames {
			// fileChip is the chip node shown on Holdingspace.
			fileChip := chip.NameStartingWith(file)
			// toggleImageButton is the button shown within the chip when it's clicked.
			toggleImageButton := nodewith.HasClass("ToggleImageButton").Ancestor(fileChip)

			if err := uiauto.Combine("left click the file and verify it's selected",
				res.ui.LeftClick(fileChip),
				res.ui.WaitUntilExists(toggleImageButton),
			)(ctx); err != nil {
				return err
			}
		}

		return nil
	}
}

// verifyLaunchFiles verifies that the specified files are launched in new tabs or Text app.
func verifyLaunchFiles(res *downloadResource, fileNames []string) uiauto.Action {
	return func(ctx context.Context) error {
		// The way to verify launched files differs based on browser type.
		switch res.browserType {
		case browser.TypeAsh:
			browserRoot := nodewith.Ancestor(nodewith.Role(role.Window).HasClass("BrowserFrame").NameStartingWith("Chrome - "))
			for _, file := range fileNames {
				tabNode := browserRoot.Name(file).HasClass("Tab").Role(role.Tab)
				if err := res.ui.WaitUntilExists(tabNode)(ctx); err != nil {
					return errors.Wrap(err, "failed to find Chrome tab")
				}
			}
		case browser.TypeLacros:
			// If the primary browser is LaCrOS:
			// 1. Only one file will be launched regardless of the number of launched files.
			// 2. The viewer/editor will be the Text app instead of browser.
			textRoot := nodewith.Name("Text").Role(role.RootWebArea)
			if err := res.ui.WaitUntilExists(textRoot)(ctx); err != nil {
				return errors.Wrap(err, "failed to find Text app")
			}
		default:
			return errors.New("unsupported browser type")
		}
		return nil
	}
}
