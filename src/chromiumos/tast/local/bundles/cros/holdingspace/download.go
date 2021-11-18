// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/testing"
)

type downloadParams struct {
	downloadSize int
	testfunc     func(*uiauto.Context, string) uiauto.Action
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Download,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies download behavior in holding space",
		Contacts: []string{
			"dmblack@google.com",
			"tote-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"download.html"},
		Params: []testing.Param{{
			Name: "cancel",
			Val: downloadParams{
				downloadSize: 500 * 1024 * 1024, // 500 MB to give time to cancel.
				testfunc:     testDownloadCancel,
			},
		}, {
			Name: "pause_and_resume",
			Val: downloadParams{
				downloadSize: 500 * 1024 * 1024, // 500 MB to give time to pause/resume.
				testfunc:     testDownloadPauseAndResume,
			},
		}, {
			Name: "pin_and_unpin",
			Val: downloadParams{
				downloadSize: 1, // 1 B.
				testfunc:     testDownloadPinAndUnpin,
			},
		}},
	})
}

// Download verifies download behavior in holding space. It is expected that
// initiating a download will result in an item being added to holding space
// from which the user can cancel/pause/resume the download. Upon download
// completion, the user should be able to pin the download.
func Download(ctx context.Context, s *testing.State) {
	params := s.Param().(downloadParams)

	// Connect to a fresh Chrome instance to ensure holding space first-run state.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

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

	// Cache the name, size, and location of the download.
	downloadName := "download.txt"
	downloadSize := strconv.Itoa(params.downloadSize)
	downloadLocation := filepath.Join(filesapp.DownloadPath, downloadName)
	defer os.Remove(downloadLocation)

	// Create a local server.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Connect to the local server and navigate to `url`. Note that this will
	// cause the download to be started automatically.
	url := "download.html?filename=" + downloadName + "&filesize=" + downloadSize
	conn, err := cr.NewConn(ctx, filepath.Join(server.URL, url))
	if err != nil {
		s.Fatal("Failed to connect to local server: ", err)
	}
	defer conn.Close()

	// Left click the tray to open the bubble.
	if err := ui.LeftClick(holdingspace.FindTray())(ctx); err != nil {
		s.Fatal("Failed to left click tray: ", err)
	}

	// Perform additional parameterized testing.
	if err := params.testfunc(ui, downloadName)(ctx); err != nil {
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
func testDownloadCancel(ui *uiauto.Context, downloadName string) uiauto.Action {
	return uiauto.Combine("test cancel",
		// Right click the download chip to show the context menu. Note that the
		// download chip is currently bound to an in-progress download.
		ui.RightClick(holdingspace.FindDownloadChip().Name("Downloading "+downloadName)),

		// Left click the "Cancel" context menu item. Note that this will result in
		// the underlying download being cancelled and the context menu being
		// closed.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Cancel")),

		// Ensure the download chip is removed with its backing file.
		ui.WaitUntilGone(holdingspace.FindDownloadChip().Name(downloadName)),
	)
}

// testDownloadPauseAndResume performs testing of pausing and resuming a download.
func testDownloadPauseAndResume(ui *uiauto.Context, downloadName string) uiauto.Action {
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

		// Wait for the download to complete.
		ui.WaitUntilExists(holdingspace.FindDownloadChip().Name(downloadName)),
	)
}

// testDownloadPinAndUnpin performs testing of pinning and unpinning a download.
func testDownloadPinAndUnpin(ui *uiauto.Context, downloadName string) uiauto.Action {
	return uiauto.Combine("test pin and unpin",
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

		// Ensure that the pinned file chips is removed.
		ui.WaitUntilGone(holdingspace.FindPinnedFileChip().Name(downloadName)),
		ui.EnsureGoneFor(holdingspace.FindPinnedFileChip().Name(downloadName), 5*time.Second),

		// Ensure that the download chip continues to exist despite the pinned
		// holding space item associated with the same download being destroyed.
		ui.Exists(holdingspace.FindDownloadChip().Name(downloadName)),
	)
}
