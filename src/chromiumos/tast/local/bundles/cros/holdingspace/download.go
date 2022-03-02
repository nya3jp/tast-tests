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

	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/testing"
)

type downloadParams struct {
	testfunc    func(*uiauto.Context, string, uiauto.Action) uiauto.Action
	browserType browser.Type
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
			},
		}, {
			Name: "pause_and_resume",
			Val: downloadParams{
				testfunc:    testDownloadPauseAndResume,
				browserType: browser.TypeAsh,
			},
		}, {
			Name: "pin_and_unpin",
			Val: downloadParams{
				testfunc:    testDownloadPinAndUnpin,
				browserType: browser.TypeAsh,
			},
		}, {
			Name: "remove",
			Val: downloadParams{
				testfunc:    testDownloadRemove,
				browserType: browser.TypeAsh,
			},
		}, {
			Name: "lacros_cancel",
			Val: downloadParams{
				testfunc:    testDownloadCancel,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "lacros_pause_and_resume",
			Val: downloadParams{
				testfunc:    testDownloadPauseAndResume,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "lacros_pin_and_unpin",
			Val: downloadParams{
				testfunc:    testDownloadPinAndUnpin,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "lacros_remove",
			Val: downloadParams{
				testfunc:    testDownloadRemove,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
		Vars: []string{browserfixt.LacrosDeployedBinary},
	})
}

// Download verifies download behavior in holding space. It is expected that
// initiating a download will result in an item being added to holding space
// from which the user can cancel/pause/resume the download. Upon download
// completion, the user should be able to pin the download.
func Download(ctx context.Context, s *testing.State) {
	params := s.Param().(downloadParams)
	bt := params.browserType

	// Connect to a fresh Chrome instance to ensure holding space first-run state.
	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, s)
	if err != nil {
		s.Fatalf("Failed to connect to %v browser: %v", bt, err)
	}
	defer cr.Close(ctx)
	defer closeBrowser(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	// Ensure the tray does not exist prior adding anything to holding space.
	ui := uiauto.New(tconn)
	err = ui.EnsureGoneFor(holdingspace.FindTray(), 5*time.Second)(ctx)
	if err != nil {
		s.Fatal("Tray exists: ", err)
	}

	// Cache the name and location of the download.
	downloadName := "download.txt"
	downloadLocation := filepath.Join(filesapp.DownloadPath, downloadName)
	defer os.Remove(downloadLocation)

	// Create a local server. If a request indicates `redirect=true`, the response
	// HTML will cause automatic redirection back to the root URL after a short
	// delay. Otherwise, the response will result in a download being started that
	// will block completion until the `unblockDownloadChannel` is signaled.
	unblockDownloadChannel := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "text/html")
			if redirect := r.URL.Query().Get("redirect"); redirect == "true" {
				fmt.Fprintf(w, "<meta http-equiv='refresh' content='1;url=/' />")
				return
			}
			w.Header().Add("Content-Disposition", "attachment; filename="+downloadName)
			fmt.Fprintf(w, "Download started\n")
			f := w.(http.Flusher)
			f.Flush()
			<-unblockDownloadChannel
			fmt.Fprintf(w, "Download finished\n")
		}))
	defer server.Close()

	// Connect to the local server. Note that this method will block until the
	// browser has finished navigating to the desired URL. Since we actually want
	// to start a download and not navigate the browser we'll use a redirect
	// workaround to satisfy the requirement to navigate.
	conn, err := br.NewConn(ctx, server.URL+"?redirect=true")
	if err != nil {
		s.Fatal("Failed to connect to local server: ", err)
	}
	defer conn.Close()

	if err := uiauto.Combine("open bubble and confirm initial state",
		// Left click the tray to open the bubble.
		ui.LeftClick(holdingspace.FindTray()),

		// The pinned files section should contain an educational prompt and chip
		// informing the user that they can pin a file from the Files app.
		ui.WaitUntilExists(holdingspace.FindPinnedFilesSectionFilesAppPrompt()),
		ui.WaitUntilExists(holdingspace.FindPinnedFilesSectionFilesAppChip()),
	)(ctx); err != nil {
		s.Fatal("Failed to open bubble and confirm initial state: ", err)
	}

	// Perform additional parameterized testing.
	if err := params.testfunc(ui, downloadName, func(ctx context.Context) error {
		close(unblockDownloadChannel)
		return nil
	})(ctx); err != nil {
		s.Fatal("Fail to perform parameterized testing: ", err)
	}

	// Remove the file at `downloadLocation` which is backing the download. Note that
	// this will result in any associated holding space items being removed.
	if err := os.Remove(downloadLocation); err != nil && !os.IsNotExist(err) {
		s.Fatal("Failed to remove download: ", err)
	}

	// Ensure all holding space chips associated with the underlying download are
	// removed when the backing file is removed.
	if err := ui.WaitUntilGone(holdingspace.FindChip().Name(downloadName))(ctx); err != nil {
		s.Fatal("Chip exists: ", err)
	}
}

// testDownloadCancel performs testing of cancelling a download.
func testDownloadCancel(
	ui *uiauto.Context, downloadName string, unblockDownload uiauto.Action) uiauto.Action {
	return uiauto.Combine("test cancel",
		// Right click the download chip to show the context menu. Note that the
		// download chip is currently bound to an in-progress download.
		ui.RightClick(holdingspace.FindDownloadChip().Name("Downloading "+downloadName)),

		// Left click the "Cancel" context menu item. Note that this will result in
		// the underlying download being cancelled and the context menu being
		// closed.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Cancel")),

		// Unblock the download so that the local server can complete the download
		// request. This is necessary even though the download has been cancelled to
		// keep the local server from hanging.
		unblockDownload,

		// Ensure the download chip is removed with its backing file.
		ui.WaitUntilGone(holdingspace.FindDownloadChip().Name(downloadName)),
	)
}

// testDownloadPauseAndResume performs testing of pausing and resuming a download.
func testDownloadPauseAndResume(
	ui *uiauto.Context, downloadName string, unblockDownload uiauto.Action) uiauto.Action {
	return uiauto.Combine("test pause and resume",
		// Right click the download chip to show the context menu. Note that the
		// download chip is currently bound to an in-progress download.
		ui.RightClick(holdingspace.FindDownloadChip().Name("Downloading "+downloadName)),

		// Left click the "Pause" context menu item. Note that this will result in
		// the underlying download being paused and the context menu being closed.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Pause")),

		// Right click the download chip to show the context menu. Note that the
		// download chip is currently bound to a paused download.
		ui.RightClick(holdingspace.FindDownloadChip().Name("Download paused "+downloadName)),

		// Left click the "Resume" context menu item. Note that this will result in
		// the underlying download being resumed and the context menu being closed.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Resume")),

		// Unblock the download so that the local server can complete the download
		// request. Until the download is unblocked, the local server will hang.
		unblockDownload,

		// Wait for the download to complete.
		ui.WaitUntilExists(holdingspace.FindDownloadChip().Name(downloadName)),
	)
}

// testDownloadPinAndUnpin performs testing of pinning and unpinning a download.
func testDownloadPinAndUnpin(
	ui *uiauto.Context, downloadName string, unblockDownload uiauto.Action) uiauto.Action {
	return uiauto.Combine("test pin and unpin",
		// Unblock the download so that the local server can complete the download
		// request. Until the download is unblocked, the local server will hang.
		unblockDownload,

		// Right click the download chip to show the context menu. Note that this
		// will wait until the underlying download has completed.
		ui.RightClick(holdingspace.FindDownloadChip().Name(downloadName)),

		// Left click the "Pin" context menu item. Note that this will result in
		// a pinned holding space item being created for the underlying download and
		// the context menu being closed.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Pin")),

		// Ensure the pinned file chip is created.
		ui.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(downloadName)),

		// Right click the download chip to show the context menu.
		ui.RightClick(holdingspace.FindDownloadChip().Name(downloadName)),

		// Left click the "Unpin" context menu item. Note that this will result in
		// the pinned file chip being removed and the context menu being closed.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Unpin")),

		// Ensure that the pinned file chip is removed.
		ui.WaitUntilGone(holdingspace.FindPinnedFileChip().Name(downloadName)),
		ui.EnsureGoneFor(holdingspace.FindPinnedFileChip().Name(downloadName), 5*time.Second),

		// Ensure that the download chip continues to exist despite the pinned
		// holding space item associated with the same download being destroyed.
		ui.Exists(holdingspace.FindDownloadChip().Name(downloadName)),
	)
}

// testDownloadRemove performs testing of removing a download.
func testDownloadRemove(
	ui *uiauto.Context, downloadName string, unblockDownload uiauto.Action) uiauto.Action {
	return uiauto.Combine("test remove",
		// Unblock the download so that the local server can complete the download
		// request. Until the download is unblocked, the local server will hang.
		unblockDownload,

		// Right click the download chip to show the context menu. Note that this
		// will wait until the underlying download has completed.
		ui.RightClick(holdingspace.FindDownloadChip().Name(downloadName)),

		// Left click the "Remove" context menu item. Note that this will result in
		// the holding space item for the underlying download being removed and the
		// context menu being closed.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Remove")),

		// Ensure the download chip is removed.
		ui.WaitUntilGone(holdingspace.FindDownloadChip().Name(downloadName)),
		ui.EnsureGoneFor(holdingspace.FindDownloadChip().Name(downloadName), 5*time.Second),
	)
}
